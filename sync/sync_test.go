package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mogurastore/mdots/config"
)

// Seam: sync パッケージ公開境界 (push)
// Store/src → dest のファイルコピーを t.TempDir() の実FSで検証する。
func TestPushCopiesStoreToDest(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	srcPath := filepath.Join(store, "vimrc")
	if err := os.WriteFile(srcPath, []byte("set number\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(home, ".vimrc")
	entries := []config.Entry{{Src: "vimrc", Dest: dest}}

	if err := Push(store, entries); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("dest read error: %v", err)
	}
	if string(got) != "set number\n" {
		t.Errorf("dest content = %q, want %q", got, "set number\n")
	}
}

func TestPushCreatesParentDirs(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()

	srcPath := filepath.Join(store, "wezterm.lua")
	if err := os.WriteFile(srcPath, []byte("return {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(destRoot, ".config", "wezterm", "wezterm.lua")
	entries := []config.Entry{{Src: "wezterm.lua", Dest: dest}}

	if err := Push(store, entries); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("dest not created: %v", err)
	}
}

func TestPushExpandsHomeDest(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	srcPath := filepath.Join(store, "bashrc")
	if err := os.WriteFile(srcPath, []byte("export X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{{Src: "bashrc", Dest: "~/.bashrc"}}

	if err := Push(store, entries); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	if err != nil {
		t.Fatalf("expanded dest read error: %v", err)
	}
	if string(got) != "export X=1\n" {
		t.Errorf("dest content = %q, want %q", got, "export X=1\n")
	}
}

func TestPushRejectsDirectories(t *testing.T) {
	t.Run("srcがディレクトリはエラー", func(t *testing.T) {
		store := t.TempDir()
		destRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(store, "mydir"), 0o755); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "mydir", Dest: filepath.Join(destRoot, "mydir")}}
		if err := Push(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})

	t.Run("destがディレクトリはエラー", func(t *testing.T) {
		store := t.TempDir()
		destRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(store, "a"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		destDir := filepath.Join(destRoot, "existing")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "a", Dest: destDir}}
		if err := Push(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})
}

func TestPushOverwritesExisting(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()

	if err := os.WriteFile(filepath.Join(store, "a"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(destRoot, "a")
	if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{{Src: "a", Dest: dest}}

	if err := Push(store, entries); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("dest content = %q, want %q", got, "new")
	}
}

func TestPushPreservesPermission(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()

	srcPath := filepath.Join(store, "run.sh")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(destRoot, "run.sh")
	entries := []config.Entry{{Src: "run.sh", Dest: dest}}

	if err := Push(store, entries); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("dest perm = %o, want 755", info.Mode().Perm())
	}
}

// Seam: sync パッケージ公開境界 (pull)
// dest → Store/src のファイルコピーを t.TempDir() の実FSで検証する。
func TestPullCopiesDestToStore(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	dest := filepath.Join(home, ".vimrc")
	if err := os.WriteFile(dest, []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{{Src: "vimrc", Dest: dest}}

	if err := Pull(store, entries); err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(store, "vimrc"))
	if err != nil {
		t.Fatalf("store read error: %v", err)
	}
	if string(got) != "edited\n" {
		t.Errorf("store content = %q, want %q", got, "edited\n")
	}
}

func TestPullCreatesParentDirsOnStoreSide(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()

	dest := filepath.Join(destRoot, ".config", "wezterm", "wezterm.lua")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("return {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{{Src: filepath.Join(".config", "wezterm", "wezterm.lua"), Dest: dest}}

	if err := Pull(store, entries); err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store, ".config", "wezterm", "wezterm.lua")); err != nil {
		t.Errorf("store src not created: %v", err)
	}
}

func TestPullExpandsHomeDest(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("export X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{{Src: "bashrc", Dest: "~/.bashrc"}}

	if err := Pull(store, entries); err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(store, "bashrc"))
	if err != nil {
		t.Fatalf("store read error: %v", err)
	}
	if string(got) != "export X=1\n" {
		t.Errorf("store content = %q, want %q", got, "export X=1\n")
	}
}

func TestPullRejectsDirectories(t *testing.T) {
	t.Run("destがディレクトリはエラー", func(t *testing.T) {
		store := t.TempDir()
		destRoot := t.TempDir()
		destDir := filepath.Join(destRoot, "existing")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "a", Dest: destDir}}
		if err := Pull(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})

	t.Run("Store側がディレクトリはエラー", func(t *testing.T) {
		store := t.TempDir()
		destRoot := t.TempDir()
		dest := filepath.Join(destRoot, "a")
		if err := os.WriteFile(dest, []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(store, "a"), 0o755); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "a", Dest: dest}}
		if err := Pull(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})
}

