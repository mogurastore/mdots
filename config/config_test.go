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
		`"~/.c" = { targets = { common = { src = "c-common" } } }` + "\n" +
		`"~/.b" = { targets = { win = { src = "b-win" }, wsl = { src = "b-wsl" } } }` + "\n" +
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
		{"無指定は指定なしのみ", "", []Entry{{Src: "a", Dest: "~/.a", Override: true}}},
		{"winは指定なし＋一致のみ", "win", []Entry{{Src: "a", Dest: "~/.a", Override: true}, {Src: "b-win", Dest: "~/.b", Override: true}}},
		{"wslは指定なし＋一致のみ", "wsl", []Entry{{Src: "a", Dest: "~/.a", Override: true}, {Src: "b-wsl", Dest: "~/.b", Override: true}}},
		{"commonは普通のTargetとして一致のみ", "common", []Entry{{Src: "a", Dest: "~/.a", Override: true}, {Src: "c-common", Dest: "~/.c", Override: true}}},
		{"未知Targetは指定なしのみ", "linux", []Entry{{Src: "a", Dest: "~/.a", Override: true}}},
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

// Seam: config パッケージ公開境界 (Target 一覧)
// 全Entryのtargetsキーを集約し重複排除・ソートして返す外部挙動を検証する。
// 素Entryは無視する。未定義時は空を返す。
func TestTargets(t *testing.T) {
	t.Run("複数Entryに分散したTargetを重複排除・ソートして返す", func(t *testing.T) {
		body := "[entries]\n" +
			`"~/.c" = { targets = { wsl = { src = "c-wsl" }, win = { src = "c-win" } } }` + "\n" +
			`"~/.b" = { targets = { win = { src = "b-win" }, linux = { src = "b-linux" } } }` + "\n" +
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
		got := cfg.Targets()
		want := []string{"linux", "win", "wsl"}
		if len(got) != len(want) {
			t.Fatalf("Targets() = %q, want %q", got, want)
		}
		for i, w := range want {
			if got[i] != w {
				t.Errorf("index %d: got %q, want %q", i, got[i], w)
			}
		}
	})

	t.Run("Target未定義時は空を返す", func(t *testing.T) {
		body := "[entries]\n" +
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
		if got := cfg.Targets(); len(got) != 0 {
			t.Errorf("Targets() = %q, want empty", got)
		}
	})
}

