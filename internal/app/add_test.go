package app

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: 実行系境界 (add 登録・pull 回収)
// 実FS上の Store/HOME を用い、Add/Pull の外部挙動のみを検証する。
// 省略時は default_target へ登録し、明示時は指定 Target へ登録する。
// 登録のみ行いコピーしないこと、後続pullで回収できること、エラー時は
// mdots.tomlが不変であることを確認する。

func readTomlForAddTest(t *testing.T, store string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store, "mdots.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAddRegistersWithoutCopying(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"default_target = \"base\"\n",
	)

	key, src, resolved, err := Add(store, "~/.vimrc", "")
	if err != nil {
		t.Fatalf("Add(~/.vimrc) error = %v, want nil", err)
	}
	if key != "~/.vimrc" || src != "dotfiles/base/.vimrc" || resolved != "base" {
		t.Errorf("Add = (%q, %q, %q), want (%q, %q, %q)", key, src, resolved, "~/.vimrc", "dotfiles/base/.vimrc", "base")
	}
	body := readTomlForAddTest(t, store)
	if !strings.Contains(body, "~/.vimrc") || !strings.Contains(body, "dotfiles/base/.vimrc") {
		t.Errorf("mdots.toml should contain new Entry, got:\n%s", body)
	}
	if strings.Contains(body, "override") {
		t.Errorf("added Entry must not write override, got:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(store, "dotfiles", "base", ".vimrc")); err == nil {
		t.Error("Store src must NOT be created by add (pull collects it)")
	}
}

