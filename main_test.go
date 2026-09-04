package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Seam: CLIコマンド境界 (mdots push)
// 実FS上の Store/dest を用い、end-to-end の外部挙動のみを検証する。
func TestPushEndToEnd(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(store, "vimrc"), []byte("set number\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := "entries:\n  - src: vimrc\n    dest: ~/.vimrc\n"
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := run([]string{"push"}, store); code != 0 {
		t.Fatalf("run(push) exit = %d, want 0", code)
	}
	got, err := os.ReadFile(filepath.Join(home, ".vimrc"))
	if err != nil {
		t.Fatalf("dest read error: %v", err)
	}
	if string(got) != "set number\n" {
		t.Errorf("dest content = %q, want %q", got, "set number\n")
	}
}

func TestPushFromSubdirFindsStore(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(store, "vimrc"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := "entries:\n  - src: vimrc\n    dest: ~/.vimrc\n"
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(store, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	if code := run([]string{"push"}, sub); code != 0 {
		t.Fatalf("run(push) from subdir exit = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(home, ".vimrc")); err != nil {
		t.Errorf("dest not created from subdir: %v", err)
	}
}

func TestPushOnlyCommonEntries(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(store, "common.conf"), []byte("common\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, "win.conf"), []byte("win\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := "entries:\n" +
		"  - src: common.conf\n    dest: ~/.common.conf\n" +
		"  - src: win.conf\n    dest: ~/.win.conf\n    target: win\n"
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := run([]string{"push"}, store); code != 0 {
		t.Fatalf("run(push) exit = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(home, ".common.conf")); err != nil {
		t.Errorf("common Entry should be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".win.conf")); err == nil {
		t.Error("win Entry should NOT be copied without --target")
	}
}

func TestPushWithoutStoreFails(t *testing.T) {
	empty := t.TempDir()
	if code := run([]string{"push"}, empty); code == 0 {
		t.Error("run(push) without Store: exit = 0, want non-zero")
	}
}
