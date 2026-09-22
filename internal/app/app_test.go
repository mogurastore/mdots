package app

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

// Seam: 実行系境界 (override=false の skip 警告)
func TestPushOverrideSkippedWarnsToStderr(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"vimrc": "new\n"},
		map[string]string{".vimrc": "old\n"},
		"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\noverride = false\n",
	)

	var out, errOut bytes.Buffer
	if err := Push(store, "", &out, &errOut); err != nil {
		t.Fatalf("Push exit error = %v, want nil", err)
	}
	if out.String() != "No changes.\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "No changes.\n")
	}
	if got, err := os.ReadFile(filepath.Join(home, ".vimrc")); err != nil || string(got) != "old\n" {
		t.Fatalf("protected dest content = %q err = %v, want %q", got, err, "old\n")
	}
	want := "skipped: ~/.vimrc (override=false, push would not override)\n"
	if errOut.String() != want {
		t.Errorf("stderr = %q, want %q", errOut.String(), want)
	}
	if strings.Contains(errOut.String(), "overwrite") || strings.Contains(errOut.String(), "force") {
		t.Errorf("stderr must avoid overwrite/force, got %q", errOut.String())
	}
}
