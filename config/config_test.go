package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: config パッケージ公開境界 (Target フィルタ)
// Spec #1 の Target フィルタ仕様を外部挙動として検証する。
func TestFilterByTarget(t *testing.T) {
	commonEntry := Entry{Src: "a", Dest: "~/.a"}
	commonExplicit := Entry{Src: "b", Dest: "~/.b", Target: TargetList{"common"}}
	winEntry := Entry{Src: "c", Dest: "~/.c", Target: TargetList{"win"}}
	multiEntry := Entry{Src: "d", Dest: "~/.d", Target: TargetList{"win", "wsl"}}
	wslEntry := Entry{Src: "e", Dest: "~/.e", Target: TargetList{"wsl"}}

	all := []Entry{commonEntry, commonExplicit, winEntry, multiEntry, wslEntry}

	tests := []struct {
		name   string
		target string
		want   []string
	}{
		{"Target未指定はcommonのみ", "", []string{"a", "b"}},
		{"Target指定winはcommon+win", "win", []string{"a", "b", "c", "d"}},
		{"Target指定wslはcommon+wsl(配列一致含む)", "wsl", []string{"a", "b", "d", "e"}},
		{"該当なしTargetはcommonのみ", "linux", []string{"a", "b"}},
		{"Target指定commonはcommonのみと同等", "common", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterByTarget(all, tt.target)
			if len(got) != len(tt.want) {
				t.Fatalf("FilterByTarget(%q) = %d件, want %d件 (%v)", tt.target, len(got), len(tt.want), tt.want)
			}
			for i, w := range tt.want {
				if got[i].Src != w {
					t.Errorf("index %d: got Src=%q, want %q", i, got[i].Src, w)
				}
			}
		})
	}
}

// Seam: config パッケージ公開境界 (Store の mdots.yaml 読込・validation)
// Entry の src/dest 必須と target の string | string[] を外部挙動で検証する。
func TestLoadStoreConfig(t *testing.T) {
	t.Run("stringと配列のTargetを読み込める", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.yaml")
		body := "entries:\n" +
			"  - src: vimrc\n    dest: ~/.vimrc\n" +
			"  - src: wezterm.lua\n    dest: ~/.config/wezterm/wezterm.lua\n    target: win\n" +
			"  - src: shared.conf\n    dest: ~/.config/shared.conf\n    target: [win, wsl]\n" +
			"  - src: common.conf\n    dest: ~/.config/common.conf\n    target: common\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if len(cfg.Entries) != 4 {
			t.Fatalf("Entries = %d件, want 4件", len(cfg.Entries))
		}
		if len(cfg.Entries[0].Target) != 0 {
			t.Errorf("省略Targetはcommon扱い: got %v", cfg.Entries[0].Target)
		}
		if len(cfg.Entries[2].Target) != 2 || cfg.Entries[2].Target[0] != "win" {
			t.Errorf("配列Targetの読み込み不正: got %v", cfg.Entries[2].Target)
		}
	})

	t.Run("src/dest必須のvalidation", func(t *testing.T) {
		dir := t.TempDir()
		cases := map[string]string{
			"src欠落":     "entries:\n  - dest: ~/.a\n",
			"dest欠落":    "entries:\n  - src: a\n",
			"target型不正": "entries:\n  - src: a\n    dest: ~/.a\n    target: 123\n",
		}
		for name, body := range cases {
			p := filepath.Join(dir, "mdots.yaml")
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(p); err == nil {
				t.Errorf("%s: エラー expected, got nil", name)
			}
		}
	})
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

// Seam: config パッケージ公開境界 (Store 発見)
// カレント直下の mdots.yaml のみを参照する外部挙動を t.TempDir() の実FSで検証する。
func TestFindStore(t *testing.T) {
	t.Run("カレント直下のStoreを発見できる", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "mdots.yaml"), []byte("entries: []\n"), 0o644); err != nil {
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
		if err := os.WriteFile(filepath.Join(root, "mdots.yaml"), []byte("entries: []\n"), 0o644); err != nil {
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
		if !strings.Contains(err.Error(), "mdots.yaml not found in "+sub) {
			t.Errorf("エラーメッセージ不正: got %q, want contain %q", err.Error(), "mdots.yaml not found in "+sub)
		}
	})

	t.Run("見つからないときはin形式のエラー", func(t *testing.T) {
		start := t.TempDir()
		_, err := FindStore(start)
		if err == nil {
			t.Fatal("エラー expected, got nil")
		}
		if !strings.Contains(err.Error(), "mdots.yaml not found in "+start) {
			t.Errorf("エラーメッセージ不正: got %q, want contain %q", err.Error(), "mdots.yaml not found in "+start)
		}
		if strings.Contains(err.Error(), "searched from") {
			t.Errorf("旧文言が残っている: got %q", err.Error())
		}
	})
}
