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
// 解決は単一 Target のみで、省略時は default_target へ解決する。
// 未定義名・欠落した default_target はエラーにする（旧黙示フォールバックの反転）。

func newEntriesToml() string {
	return "default_target = \"base\"\n" +
		"[targets.base.\"~/.base.conf\"]\nsrc = \"base.conf\"\n" +
		"[targets.win.\"~/.win.conf\"]\nsrc = \"win.conf\"\n" +
		"[targets.wsl.\"~/.wsl.conf\"]\nsrc = \"wsl.conf\"\n"
}

func TestPushWithTargetRepresentative(t *testing.T) {
	entriesToml := newEntriesToml()
	storeFiles := map[string]string{
		"base.conf": "base\n",
		"win.conf":  "win\n",
		"wsl.conf":  "wsl\n",
	}

	t.Run("省略時は既定のみ", func(t *testing.T) {
		store, home := setupStoreWithHome(t, storeFiles, nil, entriesToml)

		if err := Push(store, "", io.Discard, io.Discard); err != nil {
			t.Fatalf("Push without target error = %v, want nil", err)
		}
		if _, err := os.Stat(filepath.Join(home, ".base.conf")); err != nil {
			t.Errorf(".base.conf should be copied: %v", err)
		}
		for _, f := range []string{".win.conf", ".wsl.conf"} {
			if _, err := os.Stat(filepath.Join(home, f)); err == nil {
				t.Errorf("%s should NOT be copied with default", f)
			}
		}
	})

	t.Run("明示は単一のみ", func(t *testing.T) {
		store, home := setupStoreWithHome(t, storeFiles, nil, entriesToml)

		if err := Push(store, "win", io.Discard, io.Discard); err != nil {
			t.Fatalf("Push with target win error = %v, want nil", err)
		}
		if _, err := os.Stat(filepath.Join(home, ".win.conf")); err != nil {
			t.Errorf(".win.conf should be copied: %v", err)
		}
		for _, f := range []string{".base.conf", ".wsl.conf"} {
			if _, err := os.Stat(filepath.Join(home, f)); err == nil {
				t.Errorf("%s should NOT be copied with target win", f)
			}
		}
	})

	t.Run("未知Targetはエラー", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, storeFiles, nil, entriesToml)

		err := Push(store, "linux", io.Discard, io.Discard)
		if err == nil {
			t.Fatal("Push with unknown target: error = nil, want non-nil")
		}
		for _, want := range []string{"linux", "default_target", "targets"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("未知エラーは %q を含むべき, got %q", want, err.Error())
			}
		}
	})

	t.Run("同一destの跨Targetは各々解決", func(t *testing.T) {
		toml := "default_target = \"base\"\n" +
			"[targets.base.\"~/.shared\"]\nsrc = \"base-shared\"\n" +
			"[targets.wsl.\"~/.shared\"]\nsrc = \"wsl-shared\"\n"
		files := map[string]string{"base-shared": "base\n", "wsl-shared": "wsl\n"}
		store, home := setupStoreWithHome(t, files, nil, toml)
		if err := Push(store, "", io.Discard, io.Discard); err != nil {
			t.Fatalf("Push default error = %v", err)
		}
		got, err := os.ReadFile(filepath.Join(home, ".shared"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "base\n" {
			t.Errorf("shared content = %q, want base", got)
		}
		store2, home2 := setupStoreWithHome(t, files, nil, toml)
		if err := Push(store2, "wsl", io.Discard, io.Discard); err != nil {
			t.Fatalf("Push wsl error = %v", err)
		}
		got2, err := os.ReadFile(filepath.Join(home2, ".shared"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got2) != "wsl\n" {
			t.Errorf("shared content = %q, want wsl", got2)
		}
	})
}

func TestPullWithTargetRepresentative(t *testing.T) {
	entriesToml := newEntriesToml()
	homeFiles := map[string]string{
		".base.conf": "base edited\n",
		".win.conf":  "win edited\n",
		".wsl.conf":  "wsl edited\n",
	}

	t.Run("明示は単一のみ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, homeFiles, entriesToml)

		if err := Pull(store, "win", io.Discard, io.Discard); err != nil {
			t.Fatalf("Pull with target win error = %v, want nil", err)
		}
		got, err := os.ReadFile(filepath.Join(store, "win.conf"))
		if err != nil {
			t.Fatalf("store read error: %v", err)
		}
		if string(got) != "win edited\n" {
			t.Errorf("store win.conf content = %q, want %q", got, "win edited\n")
		}
		for _, f := range []string{"base.conf", "wsl.conf"} {
			if _, err := os.Stat(filepath.Join(store, f)); err == nil {
				t.Errorf("%s should NOT be pulled with target win", f)
			}
		}
	})

	t.Run("省略時は既定のみ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, homeFiles, entriesToml)

		if err := Pull(store, "", io.Discard, io.Discard); err != nil {
			t.Fatalf("Pull without target error = %v, want nil", err)
		}
		if _, err := os.Stat(filepath.Join(store, "base.conf")); err != nil {
			t.Errorf("base.conf should be pulled: %v", err)
		}
		for _, f := range []string{"win.conf", "wsl.conf"} {
			if _, err := os.Stat(filepath.Join(store, f)); err == nil {
				t.Errorf("%s should NOT be pulled with default", f)
			}
		}
	})

	t.Run("未知Targetはエラー", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, homeFiles, entriesToml)

		if err := Pull(store, "linux", io.Discard, io.Discard); err == nil {
			t.Fatal("Pull with unknown target: error = nil, want non-nil")
		}
	})
}

