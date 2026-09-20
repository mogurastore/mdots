package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: CLIコマンド境界 (mdots add の実行系・E2E)
// 実FS上の Store/HOME を用い、runWithWriters 経由の外部挙動のみを検証する。
// 登録のみ行いコピーしないこと、後続pullで回収できること、エラー時は
// mdots.tomlが不変であることを確認する。正規化・src算出・保存記法の網羅は
// 設定境界テスト、委譲・文面・exitはCLI境界テストに寄せる。

// setupAddEnv は add E2E用の Store/HOME を準備する。homeFiles/storeFilesの
// ネストした親ディレクトリは mkdir -p で作る。HOME は t.Setenv で隔離する。
func setupAddEnv(t *testing.T, storeFiles, homeFiles map[string]string, tomlBody string) (store, home string) {
	t.Helper()
	store = t.TempDir()
	home = t.TempDir()
	t.Setenv("HOME", home)
	for name, body := range storeFiles {
		p := filepath.Join(store, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range homeFiles {
		p := filepath.Join(home, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(store, "mdots.toml"), []byte(tomlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return store, home
}

func readTomlForAddTest(t *testing.T, store string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store, "mdots.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAddEndToEndRegistersWithoutCopying(t *testing.T) {
	store, _ := setupAddEnv(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"[entries]\n",
	)

	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"add", "~/.vimrc"}, store, &out, &errOut); code != 0 {
		t.Fatalf("run(add ~/.vimrc) exit = %d, want 0 (stderr=%q)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "~/.vimrc") || !strings.Contains(out.String(), "dotfiles/.vimrc") {
		t.Errorf("stdout should contain key and src, got %q", out.String())
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

// Seam: CLIコマンド境界 (mdots add --target の実行系・E2E)
// targets形式での登録・文面・pull回収を外部挙動で検証する。
func TestAddEndToEndWithTargetRegistersTargetsForm(t *testing.T) {
	store, _ := setupAddEnv(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"[entries]\n",
	)

	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"add", "--target", "win", "~/.vimrc"}, store, &out, &errOut); code != 0 {
		t.Fatalf("run(add --target win ~/.vimrc) exit = %d, want 0 (stderr=%q)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "~/.vimrc") || !strings.Contains(out.String(), "win") || !strings.Contains(out.String(), "dotfiles/win/.vimrc") {
		t.Errorf("stdout should contain key, target and src, got %q", out.String())
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
	if code := run([]string{"pull", "--target", "win"}, store); code != 0 {
		t.Fatalf("run(pull --target win) after add exit = %d, want 0", code)
	}
	got, err := os.ReadFile(filepath.Join(store, "dotfiles", "win", ".vimrc"))
	if err != nil {
		t.Fatalf("store read error after pull --target: %v", err)
	}
	if string(got) != "set number\n" {
		t.Errorf("store content = %q, want %q", got, "set number\n")
	}
}

// Seam: CLIコマンド境界 (mdots add --target の追記マージ・E2E)
// targets形式で別Targetを追記し、pullで回収できる外部挙動を検証する。
func TestAddEndToEndWithTargetMergesNewTarget(t *testing.T) {
	store, _ := setupAddEnv(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"[entries]\n\"~/.vimrc\" = { targets = { win = { src = \"dotfiles/win/.vimrc\" } } }\n",
	)
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"add", "--target", "wsl", "~/.vimrc"}, store, &out, &errOut); code != 0 {
		t.Fatalf("run(add --target wsl) exit = %d, want 0 (stderr=%q)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "wsl") || !strings.Contains(out.String(), "dotfiles/wsl/.vimrc") {
		t.Errorf("stdout should contain target and src, got %q", out.String())
	}
	body := readTomlForAddTest(t, store)
	if !strings.Contains(body, "win") || !strings.Contains(body, "wsl") {
		t.Errorf("mdots.toml should contain both targets, got:\n%s", body)
	}
	if code := run([]string{"pull", "--target", "wsl"}, store); code != 0 {
		t.Fatalf("run(pull --target wsl) after merge exit = %d, want 0", code)
	}
	got, err := os.ReadFile(filepath.Join(store, "dotfiles", "wsl", ".vimrc"))
	if err != nil {
		t.Fatalf("store read error after pull --target wsl: %v", err)
	}
	if string(got) != "set number\n" {
		t.Errorf("store content = %q, want %q", got, "set number\n")
	}
}

func TestAddEndToEndPullCollectsAfterAdd(t *testing.T) {
	store, _ := setupAddEnv(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"[entries]\n",
	)

	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"add", "~/.vimrc"}, store, &out, &errOut); code != 0 {
		t.Fatalf("run(add) exit = %d, want 0 (stderr=%q)", code, errOut.String())
	}
	if code := run([]string{"pull"}, store); code != 0 {
		t.Fatalf("run(pull) after add exit = %d, want 0", code)
	}
	got, err := os.ReadFile(filepath.Join(store, "dotfiles", ".vimrc"))
	if err != nil {
		t.Fatalf("store read error after pull: %v", err)
	}
	if string(got) != "set number\n" {
		t.Errorf("store content = %q, want %q", got, "set number\n")
	}
}

func TestAddEndToEndAcceptsAbsolutePathUnderHome(t *testing.T) {
	store, home := setupAddEnv(t,
		nil,
		map[string]string{".vimrc": "x\n"},
		"[entries]\n",
	)

	abs := filepath.Join(home, ".vimrc")
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"add", abs}, store, &out, &errOut); code != 0 {
		t.Fatalf("run(add %s) exit = %d, want 0 (stderr=%q)", abs, code, errOut.String())
	}
	if !strings.Contains(out.String(), "~/.vimrc") {
		t.Errorf("stdout should contain normalized key ~/.vimrc, got %q", out.String())
	}
	if body := readTomlForAddTest(t, store); !strings.Contains(body, "~/.vimrc") {
		t.Errorf("mdots.toml should contain normalized key, got:\n%s", body)
	}
}

func TestAddEndToEndErrorsLeaveTomlUnchanged(t *testing.T) {
	t.Run("登録済みdestは失敗し不変", func(t *testing.T) {
		store, _ := setupAddEnv(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"[entries]\n\"~/.vimrc\" = { src = \"dotfiles/.vimrc\" }\n",
		)
		before := readTomlForAddTest(t, store)
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"add", "~/.vimrc"}, store, &out, &errOut); code == 0 {
			t.Fatal("run(add registered): exit = 0, want non-zero")
		}
		if errOut.String() == "" {
			t.Error("stderr should not be empty on duplicate")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("素のsrc済みへのtarget追加は失敗し不変", func(t *testing.T) {
		store, _ := setupAddEnv(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"[entries]\n\"~/.vimrc\" = { src = \"dotfiles/.vimrc\" }\n",
		)
		before := readTomlForAddTest(t, store)
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"add", "--target", "win", "~/.vimrc"}, store, &out, &errOut); code == 0 {
			t.Fatal("run(add --target registered): exit = 0, want non-zero")
		}
		if errOut.String() == "" {
			t.Error("stderr should not be empty on duplicate with target")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("同一targetの再登録は失敗し不変", func(t *testing.T) {
		store, _ := setupAddEnv(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"[entries]\n\"~/.vimrc\" = { targets = { win = { src = \"dotfiles/win/.vimrc\" } } }\n",
		)
		before := readTomlForAddTest(t, store)
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"add", "--target", "win", "~/.vimrc"}, store, &out, &errOut); code == 0 {
			t.Fatal("run(add --target duplicate): exit = 0, want non-zero")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("Store側src既存は失敗し不変", func(t *testing.T) {
		store, _ := setupAddEnv(t,
			map[string]string{"dotfiles/.vimrc": "existing\n"},
			map[string]string{".vimrc": "x\n"},
			"[entries]\n",
		)
		before := readTomlForAddTest(t, store)
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"add", "~/.vimrc"}, store, &out, &errOut); code == 0 {
			t.Fatal("run(add existing src): exit = 0, want non-zero")
		}
		if errOut.String() == "" {
			t.Error("stderr should not be empty on existing src")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("dest不在は失敗し不変", func(t *testing.T) {
		store, _ := setupAddEnv(t, nil, nil, "[entries]\n")
		before := readTomlForAddTest(t, store)
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"add", "~/.missing"}, store, &out, &errOut); code == 0 {
			t.Fatal("run(add missing): exit = 0, want non-zero")
		}
		if errOut.String() == "" {
			t.Error("stderr should not be empty on missing dest")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("ディレクトリは失敗し不変", func(t *testing.T) {
		store, home := setupAddEnv(t, nil, nil, "[entries]\n")
		if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
			t.Fatal(err)
		}
		before := readTomlForAddTest(t, store)
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"add", "~/.config"}, store, &out, &errOut); code == 0 {
			t.Fatal("run(add dir): exit = 0, want non-zero")
		}
		if errOut.String() == "" {
			t.Error("stderr should not be empty on directory")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})

	t.Run("HOME外は失敗し不変", func(t *testing.T) {
		store, _ := setupAddEnv(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"[entries]\n",
		)
		before := readTomlForAddTest(t, store)
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		if code := runWithWriters([]string{"add", outside}, store, &out, &errOut); code == 0 {
			t.Fatalf("run(add %s): exit = 0, want non-zero", outside)
		}
		if errOut.String() == "" {
			t.Error("stderr should not be empty on outside-HOME")
		}
		if got := readTomlForAddTest(t, store); got != before {
			t.Errorf("mdots.toml must be unchanged on error:\nbefore:\n%s\ngot:\n%s", before, got)
		}
	})
}

func TestAddEndToEndPreservesCommentsAndAppends(t *testing.T) {
	store, _ := setupAddEnv(t,
		nil,
		map[string]string{".vimrc": "x\n"},
		"# my comment\n[entries]\n\"~/.bashrc\" = { src = \"dotfiles/.bashrc\" }\n",
	)
	before := readTomlForAddTest(t, store)
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"add", "~/.vimrc"}, store, &out, &errOut); code != 0 {
		t.Fatalf("run(add) exit = %d, want 0 (stderr=%q)", code, errOut.String())
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

func TestAddEndToEndWithoutStoreFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".vimrc")
	if err := os.WriteFile(target, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	var out, errOut bytes.Buffer
	if code := runWithWriters([]string{"add", "~/.vimrc"}, empty, &out, &errOut); code == 0 {
		t.Fatal("run(add) without Store: exit = 0, want non-zero")
	}
	if !strings.Contains(errOut.String(), "mdots.toml not found in "+empty) {
		t.Errorf("stderr should contain not-found error, got %q", errOut.String())
	}
}