func TestPullMissingDestIsError(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()
	entries := []config.Entry{{Src: "missing", Dest: filepath.Join(destRoot, "missing")}}
	if err := Pull(store, entries); err == nil {
		t.Error("エラー expected, got nil")
	}
}

func TestPullOverwritesExisting(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()

	dest := filepath.Join(destRoot, "a")
	if err := os.WriteFile(dest, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(store, "a")
	if err := os.WriteFile(srcPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{{Src: "a", Dest: dest}}

	if err := Pull(store, entries); err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	got, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("store content = %q, want %q", got, "new")
	}
}

func TestPullPreservesPermission(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()

	dest := filepath.Join(destRoot, "run.sh")
	if err := os.WriteFile(dest, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{{Src: "run.sh", Dest: dest}}

	if err := Pull(store, entries); err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	info, err := os.Stat(filepath.Join(store, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("store perm = %o, want 755", info.Mode().Perm())
	}
}

func TestPushMissingSrcIsError(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()
	entries := []config.Entry{{Src: "missing", Dest: filepath.Join(destRoot, "missing")}}
	if err := Push(store, entries); err == nil {
		t.Error("エラー expected, got nil")
	}
}

// Seam: sync パッケージ公開境界 (diff)
// Store/src と dest の差分を diff -u 風に出力する。実FSで検証する。
func TestDiffNoDiffWhenIdentical(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()

	srcPath := filepath.Join(store, "vimrc")
	if err := os.WriteFile(srcPath, []byte("set number\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(destRoot, ".vimrc")
	if err := os.WriteFile(dest, []byte("set number\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{{Src: "vimrc", Dest: dest}}

	out, hasDiff, err := Diff(store, entries)
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	if hasDiff {
		t.Error("hasDiff = true, want false")
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestDiffShowsUnifiedDiffWhenDifferent(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()

	srcPath := filepath.Join(store, "vimrc")
	if err := os.WriteFile(srcPath, []byte("set number\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(destRoot, ".vimrc")
	if err := os.WriteFile(dest, []byte("set nonumber\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{{Src: "vimrc", Dest: dest}}

	out, hasDiff, err := Diff(store, entries)
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if !strings.Contains(out, "---") || !strings.Contains(out, "+++") {
		t.Errorf("output should contain ---/+++, got %q", out)
	}
}

func TestDiffMissingFileIsError(t *testing.T) {
	t.Run("src不在はエラー", func(t *testing.T) {
		store := t.TempDir()
		destRoot := t.TempDir()
		dest := filepath.Join(destRoot, "a")
		if err := os.WriteFile(dest, []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "missing", Dest: dest}}
		if _, _, err := Diff(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})

	t.Run("dest不在はエラー", func(t *testing.T) {
		store := t.TempDir()
		destRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(store, "a"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "a", Dest: filepath.Join(destRoot, "missing")}}
		if _, _, err := Diff(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})
}

func TestDiffMultipleEntriesOnlyDifferingOutput(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()

	if err := os.WriteFile(filepath.Join(store, "same"), []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destRoot, "same"), []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, "changed"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destRoot, "changed"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{
		{Src: "same", Dest: filepath.Join(destRoot, "same")},
		{Src: "changed", Dest: filepath.Join(destRoot, "changed")},
	}

	out, hasDiff, err := Diff(store, entries)
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if !strings.Contains(out, "changed") {
		t.Errorf("output should mention changed Entry, got %q", out)
	}
	if strings.Contains(out, "--- same") {
		t.Errorf("output should NOT contain same Entry, got %q", out)
	}
	if !strings.Contains(out, "-old") || !strings.Contains(out, "+new") {
		t.Errorf("output should contain -/+ lines, got %q", out)
	}
}