func TestPushFromSubdirFails(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"dotfiles/base/vimrc": "x\n"},
		nil,
		"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"dotfiles/base/vimrc\"\n",
	)
	sub := filepath.Join(store, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Push(sub, "", io.Discard, io.Discard); err == nil {
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
		"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\n",
	)

	if err := Pull(store, "", io.Discard, io.Discard); err == nil {
		t.Error("Pull with missing dest: error = nil, want non-nil")
	}
}

// Seam: 実行系境界 (push/pull の override 保護配線代表例)
func TestPushOverrideProtectsExistingRepresentative(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"vimrc": "new\n"},
		map[string]string{".vimrc": "old\n"},
		"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\noverride = false\n",
	)

	if err := Push(store, "", io.Discard, io.Discard); err != nil {
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
		"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\noverride = false\n",
	)

	if err := Pull(store, "", io.Discard, io.Discard); err != nil {
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

// Seam: 実行系境界 (push/pull 成功報告)
func TestPushPullReportCopiedRepresentative(t *testing.T) {
	t.Run("pushはsrc->destをstdoutへ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "new\n"},
			map[string]string{".vimrc": "old\n"},
			"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\n",
		)
		var out, errOut bytes.Buffer
		if err := Push(store, "", &out, &errOut); err != nil {
			t.Fatalf("Push error = %v, want nil", err)
		}
		if out.String() != "copied vimrc -> ~/.vimrc\n" {
			t.Errorf("stdout = %q, want %q", out.String(), "copied vimrc -> ~/.vimrc\n")
		}
		if errOut.String() != "" {
			t.Errorf("stderr = %q, want empty", errOut.String())
		}
	})

	t.Run("pullはdest->srcをstdoutへ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "old\n"},
			map[string]string{".vimrc": "new\n"},
			"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\n",
		)
		var out, errOut bytes.Buffer
		if err := Pull(store, "", &out, &errOut); err != nil {
			t.Fatalf("Pull error = %v, want nil", err)
		}
		if out.String() != "copied ~/.vimrc -> vimrc\n" {
			t.Errorf("stdout = %q, want %q", out.String(), "copied ~/.vimrc -> vimrc\n")
		}
	})

	t.Run("複数は配置先ソート順", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"b-src": "b\n", "a-src": "a\n"},
			nil,
			"default_target = \"base\"\n[targets.base.\"~/.b\"]\nsrc = \"b-src\"\n[targets.base.\"~/.a\"]\nsrc = \"a-src\"\n",
		)
		var out bytes.Buffer
		if err := Push(store, "", &out, io.Discard); err != nil {
			t.Fatalf("Push error = %v, want nil", err)
		}
		want := "copied a-src -> ~/.a\ncopied b-src -> ~/.b\n"
		if out.String() != want {
			t.Errorf("stdout = %q, want %q", out.String(), want)
		}
	})

	t.Run("全skipはNo changesと警告", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "new\n"},
			map[string]string{".vimrc": "old\n"},
			"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\noverride = false\n",
		)
		var out, errOut bytes.Buffer
		if err := Push(store, "", &out, &errOut); err != nil {
			t.Fatalf("Push error = %v, want nil", err)
		}
		if out.String() != "No changes.\n" {
			t.Errorf("stdout = %q, want %q", out.String(), "No changes.\n")
		}
		if !strings.Contains(errOut.String(), "skipped: ~/.vimrc") {
			t.Errorf("stderr should contain skipped, got %q", errOut.String())
		}
	})

	t.Run("一部skipは成功分のみ報告", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"a-src": "a\n", "b-src": "b-new\n"},
			map[string]string{".b": "b-old\n"},
			"default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a-src\"\n[targets.base.\"~/.b\"]\nsrc = \"b-src\"\noverride = false\n",
		)
		var out, errOut bytes.Buffer
		if err := Push(store, "", &out, &errOut); err != nil {
			t.Fatalf("Push error = %v, want nil", err)
		}
		if out.String() != "copied a-src -> ~/.a\n" {
			t.Errorf("stdout = %q, want only copied a", out.String())
		}
		if !strings.Contains(errOut.String(), "skipped: ~/.b") {
			t.Errorf("stderr should contain skipped b, got %q", errOut.String())
		}
	})

	t.Run("pull全skipはNo changesと警告", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "old\n"},
			map[string]string{".vimrc": "new\n"},
			"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\noverride = false\n",
		)
		var out, errOut bytes.Buffer
		if err := Pull(store, "", &out, &errOut); err != nil {
			t.Fatalf("Pull error = %v, want nil", err)
		}
		if out.String() != "No changes.\n" {
			t.Errorf("stdout = %q, want %q", out.String(), "No changes.\n")
		}
		if !strings.Contains(errOut.String(), "skipped: ~/.vimrc") {
			t.Errorf("stderr should contain skipped, got %q", errOut.String())
		}
	})

	t.Run("pull一部skipは成功分のみ報告", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"a-src": "a-old\n", "b-src": "b-old\n"},
			map[string]string{".a": "a-new\n", ".b": "b-new\n"},
			"default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a-src\"\n[targets.base.\"~/.b\"]\nsrc = \"b-src\"\noverride = false\n",
		)
		var out, errOut bytes.Buffer
		if err := Pull(store, "", &out, &errOut); err != nil {
			t.Fatalf("Pull error = %v, want nil", err)
		}
		if out.String() != "copied ~/.a -> a-src\n" {
			t.Errorf("stdout = %q, want only copied a", out.String())
		}
		if !strings.Contains(errOut.String(), "skipped: ~/.b") {
			t.Errorf("stderr should contain skipped b, got %q", errOut.String())
		}
	})

	t.Run("Entry0件はNo changes", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil,
			"default_target = \"base\"\n[targets.base]\n",
		)
		var out, errOut bytes.Buffer
		if err := Push(store, "", &out, &errOut); err != nil {
			t.Fatalf("Push error = %v, want nil", err)
		}
		if out.String() != "No changes.\n" {
			t.Errorf("push stdout = %q, want %q", out.String(), "No changes.\n")
		}
		out.Reset()
		errOut.Reset()
		if err := Pull(store, "", &out, &errOut); err != nil {
			t.Fatalf("Pull error = %v, want nil", err)
		}
		if out.String() != "No changes.\n" {
			t.Errorf("pull stdout = %q, want %q", out.String(), "No changes.\n")
		}
	})

	t.Run("エラー時はstdoutに出さない", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			nil, nil,
			"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\n",
		)
		var out, errOut bytes.Buffer
		if err := Push(store, "", &out, &errOut); err == nil {
			t.Fatal("Push with missing src: error = nil, want non-nil")
		}
		if out.String() != "" {
			t.Errorf("push stdout on error = %q, want empty", out.String())
		}
		out.Reset()
		errOut.Reset()
		if err := Pull(store, "", &out, &errOut); err == nil {
			t.Fatal("Pull with missing dest: error = nil, want non-nil")
		}
		if out.String() != "" {
			t.Errorf("pull stdout on error = %q, want empty", out.String())
		}
	})

	t.Run("部分コピー後のエラーは報告しない", func(t *testing.T) {
		store, home := setupStoreWithHome(t,
			map[string]string{"a-src": "a\n"},
			nil,
			"default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a-src\"\n[targets.base.\"~/.b\"]\nsrc = \"b-missing\"\n",
		)
		var out, errOut bytes.Buffer
		if err := Push(store, "", &out, &errOut); err == nil {
			t.Fatal("Push partial error: error = nil, want non-nil")
		}
		if out.String() != "" {
			t.Errorf("push stdout on partial error = %q, want empty", out.String())
		}
		if _, err := os.Stat(filepath.Join(home, ".a")); err != nil {
			t.Errorf("first entry should be copied before error: %v", err)
		}

		store2, _ := setupStoreWithHome(t,
			map[string]string{"a-src": "a-old\n"},
			map[string]string{".a": "a-new\n"},
			"default_target = \"base\"\n[targets.base.\"~/.a\"]\nsrc = \"a-src\"\n[targets.base.\"~/.b\"]\nsrc = \"b-src\"\n",
		)
		out.Reset()
		errOut.Reset()
		if err := Pull(store2, "", &out, &errOut); err == nil {
			t.Fatal("Pull partial error: error = nil, want non-nil")
		}
		if out.String() != "" {
			t.Errorf("pull stdout on partial error = %q, want empty", out.String())
		}
	})
}

