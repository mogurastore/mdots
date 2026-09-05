package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: CLIコマンド境界 (mdots --help / --version)
// 実FSは使わず、外部挙動（exit codeと出力文面）のみを検証する。
func TestGlobalHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"--help"}, t.TempDir(), &out, &errOut); code != 0 {
		t.Fatalf("run(--help) exit = %d, want 0", code)
	}
	got := out.String()
	for _, want := range []string{"usage:", "push", "pull", "diff"} {
		if !strings.Contains(got, want) {
			t.Errorf("help output should contain %q, got %q", want, got)
		}
	}
}

func TestGlobalHelpShortFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"-h"}, t.TempDir(), &out, &errOut); code != 0 {
		t.Fatalf("run(-h) exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "usage:") {
		t.Errorf("help output should contain usage:, got %q", out.String())
	}
}

func TestPushHelp(t *testing.T) {
	for _, args := range [][]string{{"push", "--help"}, {"push", "-h"}} {
		var out, errOut bytes.Buffer
		store := t.TempDir()
		if code := runWithWriters(args, store, &out, &errOut); code != 0 {
			t.Fatalf("run(%v) exit = %d, want 0", args, code)
		}
		got := out.String()
		for _, want := range []string{"push", "--target", "--dry-run"} {
			if !strings.Contains(got, want) {
				t.Errorf("run(%v): output should contain %q, got %q", args, want, got)
			}
		}
	}
}

func TestPullHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"pull", "--help"}, t.TempDir(), &out, &errOut); code != 0 {
		t.Fatalf("run(pull --help) exit = %d, want 0", code)
	}
	for _, want := range []string{"pull", "--target", "--dry-run"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output should contain %q, got %q", want, out.String())
		}
	}
}

func TestDiffHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"diff", "--help"}, t.TempDir(), &out, &errOut); code != 0 {
		t.Fatalf("run(diff --help) exit = %d, want 0", code)
	}
	for _, want := range []string{"diff", "--target"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output should contain %q, got %q", want, out.String())
		}
	}
}

func TestVersion(t *testing.T) {
	old := version
	version = "v0.0.0-test"
	defer func() { version = old }()

	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"--version"}, t.TempDir(), &out, &errOut); code != 0 {
		t.Fatalf("run(--version) exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "v0.0.0-test") {
		t.Errorf("version output should contain %q, got %q", "v0.0.0-test", out.String())
	}
}

// Seam: CLIコマンド境界 (mdots push/pull --dry-run)
// 実FS上の Store/dest を用い、書き込みなし・差分相当出力の外部挙動のみを検証する。
func TestPushDryRunDoesNotWriteButShowsDiff(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(store, "vimrc"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".vimrc"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := "entries:\n  - src: vimrc\n    dest: ~/.vimrc\n"
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := runWithWriters([]string{"push", "--dry-run"}, store, &out, &errOut)
	if code == 0 {
		t.Error("run(push --dry-run) with changes: exit = 0, want non-zero")
	}
	if !strings.Contains(out.String(), "---") || !strings.Contains(out.String(), "+++") {
		t.Errorf("dry-run output should contain ---/+++, got %q", out.String())
	}
	got, err := os.ReadFile(filepath.Join(home, ".vimrc"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old\n" {
		t.Errorf("dest must NOT be written on dry-run: content = %q, want %q", got, "old\n")
	}
}

func TestPushDryRunNoDiffExitsZero(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(store, "vimrc"), []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".vimrc"), []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := "entries:\n  - src: vimrc\n    dest: ~/.vimrc\n"
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"push", "--dry-run"}, store, &out, &errOut); code != 0 {
		t.Errorf("run(push --dry-run) without changes: exit = %d, want 0", code)
	}
}

func TestPullDryRunDoesNotWriteButShowsDiff(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(store, "vimrc"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".vimrc"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := "entries:\n  - src: vimrc\n    dest: ~/.vimrc\n"
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := runWithWriters([]string{"pull", "--dry-run"}, store, &out, &errOut)
	if code == 0 {
		t.Error("run(pull --dry-run) with changes: exit = 0, want non-zero")
	}
	if !strings.Contains(out.String(), "---") || !strings.Contains(out.String(), "+++") {
		t.Errorf("dry-run output should contain ---/+++, got %q", out.String())
	}
	got, err := os.ReadFile(filepath.Join(store, "vimrc"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old\n" {
		t.Errorf("Store must NOT be written on dry-run: content = %q, want %q", got, "old\n")
	}
}

func TestDiffRejectsDryRun(t *testing.T) {
	store := t.TempDir()
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte("entries: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"diff", "--dry-run"}, store, &out, &errOut); code == 0 {
		t.Error("run(diff --dry-run): exit = 0, want non-zero")
	}
}

func TestStoreNotFoundFriendlyError(t *testing.T) {
	empty := t.TempDir()
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"push"}, empty, &out, &errOut); code == 0 {
		t.Fatal("run(push) without Store: exit = 0, want non-zero")
	}
	if !strings.Contains(errOut.String(), "mdots.yaml not found: searched from ") {
		t.Errorf("stderr should contain friendly message, got %q", errOut.String())
	}
}