// Seam: config パッケージ公開境界 (新形式の読込)
// 配置先キー・{src}/{targets}排他・Targetキー化の読込を外部挙動で検証する。
func TestLoadNewFormat(t *testing.T) {
	t.Run("新形式のサンプルを読み込める", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		body := "[entries]\n" +
			`"~/.config/starship.toml" = { src = "dotfiles/.config/starship.toml" }` + "\n" +
			`"~/.gitconfig" = { targets = { win = { src = "dotfiles/win/.gitconfig" }, wsl = { src = "dotfiles/wsl/.gitconfig" } } }` + "\n"
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

	t.Run("改行ありのtargetsも読み込める", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		body := "[entries]\n" +
			`"~/.gitconfig" = {` + "\n" +
			`  targets = {` + "\n" +
			`    win = { src = "dotfiles/win/.gitconfig" },` + "\n" +
			`    wsl = { src = "dotfiles/wsl/.gitconfig" },` + "\n" +
			`  },` + "\n" +
			`}` + "\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		got := cfg.Resolve("wsl")
		if len(got) != 1 {
			t.Fatalf("Resolve(wsl) = %d件, want 1件 (%+v)", len(got), got)
		}
		if got[0].Dest != "~/.gitconfig" || got[0].Src != "dotfiles/wsl/.gitconfig" {
			t.Errorf("一致Targetの解決不正: %+v", got[0])
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
// 排他違反・欠落・空を明確に失敗させる外部挙動を検証する。
func TestLoadValidationErrors(t *testing.T) {
	cases := map[string]string{
		"srcとtargetsの併記は拒否":  "[entries]\n\"~/.a\" = { src = \"a\", targets = { win = { src = \"b\" } } }\n",
		"両方なしは拒否":            "[entries]\n\"~/.a\" = {}\n",
		"空srcは拒否":            "[entries]\n\"~/.a\" = { src = \"\" }\n",
		"空targetsは拒否":        "[entries]\n\"~/.a\" = { targets = {} }\n",
		"空targetは拒否":         "[entries]\n\"~/.a\" = { targets = { \"\" = { src = \"a\" } } }\n",
		"targets要素の空srcは拒否":  "[entries]\n\"~/.a\" = { targets = { win = { src = \"\" } } }\n",
		"targets要素のsrc欠落は拒否": "[entries]\n\"~/.a\" = { targets = { win = {} } }\n",
		"空destは拒否":           "[entries]\n\"\" = { src = \"a\" }\n",
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

// Seam: config パッケージ公開境界 (想定外形式の拒否)
// rejectUnexpectedFormat が旧記法を含む想定外形式を明確に失敗させる外部挙動を検証する。
// Validate の網羅は TestLoadValidationErrors に寄せ、ここでは形式レベルの拒否のみ扱う。
func TestLoadRejectsUnexpectedFormat(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"旧[[entries]]配列は拒否": {"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n", "unknown field"},
		"旧target配列は拒否":      {"[entries]\n\"~/.a\" = { src = \"a\", target = [\"win\"] }\n", "unknown field"},
		"旧target文字列は拒否":     {"[entries]\n\"~/.a\" = { src = \"a\", target = \"win\" }\n", "unknown field"},
		"旧targets配列は拒否":     {"[entries]\n\"~/.a\" = { targets = [{ target = \"win\", src = \"a\" }] }\n", "unknown field"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "mdots.toml")
			if err := os.WriteFile(p, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(p)
			if err == nil {
				t.Fatalf("%s: エラー expected, got nil", name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s: %q を含む明確な失敗 expected, got %q", name, tc.want, err.Error())
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
// 雛形は新形式（配置先キー・{src}/{targets}排他・Targetキー化）のみを含み、
// 旧形式の記法（[[entries]]・dest =・target配列・旧targets配列・common特別扱い）を含まない。
// 記法は add の Save と同じテーブル形式とし、インライン形式は使わない。
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
	for _, want := range []string{"[entries.", "src =", "targets", "targets.win", `"~/`, "override"} {
		if !strings.Contains(body, want) {
			t.Errorf("template should contain %q, got:\n%s", want, body)
		}
	}
	if strings.Contains(body, "[entries]\n") {
		t.Errorf("template must not contain bare [entries] line, got:\n%s", body)
	}
	for _, old := range []string{"[[entries]]", "dest =", "common", "target = [", "target = \"", "{ target = ", "= {"} {
		if strings.Contains(body, old) {
			t.Errorf("template must not contain old format %q, got:\n%s", old, body)
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

// Seam: config パッケージ公開境界 (override の読み・解決)
// 配置先直下・targets 内の override を実ファイル＋Load/Resolve の外部挙動で検証する。
// 省略時は true、選択自体には影響しない。
func TestLoadOverrideResolves(t *testing.T) {
	load := func(t *testing.T, body string) Config {
		t.Helper()
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		return cfg
	}

	t.Run("省略時はtrue", func(t *testing.T) {
		cfg := load(t, "[entries]\n\"~/.a\" = { src = \"a\" }\n")
		got := cfg.Resolve("")
		if len(got) != 1 || !got[0].Override {
			t.Errorf("省略時は Override=true expected, got %+v", got)
		}
	})

	t.Run("配置先直下のfalseを解決できる", func(t *testing.T) {
		cfg := load(t, "[entries]\n\"~/.a\" = { src = \"a\", override = false }\n")
		got := cfg.Resolve("")
		if len(got) != 1 || got[0].Override {
			t.Errorf("Override=false expected, got %+v", got)
		}
	})

	t.Run("配置先直下のtrueを解決できる", func(t *testing.T) {
		cfg := load(t, "[entries]\n\"~/.a\" = { src = \"a\", override = true }\n")
		got := cfg.Resolve("")
		if len(got) != 1 || !got[0].Override {
			t.Errorf("Override=true expected, got %+v", got)
		}
	})

	t.Run("targets内でTargetごとに変えられる", func(t *testing.T) {
		body := "[entries]\n" +
			`"~/.a" = { targets = { win = { src = "b", override = false }, wsl = { src = "c" } } }` + "\n"
		cfg := load(t, body)
		gotWin := cfg.Resolve("win")
		if len(gotWin) != 1 || gotWin[0].Override {
			t.Errorf("win は false expected, got %+v", gotWin)
		}
		gotWsl := cfg.Resolve("wsl")
		if len(gotWsl) != 1 || !gotWsl[0].Override {
			t.Errorf("wsl は省略時 true expected, got %+v", gotWsl)
		}
	})

	t.Run("overrideは選択に影響しない", func(t *testing.T) {
		body := "[entries]\n" +
			`"~/.a" = { src = "a", override = false }` + "\n" +
			`"~/.b" = { targets = { win = { src = "b", override = false } } }` + "\n"
		cfg := load(t, body)
		if got := cfg.Resolve(""); len(got) != 1 || got[0].Dest != "~/.a" {
			t.Errorf("無指定は指定なしのみ expected, got %+v", got)
		}
		if got := cfg.Resolve("win"); len(got) != 2 {
			t.Errorf("win は2件 expected, got %+v", got)
		}
		if got := cfg.Resolve("linux"); len(got) != 1 || got[0].Dest != "~/.a" {
			t.Errorf("未知Targetは指定なしのみ expected, got %+v", got)
		}
	})
}

// Seam: config パッケージ公開境界 (override の検証エラー)
// 不正値・未知フィールドを明確に失敗させる外部挙動を検証する。
func TestLoadOverrideValidationErrors(t *testing.T) {
	cases := map[string]string{
		"配置先直下の文字列は拒否":        "[entries]\n\"~/.a\" = { src = \"a\", override = \"yes\" }\n",
		"配置先直下の数値は拒否":         "[entries]\n\"~/.a\" = { src = \"a\", override = 1 }\n",
		"targets内の文字列は拒否":     "[entries]\n\"~/.a\" = { targets = { win = { src = \"a\", override = \"no\" } } }\n",
		"配置先直下の未知フィールドは拒否":    "[entries]\n\"~/.a\" = { src = \"a\", overwride = false }\n",
		"targets内の未知フィールドは拒否": "[entries]\n\"~/.a\" = { targets = { win = { src = \"a\", overide = false } } }\n",
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
			if !strings.Contains(err.Error(), "override") && !strings.Contains(err.Error(), "unknown field") {
				t.Errorf("%s: override/unknown を含む明確な失敗 expected, got %q", name, err.Error())
			}
		})
	}
}

// Seam: config パッケージ公開境界 (add向けdest正規化)
// ~/...維持・HOME配下絶対パス→~/...・それ以外エラーの外部挙動を検証する。
func TestNormalizeDest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, err := NormalizeDest("~/.vimrc"); err != nil || got != "~/.vimrc" {
		t.Errorf("NormalizeDest(~/...) = %q, %v; want %q, nil", got, err, "~/.vimrc")
	}
	if got, err := NormalizeDest("~"); err != nil || got != "~" {
		t.Errorf("NormalizeDest(~) = %q, %v; want %q, nil", got, err, "~")
	}
	abs := filepath.Join(home, ".vimrc")
	if got, err := NormalizeDest(abs); err != nil || got != "~/.vimrc" {
		t.Errorf("NormalizeDest(HOME配下絶対) = %q, %v; want %q, nil", got, err, "~/.vimrc")
	}
	if got, err := NormalizeDest(home); err != nil || got != "~" {
		t.Errorf("NormalizeDest(HOME自体) = %q, %v; want %q, nil", got, err, "~")
	}
	for _, raw := range []string{"/etc/hosts", "relative/path", "~other/.vimrc", ""} {
		if got, err := NormalizeDest(raw); err == nil {
			t.Errorf("NormalizeDest(%q) = %q, want error", raw, got)
		}
	}
}

// Seam: config パッケージ公開境界 (add向けsrc算出)
// ~/除去＋dotfiles/＋残りの外部挙動を検証する。
func TestSrcForDest(t *testing.T) {
	if got, err := SrcForDest("~/.vimrc"); err != nil || got != "dotfiles/.vimrc" {
		t.Errorf("SrcForDest(~/.vimrc) = %q, %v; want %q, nil", got, err, "dotfiles/.vimrc")
	}
	if got, err := SrcForDest("~/.config/helix/config.toml"); err != nil || got != "dotfiles/.config/helix/config.toml" {
		t.Errorf("SrcForDest深い階層 = %q, %v", got, err)
	}
	if got, err := SrcForDest("~"); err != nil || got != "dotfiles" {
		t.Errorf("SrcForDest(~) = %q, %v; want dotfiles, nil", got, err)
	}
	if _, err := SrcForDest("/etc/hosts"); err == nil {
		t.Error("SrcForDest(絶対パス): エラー expected, got nil")
	}
}

// Seam: config パッケージ公開境界 (add向けsrc算出・target付き)
// dotfiles/<target>/... への写像の外部挙動を検証する。
func TestSrcForDestWithTarget(t *testing.T) {
	if got, err := SrcForDestWithTarget("~/.vimrc", "win"); err != nil || got != "dotfiles/win/.vimrc" {
		t.Errorf("SrcForDestWithTarget(~/.vimrc, win) = %q, %v; want %q, nil", got, err, "dotfiles/win/.vimrc")
	}
	if got, err := SrcForDestWithTarget("~/.config/helix/config.toml", "wsl"); err != nil || got != "dotfiles/wsl/.config/helix/config.toml" {
		t.Errorf("SrcForDestWithTarget深い階層 = %q, %v", got, err)
	}
	if got, err := SrcForDestWithTarget("~", "win"); err != nil || got != "dotfiles/win" {
		t.Errorf("SrcForDestWithTarget(~, win) = %q, %v; want %q, nil", got, err, "dotfiles/win")
	}
	if got, err := SrcForDestWithTarget("~/.vimrc", ""); err != nil || got != "dotfiles/.vimrc" {
		t.Errorf("SrcForDestWithTarget空targetは素と同じ = %q, %v", got, err)
	}
	if _, err := SrcForDestWithTarget("/etc/hosts", "win"); err == nil {
		t.Error("SrcForDestWithTarget(絶対パス): エラー expected, got nil")
	}
}

// Seam: config パッケージ公開境界 (add向け登録・target付き)
// targets形式での登録・別target追記・同一target/素のsrc済みエラー・
// overrideを書かない外部挙動を検証する。
func TestAddTargetRegisters(t *testing.T) {
	var cfg Config
	if err := cfg.AddTarget("~/.vimrc", "win", "dotfiles/win/.vimrc"); err != nil {
		t.Fatalf("AddTarget error: %v", err)
	}
	got := cfg.Resolve("win")
	if len(got) != 1 || got[0].Dest != "~/.vimrc" || got[0].Src != "dotfiles/win/.vimrc" || !got[0].Override {
		t.Errorf("登録後の解決不正: %+v", got)
	}
	if got := cfg.Resolve(""); len(got) != 0 {
		t.Errorf("無指定解決でtarget付きは出ない expected, got %+v", got)
	}
	if got := cfg.Resolve("wsl"); len(got) != 0 {
		t.Errorf("不一致Targetで出ない expected, got %+v", got)
	}
	// 別targetは追記マージして成功する。
	if err := cfg.AddTarget("~/.vimrc", "wsl", "dotfiles/wsl/.vimrc"); err != nil {
		t.Fatalf("別target追記 error: %v", err)
	}
	if got := cfg.Resolve("win"); len(got) != 1 || got[0].Src != "dotfiles/win/.vimrc" {
		t.Errorf("追記後のwin解決不正: %+v", got)
	}
	if got := cfg.Resolve("wsl"); len(got) != 1 || got[0].Src != "dotfiles/wsl/.vimrc" {
		t.Errorf("追記後のwsl解決不正: %+v", got)
	}
	if err := cfg.AddTarget("~/.vimrc", "win", "dotfiles/win/.vimrc"); err == nil {
		t.Error("同一targetの再登録: エラー expected, got nil")
	} else if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("同一targetエラー文言不正: got %q", err.Error())
	}
	var cfg2 Config
	if err := cfg2.Add("~/.a", "dotfiles/.a"); err != nil {
		t.Fatal(err)
	}
	if err := cfg2.AddTarget("~/.a", "win", "dotfiles/win/.a"); err == nil {
		t.Error("素のsrc済みへのtarget追加: エラー expected, got nil")
	}
	for _, tc := range []struct{ dest, target, src string }{
		{"", "win", "a"},
		{"~/.a", "", "a"},
		{"~/.a", "win", ""},
	} {
		var c Config
		if err := c.AddTarget(tc.dest, tc.target, tc.src); err == nil {
			t.Errorf("AddTarget(%q,%q,%q): エラー expected, got nil", tc.dest, tc.target, tc.src)
		}
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(p, []byte("[entries]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendTargetEntry(p, "~/.vimrc", "win", "dotfiles/win/.vimrc"); err != nil {
		t.Fatalf("Append error: %v", err)
	}
	if err := AppendTargetEntry(p, "~/.vimrc", "wsl", "dotfiles/wsl/.vimrc"); err != nil {
		t.Fatalf("Append error: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "override") {
		t.Errorf("targets登録でoverrideを書かない expected, got:\n%s", data)
	}
	back, err := Load(p)
	if err != nil {
		t.Fatalf("保存物のLoad error: %v", err)
	}
	if got := back.Resolve("win"); len(got) != 1 || got[0].Src != "dotfiles/win/.vimrc" {
		t.Errorf("往復後のwin解決不正: %+v", got)
	}
	if got := back.Resolve("wsl"); len(got) != 1 || got[0].Src != "dotfiles/wsl/.vimrc" {
		t.Errorf("往復後のwsl解決不正: %+v", got)
	}
}

// Seam: config パッケージ公開境界 (add向け登録)
// 素のsrc形式での登録・既存destエラー・overrideを書かない外部挙動を検証する。
func TestAddRegistersPlainSrc(t *testing.T) {
	var cfg Config
	if err := cfg.Add("~/.vimrc", "dotfiles/.vimrc"); err != nil {
		t.Fatalf("Add error: %v", err)
	}
	got := cfg.Resolve("")
	if len(got) != 1 || got[0].Dest != "~/.vimrc" || got[0].Src != "dotfiles/.vimrc" || !got[0].Override {
		t.Errorf("登録後の解決不正: %+v", got)
	}
	if err := cfg.Add("~/.vimrc", "dotfiles/.vimrc"); err == nil {
		t.Error("既存destの再登録: エラー expected, got nil")
	} else if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("既存destエラー文言不正: got %q", err.Error())
	}
	if err := cfg.Add("", "a"); err == nil {
		t.Error("空dest: エラー expected, got nil")
	}
	if err := cfg.Add("~/.a", ""); err == nil {
		t.Error("空src: エラー expected, got nil")
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(p, []byte("[entries]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendPlainEntry(p, "~/.vimrc", "dotfiles/.vimrc"); err != nil {
		t.Fatalf("Append error: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "override") {
		t.Errorf("素のsrc登録でoverrideを書かない expected, got:\n%s", data)
	}
}

// Seam: config パッケージ公開境界 (targets追記の空親テーブル省略)
// 空の中間テーブルヘッダを出さない外部挙動を完全一致で検証する。
func TestAppendOmitsEmptyParentHeaders(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendTargetEntry(p, "~/.gitconfig", "wsl", "dotfiles/wsl/.gitconfig"); err != nil {
		t.Fatalf("Append error: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := "[entries.\"~/.gitconfig\".targets.wsl]\nsrc = \"dotfiles/wsl/.gitconfig\"\n"
	if string(data) != want {
		t.Errorf("空親テーブル省略の完全一致失敗:\ngot:\n%s\nwant:\n%s", data, want)
	}
	back, err := Load(p)
	if err != nil {
		t.Fatalf("省略形のLoad error: %v", err)
	}
	if got := back.Resolve("wsl"); len(got) != 1 || got[0].Src != "dotfiles/wsl/.gitconfig" {
		t.Errorf("省略形の往復後解決不正: %+v", got)
	}
}

// Seam: config パッケージ公開境界 (add向け追記の空行区切り)
// 非空ファイルへの追記は空行1行で区切り、空行済みは重ねない外部挙動を検証する。
func TestAppendInsertsBlankLineSeparator(t *testing.T) {
	t.Run("単一改行終わりは空行1行を挿む", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "[entries]\n\"~/.bashrc\" = { src = \"dotfiles/.bashrc\" }\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendPlainEntry(p, "~/.vimrc", "dotfiles/.vimrc"); err != nil {
			t.Fatalf("Append error: %v", err)
		}
		got := readFileForTest(t, p)
		want := before + "\n[entries.\"~/.vimrc\"]\nsrc = \"dotfiles/.vimrc\"\n"
		if got != want {
			t.Errorf("空行区切り失敗:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("空行終わりは重ねない", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "[entries]\n\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendPlainEntry(p, "~/.vimrc", "dotfiles/.vimrc"); err != nil {
			t.Fatalf("Append error: %v", err)
		}
		got := readFileForTest(t, p)
		want := before + "[entries.\"~/.vimrc\"]\nsrc = \"dotfiles/.vimrc\"\n"
		if got != want {
			t.Errorf("空行重複失敗:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("末尾改行なしも空行で区切る", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		if err := os.WriteFile(p, []byte("[entries]"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendPlainEntry(p, "~/.vimrc", "dotfiles/.vimrc"); err != nil {
			t.Fatalf("Append error: %v", err)
		}
		got := readFileForTest(t, p)
		want := "[entries]\n\n[entries.\"~/.vimrc\"]\nsrc = \"dotfiles/.vimrc\"\n"
		if got != want {
			t.Errorf("末尾改行なしの区切り失敗:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
}

// Seam: config パッケージ公開境界 (add向け追記記法)
// テーブル形式・[entries]親ヘッダ行なし・インデントなし・追記間は空行1行の完全一致を検証する。
func TestAppendExactFormat(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendPlainEntry(p, "~/.bashrc", "dotfiles/.bashrc"); err != nil {
		t.Fatal(err)
	}
	if err := AppendPlainEntry(p, "~/.vimrc", "dotfiles/.vimrc"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := "[entries.\"~/.bashrc\"]\nsrc = \"dotfiles/.bashrc\"\n\n[entries.\"~/.vimrc\"]\nsrc = \"dotfiles/.vimrc\"\n"
	if string(data) != want {
		t.Errorf("保存記法の完全一致失敗:\ngot:\n%s\nwant:\n%s", data, want)
	}
	if strings.Contains(string(data), "[entries]\n") {
		t.Errorf("[entries]親ヘッダ行なし expected, got:\n%s", data)
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			t.Errorf("インデントなし expected, got line %q", line)
		}
	}
}

// Seam: config パッケージ公開境界 (add向け追記の往復)
// 追記物が既存読み込み（ヘッダあり・なし・インライン・targets・override混在）で読める外部挙動を検証する。
func TestAppendRoundTripMixed(t *testing.T) {
	body := "[entries]\n" +
		`"~/.a" = { src = "dotfiles/.a" }` + "\n" +
		`"~/.b" = { src = "dotfiles/.b", override = false }` + "\n" +
		`"~/.c" = { targets = { win = { src = "dotfiles/win/.c" }, wsl = { src = "dotfiles/wsl/.c", override = false } } }` + "\n"
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.toml")
	if err := os.WriteFile(srcPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(srcPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if err := cfg.Add("~/.d", "dotfiles/.d"); err != nil {
		t.Fatalf("Add error: %v", err)
	}
	dstPath := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(dstPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendPlainEntry(dstPath, "~/.d", "dotfiles/.d"); err != nil {
		t.Fatalf("Append error: %v", err)
	}
	back, err := Load(dstPath)
	if err != nil {
		t.Fatalf("保存物のLoad error: %v\n保存物:\n%s", err, readFileForTest(t, dstPath))
	}
	if len(back.Entries) != 4 {
		t.Fatalf("往復後のEntries = %d件, want 4件", len(back.Entries))
	}
	for _, target := range []string{"", "win", "wsl"} {
		want := cfg.Resolve(target)
		got := back.Resolve(target)
		if len(got) != len(want) {
			t.Fatalf("Resolve(%q) 件数不一致: got %+v, want %+v", target, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("Resolve(%q)[%d] = %+v, want %+v", target, i, got[i], want[i])
			}
		}
	}
}

// Seam: config パッケージ公開境界 (add向け追記の特殊文字往復)
// 特殊文字を含むキーでも往復できる外部挙動を検証する。
func TestAppendRoundTripSpecialChars(t *testing.T) {
	dests := []string{`~/.config/a"b`, `~/a\b`, `~/sp ace`, `~/.config/#hash`}
	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range dests {
		src, err := SrcForDest(d)
		if err != nil {
			t.Fatalf("SrcForDest(%q) error: %v", d, err)
		}
		if err := AppendPlainEntry(p, d, src); err != nil {
			t.Fatalf("Append(%q) error: %v", d, err)
		}
	}
	back, err := Load(p)
	if err != nil {
		t.Fatalf("特殊文字保存物のLoad error: %v\n保存物:\n%s", err, readFileForTest(t, p))
	}
	if len(back.Entries) != len(dests) {
		t.Fatalf("往復後のEntries = %d件, want %d件", len(back.Entries), len(dests))
	}
	for _, d := range dests {
		v, ok := back.Entries[d]
		if !ok {
			t.Errorf("往復後にキー消失: %q", d)
			continue
		}
		if v.Src == nil {
			t.Errorf("%q: src消失", d)
			continue
		}
		wantSrc, _ := SrcForDest(d)
		if *v.Src != wantSrc {
			t.Errorf("%q: src = %q, want %q", d, *v.Src, wantSrc)
		}
	}
}

// Seam: config パッケージ公開境界 (fragment追記)
// 元ファイル温存・末尾改行なし・targets追記マージ・
// 重複時不変を外部挙動で検証する。特殊文字往復は TestAppendRoundTripSpecialChars に寄せる。
func TestAppendPreservesAndMerges(t *testing.T) {
	t.Run("コメント温存して追記", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "# c\n[entries]\n\"~/.bashrc\" = { src = \"dotfiles/.bashrc\" }\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendPlainEntry(p, "~/.vimrc", "dotfiles/.vimrc"); err != nil {
			t.Fatalf("Append error: %v", err)
		}
		got := readFileForTest(t, p)
		if !strings.HasPrefix(got, before) {
			t.Errorf("既存バイト温存失敗:\nbefore:\n%s\ngot:\n%s", before, got)
		}
		back, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if len(back.Entries) != 2 {
			t.Fatalf("Entries = %d, want 2", len(back.Entries))
		}
	})
	t.Run("末尾改行なしでも壊れない", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		if err := os.WriteFile(p, []byte("[entries]\n\"~/.a\" = { src = \"a\" }"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendPlainEntry(p, "~/.b", "b"); err != nil {
			t.Fatalf("Append error: %v", err)
		}
		if _, err := Load(p); err != nil {
			t.Fatalf("Load error: %v\n%s", err, readFileForTest(t, p))
		}
	})
	t.Run("targets追記マージ", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "[entries.\"~/.vimrc\".targets.win]\nsrc = \"dotfiles/win/.vimrc\"\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendTargetEntry(p, "~/.vimrc", "wsl", "dotfiles/wsl/.vimrc"); err != nil {
			t.Fatalf("Append error: %v", err)
		}
		back, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v\n%s", err, readFileForTest(t, p))
		}
		if got := back.Resolve("win"); len(got) != 1 {
			t.Errorf("win消失: %+v", got)
		}
		if got := back.Resolve("wsl"); len(got) != 1 {
			t.Errorf("wsl消失: %+v", got)
		}
	})
	t.Run("重複は失敗し不変", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "[entries.\"~/.vimrc\"]\nsrc = \"dotfiles/.vimrc\"\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendPlainEntry(p, "~/.vimrc", "dotfiles/.vimrc"); err == nil {
			t.Fatal("重複追記: エラー expected, got nil")
		}
		if got := readFileForTest(t, p); got != before {
			t.Errorf("不変 expected:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})
}

func readFileForTest(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		return "(read error: " + err.Error() + ")"
	}
	return string(data)
}
