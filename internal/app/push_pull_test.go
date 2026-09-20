package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: 実行系境界 (push/pull の Target 解決・配線)
// 実FS上の Store/dest を用い、Push/Pull の外部挙動のみを検証する。
// 表面（フラグ解釈・exit 委譲）は internal/cli、解決規則の網羅は internal/config が担う。

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

		if err := Push(store, "", io.Discard); err != nil {
			t.Fatalf("Push without target error = %v, want nil", err)
		}
		if _, err := os.Stat(filepath.Join(home, ".common.conf")); err != nil {
			t.Errorf(".common.conf should be copied: %v", err)
		}
		for _, f := range []string{".win.conf", ".wsl.conf"} {
			if _, err := os.Stat(filepath.Join(home, f)); err == nil {
				t.Errorf("%s should NOT be copied without target", f)
			}
		}
	})

	t.Run("指定は指定なし＋一致のみ", func(t *testing.T) {
		store, home := setupStoreWithHome(t, storeFiles, nil, entriesToml)

		if err := Push(store, "win", io.Discard); err != nil {
			t.Fatalf("Push with target win error = %v, want nil", err)
		}
		for _, f := range []string{".common.conf", ".win.conf"} {
			if _, err := os.Stat(filepath.Join(home, f)); err != nil {
				t.Errorf("%s should be copied: %v", f, err)
			}
		}
		if _, err := os.Stat(filepath.Join(home, ".wsl.conf")); err == nil {
			t.Error(".wsl.conf should NOT be copied with target win")
		}
	})

	t.Run("未知Targetは指定なしのみ", func(t *testing.T) {
		store, home := setupStoreWithHome(t, storeFiles, nil, entriesToml)

		if err := Push(store, "linux", io.Discard); err != nil {
			t.Fatalf("Push with unknown target error = %v, want nil", err)
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

		if err := Pull(store, "win", io.Discard); err != nil {
			t.Fatalf("Pull with target win error = %v, want nil", err)
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
			t.Error("wsl.conf should NOT be pulled with target win")
		}
	})

	t.Run("無指定は指定なしのみ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, homeFiles, entriesToml)

		if err := Pull(store, "", io.Discard); err != nil {
			t.Fatalf("Pull without target error = %v, want nil", err)
		}
		if _, err := os.Stat(filepath.Join(store, "common.conf")); err != nil {
			t.Errorf("common.conf should be pulled: %v", err)
		}
		for _, f := range []string{"win.conf", "wsl.conf"} {
			if _, err := os.Stat(filepath.Join(store, f)); err == nil {
				t.Errorf("%s should NOT be pulled without target", f)
			}
		}
	})

	t.Run("未知Targetは指定なしのみ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, homeFiles, entriesToml)

		if err := Pull(store, "linux", io.Discard); err != nil {
			t.Fatalf("Pull with unknown target error = %v, want nil", err)
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

	if err := Push(sub, "", io.Discard); err == nil {
		t.Fatal("Push from subdir: error = nil, want non-nil")
	}
	if _, err := os.Stat(filepath.Join(home, ".vimrc")); err == nil {
		t.Error("dest must NOT be created from subdir")
	}
}

// 同期系エラーの伝播代表例。欠落・種別の網羅は同期境界テストが保証する。
func TestPullMissingDestIsError(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		nil,
		"[entries]\n\"~/.vimrc\" = { src = \"vimrc\" }\n",
	)

	if err := Pull(store, "", io.Discard); err == nil {
		t.Error("Pull with missing dest: error = nil, want non-nil")
	}
}

// Seam: 実行系境界 (push/pull の override 保護配線代表例)
// 実FS上の Store/dest を用い、保護と継続の外部挙動のみを検証する。
// 新規作成・権限・差分詳細は同期・差分の各境界テストに寄せる。
// skip 警告の文面は TestPushOverrideSkippedWarnsToStderr が押さえる。
func TestPushOverrideProtectsExistingRepresentative(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"vimrc": "new\n"},
		map[string]string{".vimrc": "old\n"},
		"[entries]\n\"~/.vimrc\" = { src = \"vimrc\", override = false }\n",
	)

	if err := Push(store, "", io.Discard); err != nil {
		t.Fatalf("Push with override=false error = %v, want nil", err)
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

	if err := Pull(store, "", io.Discard); err != nil {
		t.Fatalf("Pull with override=false error = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(store, "vimrc"))
	if err != nil {
		t.Fatalf("store read error: %v", err)
	}
	if string(got) != "old\n" {
		t.Errorf("protected store content = %q, want %q", got, "old\n")
	}
}

// Seam: 実行系境界 (targets 一覧)
// Store を用い、Targets の外部挙動のみを検証する。
// 集約・ソートの網羅は設定境界テストが保証する。
// 余剰引数拒否は CLI 表面（internal/cli）の責務のためここでは扱わない。
func TestTargetsRepresentative(t *testing.T) {
	t.Run("分散したTargetを重複排除・ソートして返す", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil,
			"[entries]\n"+
				`"~/.c" = { targets = { wsl = { src = "c-wsl" }, win = { src = "c-win" } } }`+"\n"+
				`"~/.b" = { targets = { win = { src = "b-win" } } }`+"\n"+
				`"~/.a" = { src = "a" }`+"\n",
		)
		got, err := Targets(store)
		if err != nil {
			t.Fatalf("Targets error = %v, want nil", err)
		}
		if len(got) != 2 || got[0] != "win" || got[1] != "wsl" {
			t.Errorf("Targets = %q, want %q", got, []string{"win", "wsl"})
		}
	})

	t.Run("Target未定義で空・nilエラー", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil,
			"[entries]\n\"~/.a\" = { src = \"a\" }\n",
		)
		got, err := Targets(store)
		if err != nil {
			t.Fatalf("Targets error = %v, want nil", err)
		}
		if len(got) != 0 {
			t.Errorf("Targets = %q, want empty", got)
		}
	})

	t.Run("toml不正でエラー", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil,
			"[entries]\n\"~/.a\" = {}\n",
		)
		if _, err := Targets(store); err == nil {
			t.Error("Targets with invalid toml: error = nil, want non-nil")
		}
	})
}

