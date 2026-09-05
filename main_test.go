package main

import (
	"os"
	"path/filepath"
	"testing"
)

// setupStoreWithHome は Store 準備・HOME 隔離の定型を集約する。
// storeFiles は Store 直下に作るファイル群、homeFiles は HOME 直下に作る
// ファイル群、tomlBody は mdots.toml の本文。HOME は t.Setenv で隔離する。
func setupStoreWithHome(t *testing.T, storeFiles, homeFiles map[string]string, tomlBody string) (store, home string) {
	t.Helper()
	store = t.TempDir()
	home = t.TempDir()
	t.Setenv("HOME", home)
	for name, body := range storeFiles {
		if err := os.WriteFile(filepath.Join(store, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range homeFiles {
		if err := os.WriteFile(filepath.Join(home, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(store, "mdots.toml"), []byte(tomlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return store, home
}

// Seam: CLIコマンド境界 (mdots push/pull/diff の代表例)
// 実FS上の Store/dest を用い、run 経由の外部挙動のみを検証する。
// Target 展開・フラグ解釈・差分詳細は CLI・設定・同期の各境界テストに寄せ、
// ここでは配線の代表例だけを残す。
func TestPushEndToEnd(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"vimrc": "set number\n"},
		nil,
		"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n",
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

// Target 配線の代表例。展開パターン自体は設定境界テストが保証する。
func TestPushWithTargetRepresentative(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{
			"common.conf": "common\n",
			"win.conf":    "win\n",
			"wsl.conf":    "wsl\n",
		},
		nil,
		"[[entries]]\nsrc = \"common.conf\"\ndest = \"~/.common.conf\"\n"+
			"[[entries]]\nsrc = \"win.conf\"\ndest = \"~/.win.conf\"\ntarget = \"win\"\n"+
			"[[entries]]\nsrc = \"wsl.conf\"\ndest = \"~/.wsl.conf\"\ntarget = \"wsl\"\n",
	)

	if code := run([]string{"push", "--target", "win"}, store); code != 0 {
		t.Fatalf("run(push --target win) exit = %d, want 0", code)
	}
	for _, f := range []string{".common.conf", ".win.conf"} {
		if _, err := os.Stat(filepath.Join(home, f)); err != nil {
			t.Errorf("%s should be copied: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".wsl.conf")); err == nil {
		t.Error(".wsl.conf should NOT be copied with --target win")
	}
}

func TestPushFromSubdirFails(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"vimrc": "x\n"},
		nil,
		"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n",
	)
	sub := filepath.Join(store, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	if code := run([]string{"push"}, sub); code == 0 {
		t.Fatal("run(push) from subdir exit = 0, want non-zero")
	}
	if _, err := os.Stat(filepath.Join(home, ".vimrc")); err == nil {
		t.Error("dest must NOT be created from subdir")
	}
}

func TestPullEndToEnd(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "edited\n"},
		"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n",
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

// 同期系エラーの exit 伝播の代表例。欠落・種別の網羅は同期境界テストが保証する。
func TestPullMissingDestIsError(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		nil,
		"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n",
	)

	if code := run([]string{"pull"}, store); code == 0 {
		t.Error("run(pull) with missing dest: exit = 0, want non-zero")
	}
}

func TestDiffExitCodes(t *testing.T) {
	t.Run("差分なしはexit 0", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "set number\n"},
			map[string]string{".vimrc": "set number\n"},
			"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n",
		)
		if code := run([]string{"diff"}, store); code != 0 {
			t.Errorf("run(diff) without changes: exit = %d, want 0", code)
		}
	})

	t.Run("差分ありはexit非ゼロ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "set number\n"},
			map[string]string{".vimrc": "set nonumber\n"},
			"[[entries]]\nsrc = \"vimrc\"\ndest = \"~/.vimrc\"\n",
		)
		if code := run([]string{"diff"}, store); code == 0 {
			t.Error("run(diff) with changes: exit = 0, want non-zero")
		}
	})
}

// Store 未発見時の失敗は3コマンド共通。文言自体は設定境界テストが保証する。
func TestCommandsWithoutStoreFail(t *testing.T) {
	empty := t.TempDir()
	for _, args := range [][]string{{"push"}, {"pull"}, {"diff"}} {
		if code := run(args, empty); code == 0 {
			t.Errorf("run(%v) without Store: exit = 0, want non-zero", args)
		}
	}
}
