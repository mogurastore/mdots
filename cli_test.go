package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: CLIコマンド境界 (mdots push/pull --dry-run の代表例)
// 実FS上の Store/dest を用い、書き込みなし・差分出力の外部挙動のみを検証する。
// help/version・フラグ解釈・exit 委譲は新CLIパッケージ境界テストに寄せ、
// 差分詳細は同期境界テストに寄せる。Store 準備・HOME 隔離は main_test.go の
// setupStoreWithHome に集約している.
func TestDryRunIntegration(t *testing.T) {
	t.Run("pushは差分を出して書き込まない", func(t *testing.T) {
		store, home := setupStoreWithHome(t,
			map[string]string{"vimrc": "new\n"},
			map[string]string{".vimrc": "old\n"},
			"[entries]\n\"~/.vimrc\" = { src = \"vimrc\" }\n",
		)

		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"push", "--dry-run"}, store, &out, &errOut); code == 0 {
			t.Error("run(push --dry-run) with changes: exit = 0, want non-zero")
		}
		if !strings.Contains(out.String(), "---") || !strings.Contains(out.String(), "+++") {
			t.Errorf("diff output should contain ---/+++, got %q", out.String())
		}
		if !strings.Contains(out.String(), "+++ vimrc\n") {
			t.Errorf("push dry-run must be dest->Store fixed, got %q", out.String())
		}
		if got, err := os.ReadFile(filepath.Join(home, ".vimrc")); err != nil || string(got) != "old\n" {
			t.Errorf("dest must NOT be written on dry-run: content = %q err = %v", got, err)
		}
		if got, err := os.ReadFile(filepath.Join(store, "vimrc")); err != nil || string(got) != "new\n" {
			t.Errorf("Store must NOT be written on dry-run: content = %q err = %v", got, err)
		}
	})

	t.Run("pullは逆方向に出して書き込まない", func(t *testing.T) {
		store, home := setupStoreWithHome(t,
			map[string]string{"vimrc": "old\n"},
			map[string]string{".vimrc": "new\n"},
			"[entries]\n\"~/.vimrc\" = { src = \"vimrc\" }\n",
		)

		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"pull", "--dry-run"}, store, &out, &errOut); code == 0 {
			t.Error("run(pull --dry-run) with changes: exit = 0, want non-zero")
		}
		if !strings.Contains(out.String(), "--- vimrc\n") {
			t.Errorf("pull dry-run must be Store->dest, got %q", out.String())
		}
		if got, err := os.ReadFile(filepath.Join(home, ".vimrc")); err != nil || string(got) != "new\n" {
			t.Errorf("dest must NOT be written on dry-run: content = %q err = %v", got, err)
		}
		if got, err := os.ReadFile(filepath.Join(store, "vimrc")); err != nil || string(got) != "old\n" {
			t.Errorf("Store must NOT be written on dry-run: content = %q err = %v", got, err)
		}
	})

	t.Run("差分なしはexit 0", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "same\n"},
			map[string]string{".vimrc": "same\n"},
			"[entries]\n\"~/.vimrc\" = { src = \"vimrc\" }\n",
		)

		for _, args := range [][]string{{"push", "--dry-run"}, {"pull", "--dry-run"}} {
			var out, errOut bytes.Buffer
			if code := runWithWriters(args, store, &out, &errOut); code != 0 {
				t.Errorf("run(%v) without changes: exit = %d, want 0", args, code)
			}
			if out.String() != "No changes.\n" {
				t.Errorf("run(%v) without changes: stdout = %q, want %q", args, out.String(), "No changes.\n")
			}
		}
	})

	t.Run("Target指定で解決結果のみが差分対象になる", func(t *testing.T) {
		entriesToml := "[entries]\n" +
			`"~/.common.conf" = { src = "common.conf" }` + "\n" +
			`"~/.win.conf" = { targets = [{ target = "win", src = "win.conf" }] }` + "\n"
		store, _ := setupStoreWithHome(t,
			map[string]string{"common.conf": "same\n", "win.conf": "new\n"},
			map[string]string{".common.conf": "same\n", ".win.conf": "old\n"},
			entriesToml,
		)

		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"push", "--dry-run", "--target", "win"}, store, &out, &errOut); code == 0 {
			t.Error("run(push --dry-run --target win) with changes: exit = 0, want non-zero")
		}
		if !strings.Contains(out.String(), "win.conf") {
			t.Errorf("win Entryの差分を含むべき, got %q", out.String())
		}
		if strings.Contains(out.String(), "common.conf") {
			t.Errorf("差分なしの指定なしEntryを含めるべきでない, got %q", out.String())
		}

		out.Reset()
		errOut.Reset()
		if code := runWithWriters([]string{"push", "--dry-run", "--target", "linux"}, store, &out, &errOut); code != 0 {
			t.Errorf("run(push --dry-run --target linux) without applicable diff: exit = %d, want 0 (out=%q)", code, out.String())
		}
	})

	t.Run("pullのTarget指定でも解決結果のみが差分対象になる", func(t *testing.T) {
		entriesToml := "[entries]\n" +
			`"~/.common.conf" = { src = "common.conf" }` + "\n" +
			`"~/.win.conf" = { targets = [{ target = "win", src = "win.conf" }] }` + "\n"
		store, _ := setupStoreWithHome(t,
			map[string]string{"common.conf": "same\n", "win.conf": "old\n"},
			map[string]string{".common.conf": "same\n", ".win.conf": "new\n"},
			entriesToml,
		)

		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"pull", "--dry-run", "--target", "win"}, store, &out, &errOut); code == 0 {
			t.Error("run(pull --dry-run --target win) with changes: exit = 0, want non-zero")
		}
		if !strings.Contains(out.String(), "win.conf") {
			t.Errorf("win Entryの差分を含むべき, got %q", out.String())
		}

		out.Reset()
		errOut.Reset()
		if code := runWithWriters([]string{"pull", "--dry-run", "--target", "linux"}, store, &out, &errOut); code != 0 {
			t.Errorf("run(pull --dry-run --target linux) without applicable diff: exit = %d, want 0 (out=%q)", code, out.String())
		}
	})
}

func TestColorRequiresDryRun(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"vimrc": "same\n"},
		map[string]string{".vimrc": "same\n"},
		"[entries]\n\"~/.vimrc\" = { src = \"vimrc\" }\n",
	)
	for _, args := range [][]string{{"push", "--color=always"}, {"pull", "--color=always"}} {
		var out, errOut bytes.Buffer
		if code := runWithWriters(args, store, &out, &errOut); code == 0 {
			t.Errorf("run(%v): exit = 0, want non-zero", args)
		}
		if !strings.Contains(errOut.String(), "--color requires --dry-run") {
			t.Errorf("run(%v): stderr should contain --color requires --dry-run, got %q", args, errOut.String())
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
