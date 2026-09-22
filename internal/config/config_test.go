package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: config パッケージ公開境界 (単一 Target 解決)
// targets.<名>.<dest> のみを解決し、省略時は default_target へ解決する外部挙動を検証する。
// 同一 dest の跨 Target 定義は許可し、結果は配置先ソート順。
func TestResolve(t *testing.T) {
	body := "default_target = \"base\"\n" +
		"[targets.base.\"~/.b\"]\nsrc = \"b-base\"\n" +
		"[targets.wsl.\"~/.b\"]\nsrc = \"b-wsl\"\n" +
		"[targets.base.\"~/.a\"]\nsrc = \"a\"\n"
	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	t.Run("省略時は既定へ解決", func(t *testing.T) {
		got, err := cfg.Resolve("")
		if err != nil {
			t.Fatalf("Resolve error: %v", err)
		}
		want := []Entry{{Src: "a", Dest: "~/.a", Override: true}, {Src: "b-base", Dest: "~/.b", Override: true}}
		if len(got) != len(want) {
			t.Fatalf("Resolve(\"\") = %+v, want %+v", got, want)
		}
		for i, w := range want {
			if got[i] != w {
				t.Errorf("index %d: got %+v, want %+v", i, got[i], w)
			}
		}
	})

	t.Run("明示は単一のみ", func(t *testing.T) {
		got, err := cfg.Resolve("wsl")
		if err != nil {
			t.Fatalf("Resolve error: %v", err)
		}
		if len(got) != 1 || got[0].Src != "b-wsl" || got[0].Dest != "~/.b" {
			t.Errorf("Resolve(wsl) = %+v, want single b-wsl", got)
		}
	})

	t.Run("未知の明示はエラー", func(t *testing.T) {
		if _, err := cfg.Resolve("linux"); err == nil {
			t.Error("未知Target: エラー expected, got nil")
		} else {
			for _, want := range []string{"linux", "default_target", "targets"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("未知エラーは %q を含むべき, got %q", want, err.Error())
				}
			}
		}
	})

	t.Run("配置先ソート順", func(t *testing.T) {
		got, err := cfg.Resolve("")
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(got); i++ {
			if got[i-1].Dest >= got[i].Dest {
				t.Errorf("解決結果が配置先ソート順でない: %+v", got)
				break
			}
		}
	})
}

// Seam: config パッケージ公開境界 (既定解決のエラー)
// 欠落・空文字・未定義を指す default_target を解決時に失敗させる外部挙動を検証する。
func TestResolveDefaultErrors(t *testing.T) {
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

	t.Run("欠落は省略時にエラー", func(t *testing.T) {
		cfg := load(t, "[targets.base.\"~/.a\"]\nsrc = \"a\"\n")
		if _, err := cfg.Resolve(""); err == nil {
			t.Error("エラー expected, got nil")
		} else if !strings.Contains(err.Error(), "default_target") {
			t.Errorf("default_target を含むべき, got %q", err.Error())
		}
	})

	t.Run("空文字は省略時にエラー", func(t *testing.T) {
		cfg := load(t, "default_target = \"\"\n[targets.base.\"~/.a\"]\nsrc = \"a\"\n")
		if _, err := cfg.Resolve(""); err == nil {
			t.Error("エラー expected, got nil")
		} else if !strings.Contains(err.Error(), "default_target") {
			t.Errorf("default_target を含むべき, got %q", err.Error())
		}
	})

	t.Run("未定義を指す既定はエラー", func(t *testing.T) {
		cfg := load(t, "default_target = \"nope\"\n[targets.base.\"~/.a\"]\nsrc = \"a\"\n")
		if _, err := cfg.Resolve(""); err == nil {
			t.Error("エラー expected, got nil")
		} else {
			for _, want := range []string{"nope", "default_target", "targets"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("既定エラーは %q を含むべき, got %q", want, err.Error())
				}
			}
		}
	})

	t.Run("明示は欠落時も既知なら通る", func(t *testing.T) {
		cfg := load(t, "[targets.base.\"~/.a\"]\nsrc = \"a\"\n")
		got, err := cfg.Resolve("base")
		if err != nil {
			t.Fatalf("Resolve(base) error: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Resolve(base) = %+v, want 1件", got)
		}
	})
}

