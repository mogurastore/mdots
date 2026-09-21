package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupStoreWithHome は Store 準備・HOME 隔離の定型を集約する。
func setupStoreWithHome(t *testing.T, storeFiles, homeFiles map[string]string, tomlBody string) (store, home string) {
	t.Helper()
	store = t.TempDir()
	home = t.TempDir()
	t.Setenv("HOME", home)
	for name, body := range storeFiles {
		p := filepath.Join(store, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range homeFiles {
		p := filepath.Join(home, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(store, "mdots.toml"), []byte(tomlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return store, home
}

// Seam: 実通し smoke (push の配線代表例)
func TestPushEndToEnd(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"dotfiles/base/vimrc": "set number\n"},
		nil,
		"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"dotfiles/base/vimrc\"\n",
	)

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

func TestPullEndToEnd(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "edited\n"},
		"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\n",
	)

	if code := run([]string{"pull"}, store); code != 0 {
		t.Fatalf("run(pull) exit = %d, want 0", code)
	}
	got, err := os.ReadFile(filepath.Join(store, "vimrc"))
	if err != nil {
		t.Fatalf("store read error: %v", err)
	}
	if string(got) != "edited\n" {
		t.Errorf("store content = %q, want %q", got, "edited\n")
	}
}

// Seam: 実通し smoke (add 登録の配線代表例)
func TestAddEndToEndRegistersWithoutCopying(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"default_target = \"base\"\n",
	)

	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"add", "~/.vimrc"}, store, &out, &errOut); code != 0 {
		t.Fatalf("run(add ~/.vimrc) exit = %d, want 0 (stderr=%q)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "~/.vimrc") || !strings.Contains(out.String(), "dotfiles/base/.vimrc") {
		t.Errorf("stdout should contain key and src, got %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(store, "dotfiles", "base", ".vimrc")); err == nil {
		t.Error("Store src must NOT be created by add (pull collects it)")
	}
}