// Seam: 実行系境界 (push/pull --dry-run の代表例)
// 実FS上の Store/dest を用い、書き込みなし・差分出力の外部挙動のみを検証する。
// フラグ解釈・exit 委譲は internal/cli、差分詳細は同期境界テストに寄せる。
func TestDryRunRepresentative(t *testing.T) {
	t.Run("pushは差分を出して書き込まない", func(t *testing.T) {
		store, home := setupStoreWithHome(t,
			map[string]string{"vimrc": "new\n"},
			map[string]string{".vimrc": "old\n"},
			"[entries]\n\"~/.vimrc\" = { src = \"vimrc\" }\n",
		)

		var out, errOut bytes.Buffer
		if code := PushDryRun(store, "", "auto", &out, &errOut); code == 0 {
			t.Error("PushDryRun with changes: exit = 0, want non-zero")
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
		if code := PullDryRun(store, "", "auto", &out, &errOut); code == 0 {
			t.Error("PullDryRun with changes: exit = 0, want non-zero")
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

		for _, tc := range []struct {
			name string
			fn   func(string, string, string, *bytes.Buffer, *bytes.Buffer) int
		}{
			{"push", func(cwd, target, color string, o, e *bytes.Buffer) int { return PushDryRun(cwd, target, color, o, e) }},
			{"pull", func(cwd, target, color string, o, e *bytes.Buffer) int { return PullDryRun(cwd, target, color, o, e) }},
		} {
			var out, errOut bytes.Buffer
			if code := tc.fn(store, "", "auto", &out, &errOut); code != 0 {
				t.Errorf("%s without changes: exit = %d, want 0", tc.name, code)
			}
			if out.String() != "No changes.\n" {
				t.Errorf("%s without changes: stdout = %q, want %q", tc.name, out.String(), "No changes.\n")
			}
		}
	})

	t.Run("Target指定で解決結果のみが差分対象になる", func(t *testing.T) {
		entriesToml := "[entries]\n" +
			`"~/.common.conf" = { src = "common.conf" }` + "\n" +
			`"~/.win.conf" = { targets = { win = { src = "win.conf" } } }` + "\n"
		store, _ := setupStoreWithHome(t,
			map[string]string{"common.conf": "same\n", "win.conf": "new\n"},
			map[string]string{".common.conf": "same\n", ".win.conf": "old\n"},
			entriesToml,
		)

		var out, errOut bytes.Buffer
		if code := PushDryRun(store, "win", "auto", &out, &errOut); code == 0 {
			t.Error("PushDryRun with target win and changes: exit = 0, want non-zero")
		}
		if !strings.Contains(out.String(), "win.conf") {
			t.Errorf("win Entryの差分を含むべき, got %q", out.String())
		}
		if strings.Contains(out.String(), "common.conf") {
			t.Errorf("差分なしの指定なしEntryを含めるべきでない, got %q", out.String())
		}

		out.Reset()
		errOut.Reset()
		if code := PushDryRun(store, "linux", "auto", &out, &errOut); code != 0 {
			t.Errorf("PushDryRun with unknown target without applicable diff: exit = %d, want 0 (out=%q)", code, out.String())
		}
	})

	t.Run("pullのTarget指定でも解決結果のみが差分対象になる", func(t *testing.T) {
		entriesToml := "[entries]\n" +
			`"~/.common.conf" = { src = "common.conf" }` + "\n" +
			`"~/.win.conf" = { targets = { win = { src = "win.conf" } } }` + "\n"
		store, _ := setupStoreWithHome(t,
			map[string]string{"common.conf": "same\n", "win.conf": "old\n"},
			map[string]string{".common.conf": "same\n", ".win.conf": "new\n"},
			entriesToml,
		)

		var out, errOut bytes.Buffer
		if code := PullDryRun(store, "win", "auto", &out, &errOut); code == 0 {
			t.Error("PullDryRun with target win and changes: exit = 0, want non-zero")
		}
		if !strings.Contains(out.String(), "win.conf") {
			t.Errorf("win Entryの差分を含むべき, got %q", out.String())
		}

		out.Reset()
		errOut.Reset()
		if code := PullDryRun(store, "linux", "auto", &out, &errOut); code != 0 {
			t.Errorf("PullDryRun with unknown target without applicable diff: exit = %d, want 0 (out=%q)", code, out.String())
		}
	})
}
