package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: config パッケージ公開境界 (Target 解決)
// 新形式の解決規則を外部挙動として検証する。
// 指定なしは常時適用、{targets} は完全一致のみ、不一致・無指定時はスキップ。
// "common" は普通のTargetとしてのみ一致する。結果は配置先ソート順。
func TestResolve(t *testing.T) {
	body := "[entries]\n" +
		`"~/.c" = { targets = [{ target = "common", src = "c-common" }] }` + "\n" +
		`"~/.b" = { targets = [{ target = "win", src = "b-win" }, { target = "wsl", src = "b-wsl" }] }` + "\n" +
		`"~/.a" = { src = "a" }` + "\n"
	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	tests := []struct {
		name   string
		target string
		want   []Entry
	}{
		{"無指定は指定なしのみ", "", []Entry{{Src: "a", Dest: "~/.a"}}},
		{"winは指定なし＋一致のみ", "win", []Entry{{Src: "a", Dest: "~/.a"}, {Src: "b-win", Dest: "~/.b"}}},
		{"wslは指定なし＋一致のみ", "wsl", []Entry{{Src: "a", Dest: "~/.a"}, {Src: "b-wsl", Dest: "~/.b"}}},
		{"commonは普通のTargetとして一致のみ", "common", []Entry{{Src: "a", Dest: "~/.a"}, {Src: "c-common", Dest: "~/.c"}}},
		{"未知Targetは指定なしのみ", "linux", []Entry{{Src: "a", Dest: "~/.a"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.Resolve(tt.target)
			if len(got) != len(tt.want) {
				t.Fatalf("Resolve(%q) = %d件 %+v, want %d件 %+v", tt.target, len(got), got, len(tt.want), tt.want)
			}
			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("index %d: got %+v, want %+v", i, got[i], w)
				}
			}
			// 配置先ソート順の安定性
			for i := 1; i < len(got); i++ {
				if got[i-1].Dest >= got[i].Dest {
					t.Errorf("解決結果が配置先ソート順でない: %+v", got)
					break
				}
			}
		})
	}
}

// Seam: config パッケージ公開境界 (新形式の読込)
// 配置先キー・{src}/{targets}排他・単数Targetの読込を外部挙動で検証する。
func TestLoadNewFormat(t *testing.T) {
	t.Run("新形式のサンプルを読み込める", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		body := "[entries]\n" +
			`"~/.config/starship.toml" = { src = "dotfiles/.config/starship.toml" }` + "\n" +
			`"~/.gitconfig" = { targets = [` + "\n" +
			`  { target = "win", src = "dotfiles/win/.gitconfig" },` + "\n" +
			`  { target = "wsl", src = "dotfiles/wsl/.gitconfig" },` + "\n" +
			`] }` + "\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if len(cfg.Entries) != 2 {
			t.Fatalf("Entries = %d件, want 2件", len(cfg.Entries))
		}
		got := cfg.Resolve("win")
		if len(got) != 2 {
			t.Fatalf("Resolve(win) = %d件, want 2件 (%+v)", len(got), got)
		}
		if got[0].Dest != "~/.config/starship.toml" || got[0].Src != "dotfiles/.config/starship.toml" {
			t.Errorf("指定なしEntryの解決不正: %+v", got[0])
		}
		if got[1].Dest != "~/.gitconfig" || got[1].Src != "dotfiles/win/.gitconfig" {
			t.Errorf("一致Targetの解決不正: %+v", got[1])
		}
	})

	t.Run("宣言順が逆でも解決は配置先ソート順", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		body := "[entries]\n" +
			`"~/.z" = { src = "z" }` + "\n" +
			`"~/.m" = { src = "m" }` + "\n" +
			`"~/.a" = { src = "a" }` + "\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		got := cfg.Resolve("")
		want := []string{"~/.a", "~/.m", "~/.z"}
		if len(got) != len(want) {
			t.Fatalf("Resolve = %+v, want dest %v", got, want)
		}
		for i, w := range want {
			if got[i].Dest != w {
				t.Errorf("index %d: got Dest=%q, want %q", i, got[i].Dest, w)
			}
		}
	})
}

