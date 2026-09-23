package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: CLIコマンド境界 (sort 正規形ソート)
// 実Store上の mdots.toml を辞書順に並べ替え、成功時は無言 exit 0 で終える。
func TestCliSortDispatches(t *testing.T) {
	store, _ := setupStoreWithHome(t, nil, nil,
		"default_target = \"base\"\n"+
			"[targets.wsl.\"~/.gitconfig\"]\nsrc = \"dotfiles/wsl/.gitconfig\"\n"+
			"[targets.win.\"~/.gitconfig\"]\nsrc = \"dotfiles/win/.gitconfig\"\n",
	)
	code, out, errOut := runCli(t, store, []string{"sort"})
	if code != 0 {
		t.Fatalf("Run(sort) exit = %d, want 0 (stderr=%q)", code, errOut)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty (silent on success)", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
	body, err := os.ReadFile(filepath.Join(store, "mdots.toml"))
	if err != nil {
		t.Fatal(err)
	}
	want := "default_target = \"base\"\n" +
		"\n" +
		"[targets.win.\"~/.gitconfig\"]\nsrc = \"dotfiles/win/.gitconfig\"\n" +
		"\n" +
		"[targets.wsl.\"~/.gitconfig\"]\nsrc = \"dotfiles/wsl/.gitconfig\"\n"
	if string(body) != want {
		t.Errorf("ソート結果不正:\ngot:\n%s\nwant:\n%s", body, want)
	}
}

func TestCliSortIdempotentAndErrors(t *testing.T) {
	t.Run("整列済みは不変でexit0", func(t *testing.T) {
		body := "default_target = \"base\"\n\n[targets.base.\"~/.a\"]\nsrc = \"a\"\n"
		store, _ := setupStoreWithHome(t, nil, nil, body)
		code, out, _ := runCli(t, store, []string{"sort"})
		if code != 0 {
			t.Fatalf("Run(sort) exit = %d, want 0", code)
		}
		if out != "" {
			t.Errorf("stdout = %q, want empty", out)
		}
		got, err := os.ReadFile(filepath.Join(store, "mdots.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != body {
			t.Errorf("不変 expected:\nbefore:\n%s\ngot:\n%s", body, got)
		}
	})

	t.Run("余剰引数は拒否", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil, "default_target = \"base\"\n")
		code, _, errOut := runCli(t, store, []string{"sort", "extra"})
		if code == 0 {
			t.Error("Run(sort extra): exit = 0, want non-zero")
		}
		if !strings.Contains(errOut, "unknown argument: extra") {
			t.Errorf("stderr should contain unknown argument, got %q", errOut)
		}
	})

	t.Run("Store不在は失敗", func(t *testing.T) {
		empty := t.TempDir()
		code, _, errOut := runCli(t, empty, []string{"sort"})
		if code == 0 {
			t.Fatal("Run(sort) without Store: exit = 0, want non-zero")
		}
		if !strings.Contains(errOut, "mdots.toml not found in "+empty) {
			t.Errorf("stderr should contain not-found error, got %q", errOut)
		}
	})
}

func TestCliSortHelp(t *testing.T) {
	for _, args := range [][]string{{"sort", "--help"}, {"sort", "-h"}} {
		store, _ := setupStoreWithHome(t, nil, nil, "default_target = \"base\"\n")
		code, out, _ := runCli(t, store, args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", args, code)
		}
		for _, want := range []string{"USAGE:", "sort"} {
			if !strings.Contains(out, want) {
				t.Errorf("Run(%v): output should contain %q, got %q", args, want, out)
			}
		}
	}
}
