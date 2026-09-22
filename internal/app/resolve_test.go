package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: 実行系境界 (resolve 共有寄せ)
// 隔離 Store で外部挙動（exit・出力文面・FS副作用）のみを検証する。

func readToml(t *testing.T, store string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store, "mdots.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestResolveMergesSameContentToShared(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{
			"dotfiles/win/.vimrc": "same\n",
			"dotfiles/wsl/.vimrc": "same\n",
		},
		nil,
		"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"dotfiles/win/.vimrc\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"dotfiles/wsl/.vimrc\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Resolve(store, &out, &errOut); code != 0 {
		t.Fatalf("Resolve exit = %d, want 0 (stderr=%q)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "resolved") {
		t.Errorf("stdout should contain resolved, got %q", out.String())
	}
	if !strings.Contains(out.String(), "shared/.vimrc") {
		t.Errorf("stdout should contain new src, got %q", out.String())
	}
	got, err := os.ReadFile(filepath.Join(store, "shared", ".vimrc"))
	if err != nil {
		t.Fatalf("new file missing: %v", err)
	}
	if string(got) != "same\n" {
		t.Errorf("new content = %q, want same", got)
	}
	for _, p := range []string{"dotfiles/win/.vimrc", "dotfiles/wsl/.vimrc"} {
		if _, err := os.Stat(filepath.Join(store, p)); !os.IsNotExist(err) {
			t.Errorf("%s should be deleted, err=%v", p, err)
		}
	}
	body := readToml(t, store)
	if !strings.Contains(body, "shared/.vimrc") {
		t.Errorf("toml should contain new src, got:\n%s", body)
	}
	if strings.Contains(body, "dotfiles/win/.vimrc") || strings.Contains(body, "dotfiles/wsl/.vimrc") {
		t.Errorf("toml should not contain old src, got:\n%s", body)
	}
}

func TestResolveNoCandidates(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"a-src": "aaa\n", "b-src": "bbb\n"},
		nil,
		"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Resolve(store, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out.String() != "No sharable entries.\n" {
		t.Errorf("stdout = %q, want No sharable entries.", out.String())
	}
}

func TestResolveRequiresSharedDir(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"a-src": "same\n", "b-src": "same\n"},
		nil,
		"default_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Resolve(store, &out, &errOut); code == 0 {
		t.Fatal("missing shared_dir: exit = 0, want non-zero")
	} else if !strings.Contains(errOut.String(), "shared_dir") {
		t.Errorf("stderr should contain shared_dir, got %q", errOut.String())
	}
}

func TestResolvePreservesOverrideAndDest(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"a-src": "same\n", "b-src": "same\n"},
		nil,
		"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\noverride = false\n"+
			"[targets.wsl.\"~/.other\"]\nsrc = \"b-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Resolve(store, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0 (%q)", code, errOut.String())
	}
	body := readToml(t, store)
	if !strings.Contains(body, "override = false") {
		t.Errorf("override should be preserved, got:\n%s", body)
	}
	if !strings.Contains(body, "~/.vimrc") || !strings.Contains(body, "~/.other") {
		t.Errorf("dest keys should be preserved, got:\n%s", body)
	}
}

func TestResolveAdoptsSameContent(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{
			"a-src":        "same\n",
			"b-src":        "same\n",
			"shared/a-src": "same\n",
		},
		nil,
		"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Resolve(store, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0 (%q)", code, errOut.String())
	}
	body := readToml(t, store)
	if !strings.Contains(body, "shared/a-src") {
		t.Errorf("should adopt existing same content, got:\n%s", body)
	}
}

func TestResolveFailsOnDifferentContentWithoutChanges(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{
			"a-src":        "same\n",
			"b-src":        "same\n",
			"shared/a-src": "different\n",
		},
		nil,
		"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	before := readToml(t, store)
	var out, errOut bytes.Buffer
	if code := Resolve(store, &out, &errOut); code == 0 {
		t.Fatal("different content: exit = 0, want non-zero")
	} else if errOut.String() == "" {
		t.Error("stderr should contain error")
	}
	if got := readToml(t, store); got != before {
		t.Errorf("toml must be unchanged:\nbefore:\n%s\ngot:\n%s", before, got)
	}
	for _, p := range []string{"a-src", "b-src"} {
		got, _ := os.ReadFile(filepath.Join(store, p))
		if string(got) != "same\n" {
			t.Errorf("%s changed, got %q", p, got)
		}
	}
	got, _ := os.ReadFile(filepath.Join(store, "shared", "a-src"))
	if string(got) != "different\n" {
		t.Errorf("existing different must be unchanged, got %q", got)
	}
}

func TestResolveSkipsMissing(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"a-src": "same\n"},
		nil,
		"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"missing-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Resolve(store, &out, &errOut); code != 0 {
		t.Fatalf("missing should be skipped, exit = %d (%q)", code, errOut.String())
	}
	if out.String() != "No sharable entries.\n" {
		t.Errorf("stdout = %q, want No sharable entries.", out.String())
	}
}

func TestResolveWithoutStoreFails(t *testing.T) {
	empty := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Resolve(empty, &out, &errOut); code == 0 {
		t.Fatal("without Store: exit = 0, want non-zero")
	} else if !strings.Contains(errOut.String(), "mdots.toml not found in "+empty) {
		t.Errorf("stderr = %q", errOut.String())
	}
}
