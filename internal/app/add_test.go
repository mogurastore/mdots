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
// 登録のみ行いコピーしないこと、後続pullで回収できること、エラー時は
// mdots.tomlが不変であることを確認する。正規化・src算出・保存記法の網羅は
// internal/config、委譲・文面・exitは internal/cli に寄せる。
// Store 準備・HOME 隔離は app_test.go の setupStoreWithHome に集約している。

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
		"[entries]\n",
	)

	key, src, err := Add(store, "~/.vimrc", "")
	if err != nil {
		t.Fatalf("Add(~/.vimrc) error = %v, want nil", err)
	}
	if key != "~/.vimrc" || src != "dotfiles/.vimrc" {
		t.Errorf("Add = (%q, %q), want (%q, %q)", key, src, "~/.vimrc", "dotfiles/.vimrc")
	}
	body := readTomlForAddTest(t, store)
	if !strings.Contains(body, "~/.vimrc") || !strings.Contains(body, "dotfiles/.vimrc") {
		t.Errorf("mdots.toml should contain new Entry, got:\n%s", body)
	}
	if strings.Contains(body, "override") {
		t.Errorf("added Entry must not write override, got:\n%s", body)
	}
	// 登録のみでコピーは行わない。Store側srcはまだ存在しない。
	if _, err := os.Stat(filepath.Join(store, "dotfiles", ".vimrc")); err == nil {
		t.Error("Store src must NOT be created by add (pull collects it)")
	}
}

// Seam: 実行系境界 (add --target の登録)
// targets形式での登録・pull回収を外部挙動で検証する。
func TestAddWithTargetRegistersTargetsForm(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"[entries]\n",
	)

	key, src, err := Add(store, "~/.vimrc", "win")
	if err != nil {
		t.Fatalf("Add(~/.vimrc, win) error = %v, want nil", err)
	}
	if key != "~/.vimrc" || src != "dotfiles/win/.vimrc" {
		t.Errorf("Add = (%q, %q), want (%q, %q)", key, src, "~/.vimrc", "dotfiles/win/.vimrc")
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

// Seam: 実行系境界 (add --target の追記マージ)
// targets形式で別Targetを追記し、pullで回収できる外部挙動を検証する。
func TestAddWithTargetMergesNewTarget(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"[entries]\n\"~/.vimrc\" = { targets = { win = { src = \"dotfiles/win/.vimrc\" } } }\n",
	)
	key, src, err := Add(store, "~/.vimrc", "wsl")
	if err != nil {
		t.Fatalf("Add(~/.vimrc, wsl) error = %v, want nil", err)
	}
	if key != "~/.vimrc" || src != "dotfiles/wsl/.vimrc" {
		t.Errorf("Add = (%q, %q), want (%q, %q)", key, src, "~/.vimrc", "dotfiles/wsl/.vimrc")
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
		"[entries]\n",
	)

	if _, _, err := Add(store, "~/.vimrc", ""); err != nil {
		t.Fatalf("Add error = %v, want nil", err)
	}
	if err := Pull(store, "", io.Discard); err != nil {
		t.Fatalf("Pull after add error = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(store, "dotfiles", ".vimrc"))
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
		"[entries]\n",
	)

	abs := filepath.Join(home, ".vimrc")
	key, _, err := Add(store, abs, "")
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
	t.Run("登録済みdestは失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"[entries]\n\"~/.vimrc\" = { src = \"dotfiles/.vimrc\" }\n",
		)
		before := readTomlForAddTest(t, store)
		if _, _, err := Add(store, "~/.vimrc", ""); err == nil {
			t.Fatal("Add(registered): error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("素のsrc済みへのtarget追加は失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"[entries]\n\"~/.vimrc\" = { src = \"dotfiles/.vimrc\" }\n",
		)
		before := readTomlForAddTest(t, store)
		if _, _, err := Add(store, "~/.vimrc", "win"); err == nil {
			t.Fatal("Add(registered, win): error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("同一targetの再登録は失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"[entries]\n\"~/.vimrc\" = { targets = { win = { src = \"dotfiles/win/.vimrc\" } } }\n",
		)
		before := readTomlForAddTest(t, store)
		if _, _, err := Add(store, "~/.vimrc", "win"); err == nil {
			t.Fatal("Add(duplicate target): error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("Store側src既存は失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"dotfiles/.vimrc": "existing\n"},
			map[string]string{".vimrc": "x\n"},
			"[entries]\n",
		)
		before := readTomlForAddTest(t, store)
		if _, _, err := Add(store, "~/.vimrc", ""); err == nil {
			t.Fatal("Add(existing src): error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("dest不在は失敗し不変", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil, "[entries]\n")
		before := readTomlForAddTest(t, store)
		if _, _, err := Add(store, "~/.missing", ""); err == nil {
			t.Fatal("Add(missing): error = nil, want non-nil")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("ディレクトリは失敗し不変", func(t *testing.T) {
		store, home := setupStoreWithHome(t, nil, nil, "[entries]\n")
		if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
			t.Fatal(err)
		}
		before := readTomlForAddTest(t, store)
		if _, _, err := Add(store, "~/.config", ""); err == nil {
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
			"[entries]\n",
		)
		before := readTomlForAddTest(t, store)
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Add(store, outside, ""); err == nil {
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
		"# my comment\n[entries]\n\"~/.bashrc\" = { src = \"dotfiles/.bashrc\" }\n",
	)
	before := readTomlForAddTest(t, store)
	if _, _, err := Add(store, "~/.vimrc", ""); err != nil {
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
	if !strings.Contains(body, "~/.vimrc") || !strings.Contains(body, "dotfiles/.vimrc") {
		t.Errorf("new entry must be appended, got:\n%s", body)
	}
}

func TestAddWithoutStoreFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	empty := t.TempDir()
	if _, _, err := Add(empty, "~/.vimrc", ""); err == nil {
		t.Fatal("Add without Store: error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), "mdots.toml not found in "+empty) {
		t.Errorf("error should contain not-found error, got %q", err.Error())
	}
}