// Seam: config パッケージ公開境界 (Target 一覧・既定印)
// 集約・重複排除・ソートと既定印の外部挙動を検証する。印は " (default)" で固定する。
func TestTargets(t *testing.T) {
	t.Run("分散したTargetを重複排除・ソートして返す", func(t *testing.T) {
		body := "default_target = \"win\"\n" +
			"[targets.wsl.\"~/.c\"]\nsrc = \"c-wsl\"\n" +
			"[targets.win.\"~/.c\"]\nsrc = \"c-win\"\n" +
			"[targets.linux.\"~/.b\"]\nsrc = \"b-linux\"\n"
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
		marked := cfg.TargetsMarked()
		wantMarked := []string{"linux", "win (default)", "wsl"}
		if len(marked) != len(wantMarked) {
			t.Fatalf("TargetsMarked() = %q, want %q", marked, wantMarked)
		}
		for i, w := range wantMarked {
			if marked[i] != w {
				t.Errorf("marked index %d: got %q, want %q", i, marked[i], w)
			}
		}
	})

	t.Run("同一destの跨Targetは別Targetとして集約", func(t *testing.T) {
		body := "default_target = \"base\"\n" +
			"[targets.base.\"~/.a\"]\nsrc = \"a-base\"\n" +
			"[targets.wsl.\"~/.a\"]\nsrc = \"a-wsl\"\n"
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
		if len(got) != 2 || got[0] != "base" || got[1] != "wsl" {
			t.Errorf("Targets() = %q, want [base wsl]", got)
		}
	})

	t.Run("Target未定義時は空を返す", func(t *testing.T) {
		body := "default_target = \"base\"\n"
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
		if got := cfg.TargetsMarked(); len(got) != 0 {
			t.Errorf("TargetsMarked() = %q, want empty", got)
		}
	})
}

