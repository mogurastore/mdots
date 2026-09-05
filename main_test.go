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

func TestPushWithTarget(t *testing.T) {
	setup := func(t *testing.T) (store, home string) {
		t.Helper()
		store = t.TempDir()
		home = t.TempDir()
		t.Setenv("HOME", home)

		files := map[string]string{
			"common.conf":  "common\n",
			"explicit.conf": "explicit\n",
			"win.conf":     "win\n",
			"multi.conf":   "multi\n",
			"wsl.conf":     "wsl\n",
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(store, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		yaml := "entries:\n" +
			"  - src: common.conf\n    dest: ~/.common.conf\n" +
			"  - src: explicit.conf\n    dest: ~/.explicit.conf\n    target: common\n" +
			"  - src: win.conf\n    dest: ~/.win.conf\n    target: win\n" +
			"  - src: multi.conf\n    dest: ~/.multi.conf\n    target: [win, wsl]\n" +
			"  - src: wsl.conf\n    dest: ~/.wsl.conf\n    target: wsl\n"
		if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		return store, home
	}

	tests := []struct {
		name string
		args []string
		want []string
		not  []string
	}{
		{"winはcommon+win", []string{"push", "--target", "win"}, []string{".common.conf", ".explicit.conf", ".win.conf", ".multi.conf"}, []string{".wsl.conf"}},
		{"wslはcommon+wsl(配列一致含む)", []string{"push", "--target", "wsl"}, []string{".common.conf", ".explicit.conf", ".multi.conf", ".wsl.conf"}, []string{".win.conf"}},
		{"未知Targetはcommonのみ", []string{"push", "--target", "linux"}, []string{".common.conf", ".explicit.conf"}, []string{".win.conf", ".multi.conf", ".wsl.conf"}},
		{"target=形式も受付", []string{"push", "--target=win"}, []string{".common.conf", ".explicit.conf", ".win.conf", ".multi.conf"}, []string{".wsl.conf"}},
		{"target commonはcommonのみと同等", []string{"push", "--target", "common"}, []string{".common.conf", ".explicit.conf"}, []string{".win.conf", ".multi.conf", ".wsl.conf"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, home := setup(t)
			if code := run(tt.args, store); code != 0 {
				t.Fatalf("run(%v) exit = %d, want 0", tt.args, code)
			}
			for _, f := range tt.want {
				if _, err := os.Stat(filepath.Join(home, f)); err != nil {
					t.Errorf("%s should be copied: %v", f, err)
				}
			}
			for _, f := range tt.not {
				if _, err := os.Stat(filepath.Join(home, f)); err == nil {
					t.Errorf("%s should NOT be copied with %v", f, tt.args)
				}
			}
		})
	}
}

func TestPushTargetFlagErrors(t *testing.T) {
	store := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte("entries: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"push", "--target"},
		{"push", "--target="},
		{"push", "--unknown"},
	} {
		if code := run(args, store); code == 0 {
			t.Errorf("run(%v): exit = 0, want non-zero", args)
		}
	}
}

// Seam: CLIコマンド境界 (mdots pull)
// 実FS上の Store/dest を用い、end-to-end の外部挙動のみを検証する。
func TestPullEndToEnd(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(home, ".vimrc"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := "entries:\n  - src: vimrc\n    dest: ~/.vimrc\n"
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

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

func TestPullWithTargetFiltersCommonPlusTarget(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(home, ".common.conf"), []byte("common edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".win.conf"), []byte("win edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".wsl.conf"), []byte("wsl edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := "entries:\n" +
		"  - src: common.conf\n    dest: ~/.common.conf\n" +
		"  - src: win.conf\n    dest: ~/.win.conf\n    target: win\n" +
		"  - src: wsl.conf\n    dest: ~/.wsl.conf\n    target: wsl\n"
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := run([]string{"pull", "--target", "win"}, store); code != 0 {
		t.Fatalf("run(pull --target win) exit = %d, want 0", code)
	}
	if got, err := os.ReadFile(filepath.Join(store, "common.conf")); err != nil || string(got) != "common edited\n" {
		t.Errorf("common should be pulled: content=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(store, "win.conf")); err != nil || string(got) != "win edited\n" {
		t.Errorf("win should be pulled: content=%q err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(store, "wsl.conf")); err == nil {
		t.Error("wsl should NOT be pulled with --target win")
	}
}

func TestPullMissingDestIsError(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	yaml := "entries:\n  - src: vimrc\n    dest: ~/.vimrc\n"
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := run([]string{"pull"}, store); code == 0 {
		t.Error("run(pull) with missing dest: exit = 0, want non-zero")
	}
}

func TestPullDestDirIsError(t *testing.T) {
	store := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "entries:\n  - src: config\n    dest: ~/.config\n"
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := run([]string{"pull"}, store); code == 0 {
		t.Error("run(pull) with dir dest: exit = 0, want non-zero")
	}
}

func TestPullWithoutStoreFails(t *testing.T) {
	empty := t.TempDir()
	if code := run([]string{"pull"}, empty); code == 0 {
		t.Error("run(pull) without Store: exit = 0, want non-zero")
	}
}

func TestPullTargetFlagErrors(t *testing.T) {
	store := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(store, "mdots.yaml"), []byte("entries: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"pull", "--target"},
		{"pull", "--target="},
		{"pull", "--unknown"},
	} {
		if code := run(args, store); code == 0 {
			t.Errorf("run(%v): exit = 0, want non-zero", args)
		}
	}
}

func TestPushWithoutStoreFails(t *testing.T) {
	empty := t.TempDir()
	if code := run([]string{"push"}, empty); code == 0 {
		t.Error("run(push) without Store: exit = 0, want non-zero")
	}
}
