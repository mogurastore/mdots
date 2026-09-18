package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

// Seam: CLIコマンド境界 (mdots push/pull の代表例)
// 実FS上の Store/dest を用い、run 経由の外部挙動のみを検証する。
// Target 展開・フラグ解釈・差分詳細は CLI・設定・同期の各境界テストに寄せ、
// ここでは配線の代表例だけを残す。
func TestPushEndToEnd(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"vimrc": "set number\n"},
		nil,
		"[entries]\n\"~/.vimrc\" = { src = \"vimrc\" }\n",
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
// 新形式の解決結果で push が動くこと、無指定・指定・未知Targetの
// 振る舞いを実FS上の外部挙動で確認する。
func TestPushWithTargetRepresentative(t *testing.T) {
	entriesToml := "[entries]\n" +
		`"~/.common.conf" = { src = "common.conf" }` + "\n" +
		`"~/.win.conf" = { targets = { win = { src = "win.conf" } } }` + "\n" +
		`"~/.wsl.conf" = { targets = { wsl = { src = "wsl.conf" } } }` + "\n"
	storeFiles := map[string]string{
		"common.conf": "common\n",
		"win.conf":    "win\n",
		"wsl.conf":    "wsl\n",
	}

	t.Run("無指定は指定なしのみ", func(t *testing.T) {
		store, home := setupStoreWithHome(t, storeFiles, nil, entriesToml)

		if code := run([]string{"push"}, store); code != 0 {
			t.Fatalf("run(push) exit = %d, want 0", code)
		}
		if _, err := os.Stat(filepath.Join(home, ".common.conf")); err != nil {
			t.Errorf(".common.conf should be copied: %v", err)
		}
		for _, f := range []string{".win.conf", ".wsl.conf"} {
			if _, err := os.Stat(filepath.Join(home, f)); err == nil {
				t.Errorf("%s should NOT be copied without --target", f)
			}
		}
	})

	t.Run("指定は指定なし＋一致のみ", func(t *testing.T) {
		store, home := setupStoreWithHome(t, storeFiles, nil, entriesToml)

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
	})

	t.Run("未知Targetは指定なしのみ", func(t *testing.T) {
		store, home := setupStoreWithHome(t, storeFiles, nil, entriesToml)

		if code := run([]string{"push", "--target", "linux"}, store); code != 0 {
			t.Fatalf("run(push --target linux) exit = %d, want 0", code)
		}
		if _, err := os.Stat(filepath.Join(home, ".common.conf")); err != nil {
			t.Errorf(".common.conf should be copied: %v", err)
		}
		for _, f := range []string{".win.conf", ".wsl.conf"} {
			if _, err := os.Stat(filepath.Join(home, f)); err == nil {
				t.Errorf("%s should NOT be copied with unknown target", f)
			}
		}
	})
}

// pull も新形式の解決結果で動くことの代表例。指定なし＋一致のみが回収され、
// 不一致・無指定時のスキップを確認する。
func TestPullWithTargetRepresentative(t *testing.T) {
	entriesToml := "[entries]\n" +
		`"~/.common.conf" = { src = "common.conf" }` + "\n" +
		`"~/.win.conf" = { targets = { win = { src = "win.conf" } } }` + "\n" +
		`"~/.wsl.conf" = { targets = { wsl = { src = "wsl.conf" } } }` + "\n"
	homeFiles := map[string]string{
		".common.conf": "common edited\n",
		".win.conf":    "win edited\n",
		".wsl.conf":    "wsl edited\n",
	}

	t.Run("指定は指定なし＋一致のみ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, homeFiles, entriesToml)

		if code := run([]string{"pull", "--target", "win"}, store); code != 0 {
			t.Fatalf("run(pull --target win) exit = %d, want 0", code)
		}
		for _, tc := range []struct{ name, want string }{
			{"common.conf", "common edited\n"},
			{"win.conf", "win edited\n"},
		} {
			got, err := os.ReadFile(filepath.Join(store, tc.name))
			if err != nil {
				t.Fatalf("store read error %s: %v", tc.name, err)
			}
			if string(got) != tc.want {
				t.Errorf("store %s content = %q, want %q", tc.name, got, tc.want)
			}
		}
		if _, err := os.Stat(filepath.Join(store, "wsl.conf")); err == nil {
			t.Error("wsl.conf should NOT be pulled with --target win")
		}
	})

	t.Run("無指定は指定なしのみ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, homeFiles, entriesToml)

		if code := run([]string{"pull"}, store); code != 0 {
			t.Fatalf("run(pull) exit = %d, want 0", code)
		}
		if _, err := os.Stat(filepath.Join(store, "common.conf")); err != nil {
			t.Errorf("common.conf should be pulled: %v", err)
		}
		for _, f := range []string{"win.conf", "wsl.conf"} {
			if _, err := os.Stat(filepath.Join(store, f)); err == nil {
				t.Errorf("%s should NOT be pulled without --target", f)
			}
		}
	})

	t.Run("未知Targetは指定なしのみ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, homeFiles, entriesToml)

		if code := run([]string{"pull", "--target", "linux"}, store); code != 0 {
			t.Fatalf("run(pull --target linux) exit = %d, want 0", code)
		}
		if _, err := os.Stat(filepath.Join(store, "common.conf")); err != nil {
			t.Errorf("common.conf should be pulled: %v", err)
		}
		for _, f := range []string{"win.conf", "wsl.conf"} {
			if _, err := os.Stat(filepath.Join(store, f)); err == nil {
				t.Errorf("%s should NOT be pulled with unknown target", f)
			}
		}
	})
}

