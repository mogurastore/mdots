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
// ネストした親ディレクトリは mkdir -p で作る。
// 本パッケージ唯一の定義である（挙動の網羅は internal/app、表面は internal/cli が担う）。
func setupStoreWithHome(t *testing.T, storeFiles, homeFiles map[string]string, tomlBody string) (store, home string) {
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

// Seam: 実通し smoke (push の配線代表例)
// 実FS上の Store/dest を用い、run 経由の外部挙動のみを検証する。
// Target 展開・フラグ解釈・差分詳細は internal/cli・internal/app・internal/config・
// internal/sync の各境界テストに寄せ、ここでは配線の代表例だけを残す。
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

// Seam: 実通し smoke (add 登録の配線代表例)
// 登録のみ行いコピーしないことの外部挙動を runWithWriters 経由で確認する。
// 正規化・src算出・保存記法の網羅は internal/config、委譲・文面・exit は
// internal/cli、登録失敗時の不変性は internal/app の各境界テストに寄せる。
func TestAddEndToEndRegistersWithoutCopying(t *testing.T) {
	store, _ := setupStoreWithHome(t,
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
	// 登録のみでコピーは行わない。Store側srcはまだ存在しない。
	if _, err := os.Stat(filepath.Join(store, "dotfiles", ".vimrc")); err == nil {
		t.Error("Store src must NOT be created by add (pull collects it)")
	}
}
