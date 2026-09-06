package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: CLIコマンド境界 (mdots diff の代表例)
// 実FS上の Store/dest を用い、書き込みなし・差分出力の外部挙動のみを検証する。
// help/version・フラグ解釈・exit 委譲は新CLIパッケージ境界テストに寄せ、
// 差分詳細は同期境界テストに寄せる。Store 準備・HOME 隔離は main_test.go の
// setupStoreWithHome に集約している。
func TestDiffIntegration(t *testing.T) {
	t.Run("差分を出して書き込まない", func(t *testing.T) {
		store, home := setupStoreWithHome(t,
			map[string]string{"vimrc": "new\n"},
			map[string]string{".vimrc": "old\n"},
			"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n",
		)

		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"diff"}, store, &out, &errOut); code == 0 {
			t.Error("run(diff) with changes: exit = 0, want non-zero")
		}
		if !strings.Contains(out.String(), "---") || !strings.Contains(out.String(), "+++") {
			t.Errorf("diff output should contain ---/+++, got %q", out.String())
		}
		if !strings.Contains(out.String(), "--- vimrc\n") {
			t.Errorf("diff must be Store->dest fixed, got %q", out.String())
		}
		if got, err := os.ReadFile(filepath.Join(home, ".vimrc")); err != nil || string(got) != "old\n" {
			t.Errorf("dest must NOT be written on diff: content = %q err = %v", got, err)
		}
		if got, err := os.ReadFile(filepath.Join(store, "vimrc")); err != nil || string(got) != "new\n" {
			t.Errorf("Store must NOT be written on diff: content = %q err = %v", got, err)
		}
	})

	t.Run("差分なしはexit 0", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "same\n"},
			map[string]string{".vimrc": "same\n"},
			"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n",
		)

		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"diff"}, store, &out, &errOut); code != 0 {
			t.Errorf("run(diff) without changes: exit = %d, want 0", code)
		}
	})
}

func TestDryRunFlagsRemoved(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"vimrc": "same\n"},
		map[string]string{".vimrc": "same\n"},
		"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n",
	)
	for _, args := range [][]string{{"push", "--dry-run"}, {"pull", "--dry-run"}} {
		var out, errOut bytes.Buffer
		if code := runWithWriters(args, store, &out, &errOut); code == 0 {
			t.Errorf("run(%v): exit = 0, want non-zero", args)
		}
	}
}

func TestStoreNotFoundFriendlyError(t *testing.T) {
	empty := t.TempDir()
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"push"}, empty, &out, &errOut); code == 0 {
		t.Fatal("run(push) without Store: exit = 0, want non-zero")
	}
	if !strings.Contains(errOut.String(), "mdots.toml not found in "+empty) {
		t.Errorf("stderr should contain friendly message, got %q", errOut.String())
	}
}