// Seam: config パッケージ公開境界 (検証エラー群)
// 排他違反・欠落・空・重複を明確に失敗させる外部挙動を検証する。
func TestLoadValidationErrors(t *testing.T) {
	cases := map[string]string{
		"srcとtargetsの併記は拒否": "[entries]\n\"~/.a\" = { src = \"a\", targets = [{ target = \"win\", src = \"b\" }] }\n",
		"両方なしは拒否":           "[entries]\n\"~/.a\" = {}\n",
		"空srcは拒否":           "[entries]\n\"~/.a\" = { src = \"\" }\n",
		"空targets配列は拒否":     "[entries]\n\"~/.a\" = { targets = [] }\n",
		"空targetは拒否":        "[entries]\n\"~/.a\" = { targets = [{ target = \"\", src = \"a\" }] }\n",
		"targets要素の空srcは拒否": "[entries]\n\"~/.a\" = { targets = [{ target = \"win\", src = \"\" }] }\n",
		"同一配置先内の重複は拒否":      "[entries]\n\"~/.a\" = { targets = [{ target = \"win\", src = \"a\" }, { target = \"win\", src = \"b\" }] }\n",
		"空destは拒否":          "[entries]\n\"\" = { src = \"a\" }\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "mdots.toml")
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(p); err == nil {
				t.Errorf("%s: エラー expected, got nil", name)
			}
		})
	}
}

// Seam: config パッケージ公開境界 (旧形式の明確な失敗)
// 旧配列形式・旧配列Targetは読めず明確に失敗する外部挙動を検証する。
func TestLoadRejectsOldFormat(t *testing.T) {
	cases := map[string]string{
		"旧[[entries]]配列は拒否": "[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n",
		"旧target配列は拒否":      "[entries]\n\"~/.a\" = { src = \"a\", target = [\"win\"] }\n",
		"旧target文字列は拒否":     "[entries]\n\"~/.a\" = { src = \"a\", target = \"win\" }\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "mdots.toml")
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(p)
			if err == nil {
				t.Fatalf("%s: エラー expected, got nil", name)
			}
			if !strings.Contains(err.Error(), "old") && !strings.Contains(err.Error(), "entries") {
				t.Errorf("%s: 明確な失敗文言 expected, got %q", name, err.Error())
			}
		})
	}
}

// Seam: config パッケージ公開境界 (dest の ~ 展開)
// dest の ~/ を os.UserHomeDir() で展開する外部挙動を検証する。
func TestExpandDest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, _ := ExpandDest("~"); got != home {
		t.Errorf("ExpandDest(~) = %q, want %q", got, home)
	}
	want := filepath.Join(home, ".config", "a")
	if got, _ := ExpandDest("~/.config/a"); got != want {
		t.Errorf("ExpandDest(~/...) = %q, want %q", got, want)
	}
	if got, _ := ExpandDest("/abs/path"); got != "/abs/path" {
		t.Errorf("絶対パスはそのまま: got %q", got)
	}
}

// Seam: config パッケージ公開境界 (init 雛形作成)
// 空Store作成・既存ありエラー・生成物がLoadを通る外部挙動を検証する。
func TestInitCreatesTemplate(t *testing.T) {
	dir := t.TempDir()
	p, err := Init(dir)
	if err != nil {
		t.Fatalf("Init error: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	body := string(data)
	for _, want := range []string{"mdots push", "src =", "dest =", "target", "common", "[[entries]]"} {
		if !strings.Contains(body, want) {
			t.Errorf("template should contain %q, got:\n%s", want, body)
		}
	}
	if _, err := Load(p); err != nil {
		t.Errorf("generated template must Load: %v", err)
	}
	if _, err := Init(dir); err == nil {
		t.Fatal("second Init: エラー expected, got nil")
	} else if !strings.Contains(err.Error(), "mdots.toml already exists in ") {
		t.Errorf("既存ありエラーメッセージ不正: got %q", err.Error())
	}
}

// Seam: config パッケージ公開境界 (Store 発見)
// カレント直下の mdots.toml のみを参照する外部挙動を t.TempDir() の実FSで検証する。
func TestFindStore(t *testing.T) {
	t.Run("カレント直下のStoreを発見できる", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "mdots.toml"), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := FindStore(root)
		if err != nil {
			t.Fatalf("FindStore error: %v", err)
		}
		if got != root {
			t.Errorf("FindStore = %q, want %q", got, root)
		}
	})

	t.Run("サブディレクトリからは失敗しin形式のエラー", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "mdots.toml"), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		sub := filepath.Join(root, "a", "b")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := FindStore(sub)
		if err == nil {
			t.Fatal("エラー expected, got nil")
		}
		if !strings.Contains(err.Error(), "mdots.toml not found in "+sub) {
			t.Errorf("エラーメッセージ不正: got %q, want contain %q", err.Error(), "mdots.toml not found in "+sub)
		}
	})

	t.Run("見つからないときはin形式のエラー", func(t *testing.T) {
		start := t.TempDir()
		_, err := FindStore(start)
		if err == nil {
			t.Fatal("エラー expected, got nil")
		}
		if !strings.Contains(err.Error(), "mdots.toml not found in "+start) {
			t.Errorf("エラーメッセージ不正: got %q, want contain %q", err.Error(), "mdots.toml not found in "+start)
		}
		if strings.Contains(err.Error(), "searched from") {
			t.Errorf("旧文言が残っている: got %q", err.Error())
		}
	})
}