// Seam: 実行系境界 (targets 一覧・既定印)
func TestTargetsRepresentative(t *testing.T) {
	t.Run("分散したTargetを重複排除・ソートし既定に印", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil,
			"default_target = \"win\"\n"+
				"[targets.wsl.\"~/.c\"]\nsrc = \"c-wsl\"\n"+
				"[targets.win.\"~/.c\"]\nsrc = \"c-win\"\n"+
				"[targets.win.\"~/.b\"]\nsrc = \"b-win\"\n",
		)
		got, err := Targets(store)
		if err != nil {
			t.Fatalf("Targets error = %v, want nil", err)
		}
		want := []string{"win (default)", "wsl"}
		if len(got) != len(want) {
			t.Fatalf("Targets = %q, want %q", got, want)
		}
		for i, w := range want {
			if got[i] != w {
				t.Errorf("index %d: got %q, want %q", i, got[i], w)
			}
		}
	})

	t.Run("toml不正でエラー", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil,
			"default_target = \"base\"\n[targets.base.\"~/.a\"]\noverride = false\n",
		)
		if _, err := Targets(store); err == nil {
			t.Error("Targets with invalid toml: error = nil, want non-nil")
		}
	})
}

// Seam: 実行系境界 (push/pull --dry-run の代表例)
func TestDryRunRepresentative(t *testing.T) {
	t.Run("pushは差分を出して書き込まない", func(t *testing.T) {
		store, home := setupStoreWithHome(t,
			map[string]string{"vimrc": "new\n"},
			map[string]string{".vimrc": "old\n"},
			"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\n",
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
			"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\n",
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
			"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\n",
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
		entriesToml := "default_target = \"base\"\n" +
			"[targets.base.\"~/.base.conf\"]\nsrc = \"base.conf\"\n" +
			"[targets.win.\"~/.win.conf\"]\nsrc = \"win.conf\"\n"
		store, _ := setupStoreWithHome(t,
			map[string]string{"base.conf": "same\n", "win.conf": "new\n"},
			map[string]string{".base.conf": "same\n", ".win.conf": "old\n"},
			entriesToml,
		)

		var out, errOut bytes.Buffer
		if code := PushDryRun(store, "win", "auto", &out, &errOut); code == 0 {
			t.Error("PushDryRun with target win and changes: exit = 0, want non-zero")
		}
		if !strings.Contains(out.String(), "win.conf") {
			t.Errorf("win Entryの差分を含むべき, got %q", out.String())
		}
		if strings.Contains(out.String(), "base.conf") {
			t.Errorf("差分なしの既定Entryを含めるべきでない, got %q", out.String())
		}

		out.Reset()
		errOut.Reset()
		if code := PushDryRun(store, "linux", "auto", &out, &errOut); code == 0 {
			// 未知はエラー扱いで exit 1 だが、差分なしの 0 ではないことを確認する。
			// エラー時は stderr に文言が出る。
			if errOut.String() == "" {
				t.Errorf("未知Targetはstderrにエラーのはず, got stdout=%q stderr=%q", out.String(), errOut.String())
			}
		}
		if !strings.Contains(errOut.String(), "linux") {
			t.Errorf("未知エラーはTarget名を含むべき, got %q", errOut.String())
		}
	})

	t.Run("pullのTarget指定でも解決結果のみが差分対象になる", func(t *testing.T) {
		entriesToml := "default_target = \"base\"\n" +
			"[targets.base.\"~/.base.conf\"]\nsrc = \"base.conf\"\n" +
			"[targets.win.\"~/.win.conf\"]\nsrc = \"win.conf\"\n"
		store, _ := setupStoreWithHome(t,
			map[string]string{"base.conf": "same\n", "win.conf": "old\n"},
			map[string]string{".base.conf": "same\n", ".win.conf": "new\n"},
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
		if code := PullDryRun(store, "linux", "auto", &out, &errOut); code == 0 {
			if errOut.String() == "" {
				t.Errorf("未知Targetはstderrにエラーのはず, got stdout=%q", out.String())
			}
		}
	})
}

// Seam: 実行系境界 (手書き登録の環境変数dest)
// add は環境変数を扱わず、mdots.toml への手書き登録を想定する。
// push/pull で先頭の単一変数のみ展開し、未設定時はエラーで中断する。
func TestPushPullWithEnvDest(t *testing.T) {
	toml := "default_target = \"base\"\n" +
		"[targets.base.\"$MDOTS_E2E_DIR\"]\nsrc = \"app.conf\"\n"

	t.Run("pushで展開してコピーする", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), "app.conf")
		t.Setenv("MDOTS_E2E_DIR", envFile)
		store, _ := setupStoreWithHome(t,
			map[string]string{"app.conf": "hello\n"},
			nil,
			toml,
		)

		if err := Push(store, "", io.Discard, io.Discard); err != nil {
			t.Fatalf("Push error = %v, want nil", err)
		}
		got, err := os.ReadFile(envFile)
		if err != nil {
			t.Fatalf("env dest read error: %v", err)
		}
		if string(got) != "hello\n" {
			t.Errorf("env dest content = %q, want %q", got, "hello\n")
		}
	})

	t.Run("pullで回収できる", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), "app.conf")
		t.Setenv("MDOTS_E2E_DIR", envFile)
		if err := os.WriteFile(envFile, []byte("edited\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		store, _ := setupStoreWithHome(t, nil, nil, toml)

		if err := Pull(store, "", io.Discard, io.Discard); err != nil {
			t.Fatalf("Pull error = %v, want nil", err)
		}
		got, err := os.ReadFile(filepath.Join(store, "app.conf"))
		if err != nil {
			t.Fatalf("store read error: %v", err)
		}
		if string(got) != "edited\n" {
			t.Errorf("store content = %q, want %q", got, "edited\n")
		}
	})

	t.Run("未設定時はエラーで中断する", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"app.conf": "hello\n"},
			nil,
			toml,
		)

		if err := Push(store, "", io.Discard, io.Discard); err == nil {
			t.Fatal("Push with unset env: error = nil, want non-nil")
		} else if !strings.Contains(err.Error(), "MDOTS_E2E_DIR") {
			t.Errorf("変数名を含むべき, got %q", err.Error())
		}
	})
}