func TestPushFromSubdirFails(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"vimrc": "x\n"},
		nil,
		"[entries]\n\"~/.vimrc\" = { src = \"vimrc\" }\n",
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
		"[entries]\n\"~/.vimrc\" = { src = \"vimrc\" }\n",
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
		"[entries]\n\"~/.vimrc\" = { src = \"vimrc\" }\n",
	)

	if code := run([]string{"pull"}, store); code == 0 {
		t.Error("run(pull) with missing dest: exit = 0, want non-zero")
	}
}

// Seam: CLIコマンド境界 (push/pull の override 保護配線代表例)
// 実FS上の Store/dest を用い、run 経由の外部挙動のみを検証する。
// 新規作成・権限・差分詳細は同期・差分の各境界テストに寄せ、ここでは配線確認に留める。
func TestPushOverrideProtectsExistingRepresentative(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"vimrc": "new\n"},
		map[string]string{".vimrc": "old\n"},
		"[entries]\n\"~/.vimrc\" = { src = \"vimrc\", override = false }\n",
	)

	if code := run([]string{"push"}, store); code != 0 {
		t.Fatalf("run(push) with override=false exit = %d, want 0", code)
	}
	got, err := os.ReadFile(filepath.Join(home, ".vimrc"))
	if err != nil {
		t.Fatalf("dest read error: %v", err)
	}
	if string(got) != "old\n" {
		t.Errorf("protected dest content = %q, want %q", got, "old\n")
	}
}

func TestPullOverrideProtectsExistingRepresentative(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"vimrc": "old\n"},
		map[string]string{".vimrc": "new\n"},
		"[entries]\n\"~/.vimrc\" = { src = \"vimrc\", override = false }\n",
	)

	if code := run([]string{"pull"}, store); code != 0 {
		t.Fatalf("run(pull) with override=false exit = %d, want 0", code)
	}
	got, err := os.ReadFile(filepath.Join(store, "vimrc"))
	if err != nil {
		t.Fatalf("store read error: %v", err)
	}
	if string(got) != "old\n" {
		t.Errorf("protected store content = %q, want %q", got, "old\n")
	}
}

// Store 未発見時の失敗は共通。文言自体は設定境界テストが保証する。
func TestCommandsWithoutStoreFail(t *testing.T) {
	empty := t.TempDir()
	for _, args := range [][]string{{"push"}, {"pull"}, {"push", "--dry-run"}, {"pull", "--dry-run"}, {"targets"}} {
		if code := run(args, empty); code == 0 {
			t.Errorf("run(%v) without Store: exit = 0, want non-zero", args)
		}
	}
}

// Seam: CLIコマンド境界 (mdots targets の配線代表例)
// 実FS上の Store を用い、runWithWriters 経由の外部挙動のみを検証する。
// 集約・ソートの網羅は設定境界テストが保証する。
func TestTargetsEndToEnd(t *testing.T) {
	t.Run("分散したTargetを重複排除・ソートして1行1名で出す", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil,
			"[entries]\n"+
				`"~/.c" = { targets = { wsl = { src = "c-wsl" }, win = { src = "c-win" } } }`+"\n"+
				`"~/.b" = { targets = { win = { src = "b-win" } } }`+"\n"+
				`"~/.a" = { src = "a" }`+"\n",
		)
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"targets"}, store, &out, &errOut); code != 0 {
			t.Fatalf("run(targets) exit = %d, want 0 (stderr=%q)", code, errOut.String())
		}
		if out.String() != "win\nwsl\n" {
			t.Errorf("stdout = %q, want %q", out.String(), "win\nwsl\n")
		}
	})

	t.Run("Target未定義で空出力・exit 0", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil,
			"[entries]\n\"~/.a\" = { src = \"a\" }\n",
		)
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"targets"}, store, &out, &errOut); code != 0 {
			t.Fatalf("run(targets) exit = %d, want 0 (stderr=%q)", code, errOut.String())
		}
		if out.String() != "" {
			t.Errorf("stdout = %q, want empty", out.String())
		}
	})

	t.Run("toml不正でstderr＋非ゼロ終了", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil,
			"[entries]\n\"~/.a\" = {}\n",
		)
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"targets"}, store, &out, &errOut); code == 0 {
			t.Error("run(targets) with invalid toml: exit = 0, want non-zero")
		}
		if errOut.String() == "" {
			t.Error("stderr should not be empty on invalid toml")
		}
	})

	t.Run("余剰引数でunknown argument＋非ゼロ終了", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil,
			"[entries]\n\"~/.a\" = { src = \"a\" }\n",
		)
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"targets", "extra"}, store, &out, &errOut); code == 0 {
			t.Error("run(targets extra): exit = 0, want non-zero")
		}
		if !strings.Contains(errOut.String(), "unknown argument: extra") {
			t.Errorf("stderr should contain unknown argument, got %q", errOut.String())
		}
	})
}