// Seam: config パッケージ公開境界 (新形式の読込)
// targets.<名>.<dest> の読込を外部挙動で検証する。
func TestLoadNewFormat(t *testing.T) {
	t.Run("テーブル形式を読み込める", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		body := "default_target = \"base\"\n" +
			"[targets.base.\"~/.config/starship.toml\"]\nsrc = \"dotfiles/base/.config/starship.toml\"\n" +
			"[targets.wsl.\"~/.gitconfig\"]\nsrc = \"dotfiles/wsl/.gitconfig\"\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if cfg.DefaultTarget != "base" {
			t.Errorf("DefaultTarget = %q, want base", cfg.DefaultTarget)
		}
		got, err := cfg.Resolve("base")
		if err != nil {
			t.Fatalf("Resolve error: %v", err)
		}
		if len(got) != 1 || got[0].Dest != "~/.config/starship.toml" {
			t.Errorf("解決不正: %+v", got)
		}
	})

	t.Run("同一destの跨Targetを読み込める", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		body := "default_target = \"base\"\n" +
			"[targets.base.\"~/.gitconfig\"]\nsrc = \"dotfiles/base/.gitconfig\"\n" +
			"[targets.wsl.\"~/.gitconfig\"]\nsrc = \"dotfiles/wsl/.gitconfig\"\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		base, err := cfg.Resolve("base")
		if err != nil {
			t.Fatal(err)
		}
		wsl, err := cfg.Resolve("wsl")
		if err != nil {
			t.Fatal(err)
		}
		if len(base) != 1 || base[0].Src != "dotfiles/base/.gitconfig" {
			t.Errorf("base解決不正: %+v", base)
		}
		if len(wsl) != 1 || wsl[0].Src != "dotfiles/wsl/.gitconfig" {
			t.Errorf("wsl解決不正: %+v", wsl)
		}
	})

	t.Run("宣言順が逆でも解決は配置先ソート順", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		body := "default_target = \"base\"\n" +
			"[targets.base.\"~/.z\"]\nsrc = \"z\"\n" +
			"[targets.base.\"~/.m\"]\nsrc = \"m\"\n" +
			"[targets.base.\"~/.a\"]\nsrc = \"a\"\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		got, err := cfg.Resolve("")
		if err != nil {
			t.Fatal(err)
		}
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
// 空・欠落を明確に失敗させる外部挙動を検証する。
func TestLoadValidationErrors(t *testing.T) {
	cases := map[string]string{
		"空targetは拒否":      "default_target = \"base\"\n[targets.\"\".\"~/.a\"]\nsrc = \"a\"\n",
		"空destは拒否":        "default_target = \"base\"\n[targets.base.\"\"]\nsrc = \"a\"\n",
		"空srcは拒否":         "default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"\"\n",
		"src欠落は拒否":        "default_target = \"base\"\n[targets.base.\"~/.a\"]\noverride = false\n",
		"targets非テーブルは拒否": "default_target = \"base\"\ntargets = \"x\"\n",
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

// Seam: config パッケージ公開境界 (想定外形式の拒否・回帰)
// 旧 entries 形式と無指定 Entry 形式を拒否する外部挙動を検証する。
// 従来「未知 Target でも exit 0」だった黙示フォールバックは廃止しエラーにする（反転テストは解決側で固定）。
func TestLoadRejectsUnexpectedFormat(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"旧[[entries]]配列は拒否": {"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n", "unknown field"},
		"旧[entries]テーブルは拒否": {"[entries.\"~/.a\"]\nsrc = \"a\"\n", "unknown field"},
		"無指定Entry形式は拒否":     {"default_target = \"base\"\n[entries]\n\"~/.a\" = { src = \"a\" }\n", "unknown field"},
		"[common]テーブルは拒否":   {"default_target = \"base\"\n[common.\"~/.a\"]\nsrc = \"a\"\n", "unknown field"},
		"旧target配列は拒否":      {"default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a\"\ntarget = [\"win\"]\n", "unknown field"},
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
// default_target = "base" と shared_dir = "dotfiles/shared" を持ち、コメント・例示は含まない。
// 生成物は Load を通り（Entry ゼロ件）、旧形式を含まない。
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
	want := "default_target = \"base\"\nshared_dir = \"dotfiles/shared\"\n"
	if body != want {
		t.Errorf("template exact match failed:\ngot:\n%s\nwant:\n%s", body, want)
	}
	if strings.Contains(body, "[entries") || strings.Contains(body, "[[entries]]") {
		t.Errorf("template must not contain entries, got:\n%s", body)
	}
	if strings.Contains(body, "[common") {
		t.Errorf("template must not contain common, got:\n%s", body)
	}
	if strings.Contains(body, "= {") {
		t.Errorf("template must not contain inline form, got:\n%s", body)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("generated template must Load: %v", err)
	}
	if cfg.DefaultTarget != "base" {
		t.Errorf("DefaultTarget = %q, want base", cfg.DefaultTarget)
	}
	if cfg.SharedDir != "dotfiles/shared" {
		t.Errorf("SharedDir = %q, want dotfiles/shared", cfg.SharedDir)
	}
	if len(cfg.Targets()) != 0 {
		t.Errorf("template should have zero entries, got Targets() = %q", cfg.Targets())
	}
	if _, err := Init(dir); err == nil {
		t.Fatal("second Init: エラー expected, got nil")
	} else if !strings.Contains(err.Error(), "mdots.toml already exists in ") {
		t.Errorf("既存ありエラーメッセージ不正: got %q", err.Error())
	}
}

// Seam: config パッケージ公開境界 (Store 発見)
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
	})
}

// Seam: config パッケージ公開境界 (override の読み・解決)
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
		cfg := load(t, "default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a\"\n")
		got, err := cfg.Resolve("")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || !got[0].Override {
			t.Errorf("省略時は Override=true expected, got %+v", got)
		}
	})

	t.Run("falseを解決できる", func(t *testing.T) {
		cfg := load(t, "default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a\"\noverride = false\n")
		got, err := cfg.Resolve("")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Override {
			t.Errorf("Override=false expected, got %+v", got)
		}
	})

	t.Run("Targetごとに変えられる", func(t *testing.T) {
		body := "default_target = \"base\"\n" +
			"[targets.base.\"~/.a\"]\nsrc = \"a-base\"\noverride = false\n" +
			"[targets.wsl.\"~/.a\"]\nsrc = \"a-wsl\"\n"
		cfg := load(t, body)
		gotBase, err := cfg.Resolve("base")
		if err != nil {
			t.Fatal(err)
		}
		if len(gotBase) != 1 || gotBase[0].Override {
			t.Errorf("base は false expected, got %+v", gotBase)
		}
		gotWsl, err := cfg.Resolve("wsl")
		if err != nil {
			t.Fatal(err)
		}
		if len(gotWsl) != 1 || !gotWsl[0].Override {
			t.Errorf("wsl は省略時 true expected, got %+v", gotWsl)
		}
	})

	t.Run("overrideは選択に影響しない", func(t *testing.T) {
		body := "default_target = \"base\"\n" +
			"[targets.base.\"~/.a\"]\nsrc = \"a\"\noverride = false\n" +
			"[targets.wsl.\"~/.b\"]\nsrc = \"b\"\noverride = false\n"
		cfg := load(t, body)
		got, err := cfg.Resolve("base")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Dest != "~/.a" {
			t.Errorf("base は1件 expected, got %+v", got)
		}
		if _, err := cfg.Resolve("linux"); err == nil {
			t.Error("未知Targetはエラー expected, got nil")
		}
	})
}

