package sync

import (
	"os"
	"path/filepath"
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

func TestPushMissingSrcIsError(t *testing.T) {
	store := t.TempDir()
	destRoot := t.TempDir()
	entries := []config.Entry{{Src: "missing", Dest: filepath.Join(destRoot, "missing")}}
	if err := Push(store, entries); err == nil {
		t.Error("エラー expected, got nil")
	}
}
