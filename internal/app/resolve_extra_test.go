package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveNamingVariants(t *testing.T) {
	t.Run("親を含む共通部", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{
				"dotfiles/win/AppData/Roaming/jj/config.toml": "same\n",
				"dotfiles/wsl/.config/jj/config.toml":         "same\n",
			},
			nil,
			"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
				"[targets.win.\"~/AppData/Roaming/jj/config.toml\"]\nsrc = \"dotfiles/win/AppData/Roaming/jj/config.toml\"\n"+
				"[targets.wsl.\"~/.config/jj/config.toml\"]\nsrc = \"dotfiles/wsl/.config/jj/config.toml\"\n",
		)
		var out, errOut bytes.Buffer
		if code := Resolve(store, &out, &errOut); code != 0 {
			t.Fatalf("exit = %d, want 0 (%q)", code, errOut.String())
		}
		if !strings.Contains(out.String(), "shared/jj/config.toml") {
			t.Errorf("should use parent common suffix, got %q", out.String())
		}
		if _, err := os.Stat(filepath.Join(store, "shared", "jj", "config.toml")); err != nil {
			t.Errorf("new file missing: %v", err)
		}
	})

	t.Run("共通なしはbasename", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"a-foo": "same\n", "b-bar": "same\n"},
			nil,
			"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
				"[targets.win.\"~/.a\"]\nsrc = \"a-foo\"\n"+
				"[targets.wsl.\"~/.b\"]\nsrc = \"b-bar\"\n",
		)
		var out, errOut bytes.Buffer
		if code := Resolve(store, &out, &errOut); code != 0 {
			t.Fatalf("exit = %d, want 0 (%q)", code, errOut.String())
		}
		if !strings.Contains(out.String(), "shared/a-foo") {
			t.Errorf("fallback should be first basename, got %q", out.String())
		}
	})

	t.Run("複数グループ一括", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{
				"a1": "one\n", "b1": "one\n",
				"a2": "two\n", "b2": "two\n",
			},
			nil,
			"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
				"[targets.a.\"~/.a1\"]\nsrc = \"a1\"\n"+
				"[targets.b.\"~/.b1\"]\nsrc = \"b1\"\n"+
				"[targets.a.\"~/.a2\"]\nsrc = \"a2\"\n"+
				"[targets.b.\"~/.b2\"]\nsrc = \"b2\"\n",
		)
		var out, errOut bytes.Buffer
		if code := Resolve(store, &out, &errOut); code != 0 {
			t.Fatalf("exit = %d, want 0 (%q)", code, errOut.String())
		}
		body := readToml(t, store)
		if strings.Contains(body, "\"a1\"") || strings.Contains(body, "\"b1\"") {
			t.Errorf("both groups should be resolved, got:\n%s", body)
		}
	})

	t.Run("衝突時は-2", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{
				"x/common": "one\n", "y/common": "one\n",
				"p/common": "two\n", "q/common": "two\n",
			},
			nil,
			"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
				"[targets.a.\"~/.a1\"]\nsrc = \"x/common\"\n"+
				"[targets.b.\"~/.b1\"]\nsrc = \"y/common\"\n"+
				"[targets.a.\"~/.a2\"]\nsrc = \"p/common\"\n"+
				"[targets.b.\"~/.b2\"]\nsrc = \"q/common\"\n",
		)
		var out, errOut bytes.Buffer
		if code := Resolve(store, &out, &errOut); code != 0 {
			t.Fatalf("exit = %d, want 0 (%q)", code, errOut.String())
		}
		if !strings.Contains(out.String(), "shared/common") {
			t.Errorf("should contain shared/common, got %q", out.String())
		}
		if !strings.Contains(out.String(), "shared/common-2") {
			t.Errorf("collision should use -2, got %q", out.String())
		}
		for _, p := range []string{"shared/common", "shared/common-2"} {
			if _, err := os.Stat(filepath.Join(store, filepath.FromSlash(p))); err != nil {
				t.Errorf("%s missing: %v", p, err)
			}
		}
		body := readToml(t, store)
		if !strings.Contains(body, "shared/common-2") {
			t.Errorf("toml should contain -2 entry, got:\n%s", body)
		}
	})

	t.Run("異内容ありは全体不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{
				"a1": "one\n", "b1": "one\n",
				"a2": "two\n", "b2": "two\n",
				"shared/a1": "other\n",
			},
			nil,
			"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
				"[targets.a.\"~/.a1\"]\nsrc = \"a1\"\n"+
				"[targets.b.\"~/.b1\"]\nsrc = \"b1\"\n"+
				"[targets.a.\"~/.a2\"]\nsrc = \"a2\"\n"+
				"[targets.b.\"~/.b2\"]\nsrc = \"b2\"\n",
		)
		before := readToml(t, store)
		var out, errOut bytes.Buffer
		if code := Resolve(store, &out, &errOut); code == 0 {
			t.Fatal("should fail on one different content")
		}
		if got := readToml(t, store); got != before {
			t.Errorf("must be unchanged on partial failure:\nbefore:\n%s\ngot:\n%s", before, got)
		}
		if _, err := os.Stat(filepath.Join(store, "shared", "a2")); !os.IsNotExist(err) {
			t.Errorf("second group must not be created on failure")
		}
	})
}

