package cli

import (
	"strings"
	"testing"
)

func TestCliResolveMerges(t *testing.T) {
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
	code, out, errOut := runCli(t, store, []string{"resolve"})
	if code != 0 {
		t.Fatalf("Run(resolve) exit = %d, want 0 (stderr=%q)", code, errOut)
	}
	if !strings.Contains(out, "resolved") || !strings.Contains(out, "shared/.vimrc") {
		t.Errorf("stdout should contain resolved and new src, got %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr should be empty on success, got %q", errOut)
	}
}

func TestCliResolveNoCandidates(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"a-src": "aaa\n", "b-src": "bbb\n"},
		nil,
		"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	code, out, _ := runCli(t, store, []string{"resolve"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != "No sharable entries.\n" {
		t.Errorf("stdout = %q, want No sharable entries.", out)
	}
}

func TestCliResolveWithoutStore(t *testing.T) {
	empty := t.TempDir()
	code, _, errOut := runCli(t, empty, []string{"resolve"})
	if code == 0 {
		t.Fatal("without Store: exit = 0, want non-zero")
	}
	if !strings.Contains(errOut, "mdots.toml not found in "+empty) {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestCliResolveRejectsExtraArgs(t *testing.T) {
	store, _ := setupStoreWithHome(t, nil, nil, "shared_dir = \"shared\"\ndefault_target = \"base\"\n")
	for _, args := range [][]string{
		{"resolve", "extra"},
		{"resolve", "--dry-run"},
		{"resolve", "--unknown"},
	} {
		if code, _, _ := runCli(t, store, args); code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
	}
}

func TestCliResolveHelp(t *testing.T) {
	for _, args := range [][]string{{"resolve", "--help"}, {"resolve", "-h"}} {
		store, _ := setupStoreWithHome(t, nil, nil, "shared_dir = \"shared\"\ndefault_target = \"base\"\n")
		code, out, _ := runCli(t, store, args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", args, code)
		}
		for _, want := range []string{"USAGE:", "resolve"} {
			if !strings.Contains(out, want) {
				t.Errorf("Run(%v): should contain %q, got %q", args, want, out)
			}
		}
	}
}
