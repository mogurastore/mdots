package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: config パッケージ公開境界 (sort 正規形)
// 意味ソートで Target 名・dest の辞書順に並べ、追記と同じ書式
// （空親ヘッダなし・インデントなし・空行1行区切り・末尾単一改行）にする。
func TestSortFileSortsTargetsAndDests(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "mdots.toml")
	before := "default_target = \"base\"\n" +
		"[targets.win.\"~/.gitconfig\"]\nsrc = \"dotfiles/win/.gitconfig\"\n" +
		"\n" +
		"[targets.wsl.\"~/.gitconfig\"]\nsrc = \"dotfiles/wsl/.gitconfig\"\n" +
		"\n" +
		"[targets.wsl.\"~/.config/mise/config.toml\"]\nsrc = \"dotfiles/wsl/.config/mise/config.toml\"\n" +
		"\n" +
		"[targets.win.\"~/.config/mise/config.toml\"]\nsrc = \"dotfiles/win/.config/mise/config.toml\"\n"
	if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SortFile(p)
	if err != nil {
		t.Fatalf("SortFile error: %v", err)
	}
	if !changed {
		t.Fatal("SortFile changed = false, want true")
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := "default_target = \"base\"\n" +
		"\n" +
		"[targets.win.\"~/.config/mise/config.toml\"]\nsrc = \"dotfiles/win/.config/mise/config.toml\"\n" +
		"\n" +
		"[targets.win.\"~/.gitconfig\"]\nsrc = \"dotfiles/win/.gitconfig\"\n" +
		"\n" +
		"[targets.wsl.\"~/.config/mise/config.toml\"]\nsrc = \"dotfiles/wsl/.config/mise/config.toml\"\n" +
		"\n" +
		"[targets.wsl.\"~/.gitconfig\"]\nsrc = \"dotfiles/wsl/.gitconfig\"\n"
	if string(got) != want {
		t.Errorf("ソート結果の完全一致失敗:\ngot:\n%s\nwant:\n%s", got, want)
	}
	back, err := Load(p)
	if err != nil {
		t.Fatalf("ソート物のLoad error: %v", err)
	}
	if len(back.TargetsMap["win"]) != 2 || len(back.TargetsMap["wsl"]) != 2 {
		t.Errorf("往復後の件数不正: %+v", back.TargetsMap)
	}
}

// Seam: config パッケージ公開境界 (sort 冪等・override温存・特殊文字)
func TestSortFileIdempotentAndPreserves(t *testing.T) {
	t.Run("2回目は不変でfalse", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		body := "default_target = \"base\"\n\n[targets.base.\"~/.b\"]\nsrc = \"b\"\n\n[targets.base.\"~/.a\"]\nsrc = \"a\"\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := SortFile(p); err != nil {
			t.Fatalf("1回目 error: %v", err)
		}
		first, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		changed, err := SortFile(p)
		if err != nil {
			t.Fatalf("2回目 error: %v", err)
		}
		if changed {
			t.Error("2回目 changed = true, want false")
		}
		second, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Errorf("冪等でない:\nfirst:\n%s\nsecond:\n%s", first, second)
		}
	})

	t.Run("overrideと特殊文字を保つ", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		body := "default_target = \"base\"\n" +
			"[targets.base.\"~/sp ace\"]\nsrc = \"dotfiles/base/sp ace\"\noverride = false\n" +
			"[targets.base.\"~/.a\"]\nsrc = \"a\"\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := SortFile(p); err != nil {
			t.Fatalf("SortFile error: %v", err)
		}
		back, err := Load(p)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		got, err := back.Resolve("base")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Dest != "~/.a" || got[1].Dest != "~/sp ace" {
			t.Errorf("ソート順不正: %+v", got)
		}
		if got[1].Override {
			t.Errorf("override=false温存失敗: %+v", got[1])
		}
	})

	t.Run("雛形のみは不変でfalse", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		if err := os.WriteFile(p, []byte("default_target = \"base\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		changed, err := SortFile(p)
		if err != nil {
			t.Fatalf("SortFile error: %v", err)
		}
		if changed {
			t.Error("changed = true, want false")
		}
	})

	t.Run("不正TOMLは失敗し不変", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"\"\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := SortFile(p); err == nil {
			t.Fatal("エラー expected, got nil")
		}
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != before {
			t.Errorf("不変 expected:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("コメントは落としてソートする", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mdots.toml")
		before := "default_target = \"base\"\n# c\n[targets.base.\"~/.b\"]\nsrc = \"b\"\n[targets.base.\"~/.a\"]\nsrc = \"a\"\n"
		if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := SortFile(p); err != nil {
			t.Fatalf("SortFile error: %v", err)
		}
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(got), "# c") {
			t.Errorf("コメント消失 expected, got:\n%s", got)
		}
		if _, err := Load(p); err != nil {
			t.Fatalf("ソート物のLoad error: %v", err)
		}
	})
}