func TestResolveSharedDirValidation(t *testing.T) {
	for _, body := range []string{
		"shared_dir = \"/abs\"\ndefault_target = \"base\"\n[targets.win.\"~/.a\"]\nsrc = \"a-src\"\n",
		"shared_dir = \"../out\"\ndefault_target = \"base\"\n[targets.win.\"~/.a\"]\nsrc = \"a-src\"\n",
		"shared_dir = \"\"\ndefault_target = \"base\"\n[targets.win.\"~/.a\"]\nsrc = \"a-src\"\n",
	} {
		store, _ := setupStoreWithHome(t,
			map[string]string{"a-src": "same\n", "b-src": "same\n"},
			nil,
			body+"[targets.wsl.\"~/.b\"]\nsrc = \"b-src\"\n",
		)
		var out, errOut bytes.Buffer
		if code := Resolve(store, &out, &errOut); code == 0 {
			t.Errorf("invalid shared_dir %q: exit = 0, want non-zero", body)
		} else if !strings.Contains(errOut.String(), "shared_dir") {
			t.Errorf("stderr should contain shared_dir, got %q", errOut.String())
		}
	}
}

func TestResolveDirectoryError(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"b-src": "same\n"},
		nil,
		"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"dir-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	if err := os.MkdirAll(filepath.Join(store, "dir-src"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Resolve(store, &out, &errOut); code == 0 {
		t.Fatal("directory: exit = 0, want non-zero")
	} else if errOut.String() == "" {
		t.Error("stderr should contain error")
	}
}

func TestResolveLeavesEmptyParentDirs(t *testing.T) {
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
		t.Fatalf("exit = %d (%q)", code, errOut.String())
	}
	// 空親ディレクトリは残す（掃除しない）。
	for _, d := range []string{"dotfiles/win", "dotfiles/wsl"} {
		if fi, err := os.Stat(filepath.Join(store, d)); err != nil || !fi.IsDir() {
			t.Errorf("%s should remain as empty dir, err=%v", d, err)
		}
	}
}

func TestResolveIgnoresHome(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{
			"a-src": "same\n",
			"b-src": "same\n",
		},
		map[string]string{".vimrc": "home-different\n"},
		"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Resolve(store, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d (%q)", code, errOut.String())
	}
	got, _ := os.ReadFile(filepath.Join(home, ".vimrc"))
	if string(got) != "home-different\n" {
		t.Errorf("HOME must not be touched, got %q", got)
	}
}

func TestDoctorIgnoresSharedDir(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"a-src": "aaa\n", "b-src": "bbb\n"},
		nil,
		"shared_dir = \"shared\"\ndefault_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Doctor(store, &out, &errOut); code != 0 {
		t.Fatalf("doctor with shared_dir: exit = %d, want 0", code)
	}
	if out.String() != "No sharable entries.\n" {
		t.Errorf("stdout = %q", out.String())
	}
}