// Seam: 実行系境界 (add --target の登録)
func TestAddWithTargetRegistersTargetsForm(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"default_target = \"base\"\n",
	)

	key, src, resolved, err := Add(store, "~/.vimrc", "win")
	if err != nil {
		t.Fatalf("Add(~/.vimrc, win) error = %v, want nil", err)
	}
	if key != "~/.vimrc" || src != "dotfiles/win/.vimrc" || resolved != "win" {
		t.Errorf("Add = (%q, %q, %q), want (%q, %q, %q)", key, src, resolved, "~/.vimrc", "dotfiles/win/.vimrc", "win")
	}
	body := readTomlForAddTest(t, store)
	if !strings.Contains(body, "~/.vimrc") || !strings.Contains(body, "win") || !strings.Contains(body, "dotfiles/win/.vimrc") {
		t.Errorf("mdots.toml should contain targets Entry, got:\n%s", body)
	}
	if strings.Contains(body, "override") {
		t.Errorf("added Entry must not write override, got:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(store, "dotfiles", "win", ".vimrc")); err == nil {
		t.Error("Store src must NOT be created by add --target (pull collects it)")
	}
	if err := Pull(store, "win", io.Discard); err != nil {
		t.Fatalf("Pull(win) after add error = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(store, "dotfiles", "win", ".vimrc"))
	if err != nil {
		t.Fatalf("store read error after pull --target: %v", err)
	}
	if string(got) != "set number\n" {
		t.Errorf("store content = %q, want %q", got, "set number\n")
	}
}

// Seam: 実行系境界 (add --target の追記マージ・跨Target許可)
func TestAddWithTargetMergesNewTarget(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"default_target = \"base\"\n[targets.win.\"~/.vimrc\"]\nsrc = \"dotfiles/win/.vimrc\"\n",
	)
	key, src, resolved, err := Add(store, "~/.vimrc", "wsl")
	if err != nil {
		t.Fatalf("Add(~/.vimrc, wsl) error = %v, want nil", err)
	}
	if key != "~/.vimrc" || src != "dotfiles/wsl/.vimrc" || resolved != "wsl" {
		t.Errorf("Add = (%q, %q, %q), want (%q, %q, %q)", key, src, resolved, "~/.vimrc", "dotfiles/wsl/.vimrc", "wsl")
	}
	body := readTomlForAddTest(t, store)
	if !strings.Contains(body, "win") || !strings.Contains(body, "wsl") {
		t.Errorf("mdots.toml should contain both targets, got:\n%s", body)
	}
	if err := Pull(store, "wsl", io.Discard); err != nil {
		t.Fatalf("Pull(wsl) after merge error = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(store, "dotfiles", "wsl", ".vimrc"))
	if err != nil {
		t.Fatalf("store read error after pull --target wsl: %v", err)
	}
	if string(got) != "set number\n" {
		t.Errorf("store content = %q, want %q", got, "set number\n")
	}
}

func TestAddPullCollectsAfterAdd(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"default_target = \"base\"\n",
	)

	if _, _, _, err := Add(store, "~/.vimrc", ""); err != nil {
		t.Fatalf("Add error = %v, want nil", err)
	}
	if err := Pull(store, "", io.Discard); err != nil {
		t.Fatalf("Pull after add error = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(store, "dotfiles", "base", ".vimrc"))
	if err != nil {
		t.Fatalf("store read error after pull: %v", err)
	}
	if string(got) != "set number\n" {
		t.Errorf("store content = %q, want %q", got, "set number\n")
	}
}

func TestAddAcceptsAbsolutePathUnderHome(t *testing.T) {
	store, home := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "x\n"},
		"default_target = \"base\"\n",
	)

	abs := filepath.Join(home, ".vimrc")
	key, _, _, err := Add(store, abs, "")
	if err != nil {
		t.Fatalf("Add(%s) error = %v, want nil", abs, err)
	}
	if key != "~/.vimrc" {
		t.Errorf("key = %q, want normalized %q", key, "~/.vimrc")
	}
	if body := readTomlForAddTest(t, store); !strings.Contains(body, "~/.vimrc") {
		t.Errorf("mdots.toml should contain normalized key, got:\n%s", body)
	}
}

func TestAddErrorsLeaveTomlUnchanged(t *testing.T) {
	t.Run("同一キーの再登録は失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"dotfiles/base/.vimrc\"\n",
		)
		before := readTomlForAddTest(t, store)
		if _, _, _, err := Add(store, "~/.vimrc", ""); err == nil {
			t.Fatal("Add(registered): error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("同一targetの再登録は失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"default_target = \"base\"\n[targets.win.\"~/.vimrc\"]\nsrc = \"dotfiles/win/.vimrc\"\n",
		)
		before := readTomlForAddTest(t, store)
		if _, _, _, err := Add(store, "~/.vimrc", "win"); err == nil {
			t.Fatal("Add(duplicate target): error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("省略時の既定欠落は失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"[targets.base.\"~/.other\"]\nsrc = \"dotfiles/base/.other\"\n",
		)
		before := readTomlForAddTest(t, store)
		if _, _, _, err := Add(store, "~/.vimrc", ""); err == nil {
			t.Fatal("Add without default_target: error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("Store側src既存は失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"dotfiles/base/.vimrc": "existing\n"},
			map[string]string{".vimrc": "x\n"},
			"default_target = \"base\"\n",
		)
		before := readTomlForAddTest(t, store)
		if _, _, _, err := Add(store, "~/.vimrc", ""); err == nil {
			t.Fatal("Add(existing src): error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("dest不在は失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil, "default_target = \"base\"\n")
		before := readTomlForAddTest(t, store)
		if _, _, _, err := Add(store, "~/.missing", ""); err == nil {
			t.Fatal("Add(missing): error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("ディレクトリは失敗し不変", func(t *testing.T) {
		store, home := setupStoreWithHome(t, nil, nil, "default_target = \"base\"\n")
		if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
			t.Fatal(err)
		}
		before := readTomlForAddTest(t, store)
		if _, _, _, err := Add(store, "~/.config", ""); err == nil {
			t.Fatal("Add(dir): error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("HOME外は失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"default_target = \"base\"\n",
		)
		before := readTomlForAddTest(t, store)
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := Add(store, outside, ""); err == nil {
			t.Fatalf("Add(%s): error = nil, want non-nil", outside)
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})
}

func TestAddPreservesCommentsAndAppends(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "x\n"},
		"default_target = \"base\"\n# my comment\n[targets.base.\"~/.bashrc\"]\nsrc = \"dotfiles/base/.bashrc\"\n",
	)
	before := readTomlForAddTest(t, store)
	if _, _, _, err := Add(store, "~/.vimrc", ""); err != nil {
		t.Fatalf("Add error = %v, want nil", err)
	}
	body := readTomlForAddTest(t, store)
	if !strings.Contains(body, "# my comment") {
		t.Errorf("comment must be preserved, got:\n%s", body)
	}
	if !strings.Contains(body, "\"~/.bashrc\"") {
		t.Errorf("existing entry must be preserved byte-wise, got:\n%s", body)
	}
	if !strings.HasPrefix(body, before) {
		t.Errorf("existing bytes must be prefix-preserved:\nbefore:\n%s\ngot:\n%s", before, body)
	}
	if !strings.Contains(body, "~/.vimrc") || !strings.Contains(body, "dotfiles/base/.vimrc") {
		t.Errorf("new entry must be appended, got:\n%s", body)
	}
}

func TestAddWithoutStoreFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	empty := t.TempDir()
	if _, _, _, err := Add(empty, "~/.vimrc", ""); err == nil {
		t.Fatal("Add without Store: error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), "mdots.toml not found in "+empty) {
		t.Errorf("error should contain not-found error, got %q", err.Error())
	}
}