// Seam: config パッケージ公開境界 (override の検証エラー)
func TestLoadOverrideValidationErrors(t *testing.T) {
	cases := map[string]string{
		"文字列は拒否":     "default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a\"\noverride = \"yes\"\n",
		"数値は拒否":      "default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a\"\noverride = 1\n",
		"未知フィールドは拒否": "default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a\"\noverwride = false\n",
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

// Seam: config パッケージ公開境界 (add向けsrc算出・target付き)
// dotfiles/<target>/... への写像と空target拒否の外部挙動を検証する。
func TestSrcForDestWithTarget(t *testing.T) {
	if got, err := SrcForDestWithTarget("~/.vimrc", "base"); err != nil || got != "dotfiles/base/.vimrc" {
		t.Errorf("SrcForDestWithTarget(~/.vimrc, base) = %q, %v; want %q, nil", got, err, "dotfiles/base/.vimrc")
	}
	if got, err := SrcForDestWithTarget("~/.config/helix/config.toml", "wsl"); err != nil || got != "dotfiles/wsl/.config/helix/config.toml" {
		t.Errorf("SrcForDestWithTarget深い階層 = %q, %v", got, err)
	}
	if got, err := SrcForDestWithTarget("~", "base"); err != nil || got != "dotfiles/base" {
		t.Errorf("SrcForDestWithTarget(~, base) = %q, %v; want %q, nil", got, err, "dotfiles/base")
	}
	if _, err := SrcForDestWithTarget("~/.vimrc", ""); err == nil {
		t.Error("空targetは無指定 plain 廃止のためエラー expected, got nil")
	}
	if _, err := SrcForDestWithTarget("/etc/hosts", "base"); err == nil {
		t.Error("SrcForDestWithTarget(絶対パス): エラー expected, got nil")
	}
}

// Seam: config パッケージ公開境界 (add向け登録・target付き)
// 同一destの跨Target許可・同一キーのみ重複エラー・overrideを書かない外部挙動を検証する。
func TestAddTargetRegisters(t *testing.T) {
	var cfg Config
	if err := cfg.AddTarget("~/.vimrc", "base", "dotfiles/base/.vimrc"); err != nil {
		t.Fatalf("AddTarget error: %v", err)
	}
	cfg.DefaultTarget = "base"
	got, err := cfg.Resolve("base")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Dest != "~/.vimrc" || got[0].Src != "dotfiles/base/.vimrc" || !got[0].Override {
		t.Errorf("登録後の解決不正: %+v", got)
	}
	if _, err := cfg.Resolve("wsl"); err == nil {
		t.Error("未登録Targetの解決はエラー expected, got nil")
	}
	// 同一destの跨Targetは許可する。
	if err := cfg.AddTarget("~/.vimrc", "wsl", "dotfiles/wsl/.vimrc"); err != nil {
		t.Fatalf("跨Target追記 error: %v", err)
	}
	wsl, err := cfg.Resolve("wsl")
	if err != nil {
		t.Fatal(err)
	}
	if len(wsl) != 1 || wsl[0].Src != "dotfiles/wsl/.vimrc" {
		t.Errorf("追記後のwsl解決不正: %+v", wsl)
	}
	if err := cfg.AddTarget("~/.vimrc", "base", "dotfiles/base/.vimrc"); err == nil {
		t.Error("同一キーの再登録: エラー expected, got nil")
	} else if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("同一キーエラー文言不正: got %q", err.Error())
	}
	for _, tc := range []struct{ dest, target, src string }{
		{"", "base", "a"},
		{"~/.a", "", "a"},
		{"~/.a", "base", ""},
	} {
		var c Config
		if err := c.AddTarget(tc.dest, tc.target, tc.src); err == nil {
			t.Errorf("AddTarget(%q,%q,%q): エラー expected, got nil", tc.dest, tc.target, tc.src)
		}
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(p, []byte("default_target = \"base\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendTargetEntry(p, "~/.vimrc", "base", "dotfiles/base/.vimrc"); err != nil {
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
	baseGot, err := back.Resolve("base")
	if err != nil {
		t.Fatal(err)
	}
	if len(baseGot) != 1 || baseGot[0].Src != "dotfiles/base/.vimrc" {
		t.Errorf("往復後のbase解決不正: %+v", baseGot)
	}
}

// Seam: config パッケージ公開境界 (targets追記の空親テーブル省略)
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
	want := "[targets.wsl.\"~/.gitconfig\"]\nsrc = \"dotfiles/wsl/.gitconfig\"\n"
	if string(data) != want {
		t.Errorf("空親テーブル省略の完全一致失敗:\ngot:\n%s\nwant:\n%s", data, want)
	}
	back, err := Load(p)
	if err != nil {
		t.Fatalf("省略形のLoad error: %v", err)
	}
	got, err := back.Resolve("wsl")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Src != "dotfiles/wsl/.gitconfig" {
		t.Errorf("省略形の往復後解決不正: %+v", got)
	}
}

// Seam: config パッケージ公開境界 (add向け追記の空行区切り)
func TestAppendInsertsBlankLineSeparator(t *testing.T) {
	t.Run("単一改行終わりは空行1行を挿む", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "default_target = \"base\"\n\n[targets.base.\"~/.bashrc\"]\nsrc = \"dotfiles/base/.bashrc\"\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendTargetEntry(p, "~/.vimrc", "base", "dotfiles/base/.vimrc"); err != nil {
			t.Fatalf("Append error: %v", err)
		}
		got := readFileForTest(t, p)
		want := before + "\n[targets.base.\"~/.vimrc\"]\nsrc = \"dotfiles/base/.vimrc\"\n"
		if got != want {
			t.Errorf("空行区切り失敗:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("空行終わりは重ねない", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "default_target = \"base\"\n\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendTargetEntry(p, "~/.vimrc", "base", "dotfiles/base/.vimrc"); err != nil {
			t.Fatalf("Append error: %v", err)
		}
		got := readFileForTest(t, p)
		want := before + "[targets.base.\"~/.vimrc\"]\nsrc = \"dotfiles/base/.vimrc\"\n"
		if got != want {
			t.Errorf("空行重複失敗:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("末尾改行なしも空行で区切る", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		if err := os.WriteFile(p, []byte("default_target = \"base\""), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendTargetEntry(p, "~/.vimrc", "base", "dotfiles/base/.vimrc"); err != nil {
			t.Fatalf("Append error: %v", err)
		}
		got := readFileForTest(t, p)
		want := "default_target = \"base\"\n\n[targets.base.\"~/.vimrc\"]\nsrc = \"dotfiles/base/.vimrc\"\n"
		if got != want {
			t.Errorf("末尾改行なしの区切り失敗:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
}

// Seam: config パッケージ公開境界 (add向け追記記法)
func TestAppendExactFormat(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendTargetEntry(p, "~/.bashrc", "base", "dotfiles/base/.bashrc"); err != nil {
		t.Fatal(err)
	}
	if err := AppendTargetEntry(p, "~/.vimrc", "base", "dotfiles/base/.vimrc"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := "[targets.base.\"~/.bashrc\"]\nsrc = \"dotfiles/base/.bashrc\"\n\n[targets.base.\"~/.vimrc\"]\nsrc = \"dotfiles/base/.vimrc\"\n"
	if string(data) != want {
		t.Errorf("保存記法の完全一致失敗:\ngot:\n%s\nwant:\n%s", data, want)
	}
	if strings.Contains(string(data), "[targets]\n") {
		t.Errorf("[targets]親ヘッダ行なし expected, got:\n%s", data)
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			t.Errorf("インデントなし expected, got line %q", line)
		}
	}
}

// Seam: config パッケージ公開境界 (add向け追記の往復)
func TestAppendRoundTripMixed(t *testing.T) {
	body := "default_target = \"base\"\n" +
		"[targets.base.\"~/.a\"]\nsrc = \"dotfiles/base/.a\"\n" +
		"[targets.base.\"~/.b\"]\nsrc = \"dotfiles/base/.b\"\noverride = false\n" +
		"[targets.wsl.\"~/.c\"]\nsrc = \"dotfiles/wsl/.c\"\n"
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.toml")
	if err := os.WriteFile(srcPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(srcPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if err := cfg.AddTarget("~/.d", "base", "dotfiles/base/.d"); err != nil {
		t.Fatalf("Add error: %v", err)
	}
	dstPath := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(dstPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendTargetEntry(dstPath, "~/.d", "base", "dotfiles/base/.d"); err != nil {
		t.Fatalf("Append error: %v", err)
	}
	back, err := Load(dstPath)
	if err != nil {
		t.Fatalf("保存物のLoad error: %v\n保存物:\n%s", err, readFileForTest(t, dstPath))
	}
	if len(back.TargetsMap["base"]) != 3 {
		t.Fatalf("往復後のbase件数 = %d, want 3", len(back.TargetsMap["base"]))
	}
	for _, target := range []string{"base", "wsl"} {
		want, err := cfg.Resolve(target)
		if err != nil {
			t.Fatal(err)
		}
		got, err := back.Resolve(target)
		if err != nil {
			t.Fatal(err)
		}
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
func TestAppendRoundTripSpecialChars(t *testing.T) {
	dests := []string{`~/.config/a"b`, `~/a\b`, `~/sp ace`, `~/.config/#hash`}
	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range dests {
		src, err := SrcForDestWithTarget(d, "base")
		if err != nil {
			t.Fatalf("SrcForDest(%q) error: %v", d, err)
		}
		if err := AppendTargetEntry(p, d, "base", src); err != nil {
			t.Fatalf("Append(%q) error: %v", d, err)
		}
	}
	back, err := Load(p)
	if err != nil {
		t.Fatalf("特殊文字保存物のLoad error: %v\n保存物:\n%s", err, readFileForTest(t, p))
	}
	if len(back.TargetsMap["base"]) != len(dests) {
		t.Fatalf("往復後のEntries = %d件, want %d件", len(back.TargetsMap["base"]), len(dests))
	}
	for _, d := range dests {
		v, ok := back.TargetsMap["base"][d]
		if !ok {
			t.Errorf("往復後にキー消失: %q", d)
			continue
		}
		if v.Src == nil {
			t.Errorf("%q: src消失", d)
			continue
		}
		wantSrc, _ := SrcForDestWithTarget(d, "base")
		if *v.Src != wantSrc {
			t.Errorf("%q: src = %q, want %q", d, *v.Src, wantSrc)
		}
	}
}

// Seam: config パッケージ公開境界 (fragment追記)
func TestAppendPreservesAndMerges(t *testing.T) {
	t.Run("コメント温存して追記", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "default_target = \"base\"\n# c\n[targets.base.\"~/.bashrc\"]\nsrc = \"dotfiles/base/.bashrc\"\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendTargetEntry(p, "~/.vimrc", "base", "dotfiles/base/.vimrc"); err != nil {
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
		if len(back.TargetsMap["base"]) != 2 {
			t.Fatalf("Entries = %d, want 2", len(back.TargetsMap["base"]))
		}
	})
	t.Run("末尾改行なしでも壊れない", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		if err := os.WriteFile(p, []byte("default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a\""), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendTargetEntry(p, "~/.b", "base", "b"); err != nil {
			t.Fatalf("Append error: %v", err)
		}
		if _, err := Load(p); err != nil {
			t.Fatalf("Load error: %v\n%s", err, readFileForTest(t, p))
		}
	})
	t.Run("跨Targetの同一destは追記できる", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"dotfiles/base/.vimrc\"\n"
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
		base, err := back.Resolve("base")
		if err != nil {
			t.Fatal(err)
		}
		if len(base) != 1 {
			t.Errorf("base消失: %+v", base)
		}
		wsl, err := back.Resolve("wsl")
		if err != nil {
			t.Fatal(err)
		}
		if len(wsl) != 1 {
			t.Errorf("wsl消失: %+v", wsl)
		}
	})
	t.Run("同一キーの重複は失敗し不変", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "[targets.base.\"~/.vimrc\"]\nsrc = \"dotfiles/base/.vimrc\"\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendTargetEntry(p, "~/.vimrc", "base", "dotfiles/base/.vimrc"); err == nil {
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
