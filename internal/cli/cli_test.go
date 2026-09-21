package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: CLIコマンド境界 (コマンド定義・Writer注入・exit)
// 実行系は実appに直結し、t.TempDir上の実Store/HOMEで外部挙動
// （exit code・出力文面・FS副作用）のみを検証する。
// Writer/ErrWriter注入は維持し、同一プロセスで完結する。

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

func runCli(t *testing.T, cwd string, args []string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, cwd, "v0.0.0-test", &out, &errOut)
	return code, out.String(), errOut.String()
}

func runCliEmpty(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	return runCli(t, t.TempDir(), args)
}

func baseWinToml() string {
	return "default_target = \"base\"\n" +
		"[targets.base.\"~/.base.conf\"]\nsrc = \"base.conf\"\n" +
		"[targets.win.\"~/.win.conf\"]\nsrc = \"win.conf\"\n"
}

func singleBaseToml() string {
	return "default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"vimrc\"\n"
}

func TestCliGlobalHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}} {
		code, out, _ := runCliEmpty(t, args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", args, code)
		}
		for _, want := range []string{"USAGE:", "COMMANDS:", "GLOBAL OPTIONS:", "push", "pull", "init", "add", "targets"} {
			if !strings.Contains(out, want) {
				t.Errorf("Run(%v): output should contain %q, got %q", args, want, out)
			}
		}
		if strings.Contains(out, "diff") {
			t.Errorf("Run(%v): output must not contain diff, got %q", args, out)
		}
	}
}

func TestCliNoArgsShowsStandardHelp(t *testing.T) {
	code, out, errOut := runCliEmpty(t, nil)
	if code != 0 {
		t.Fatalf("Run(nil) exit = %d, want 0", code)
	}
	if !strings.Contains(out, "USAGE:") {
		t.Errorf("stdout should contain USAGE:, got %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr should be empty, got %q", errOut)
	}
}

func TestCliVersion(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-v"}} {
		code, out, _ := runCliEmpty(t, args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", args, code)
		}
		if !strings.Contains(out, "mdots version v0.0.0-test") {
			t.Errorf("Run(%v): output should contain standard version, got %q", args, out)
		}
	}
}

func TestCliVersionOldFormsAreUnknown(t *testing.T) {
	t.Run("-Vは標準の未知フラグ扱い", func(t *testing.T) {
		code, out, errOut := runCliEmpty(t, []string{"-V"})
		if code == 0 {
			t.Fatal("Run(-V): exit = 0, want non-zero")
		}
		if strings.Contains(out, "mdots version ") {
			t.Errorf("Run(-V): must not show version, got %q", out)
		}
		if !strings.Contains(errOut, "Incorrect Usage") {
			t.Errorf("stderr should contain Incorrect Usage, got %q", errOut)
		}
	})

	t.Run("versionは未知コマンド扱い", func(t *testing.T) {
		code, out, errOut := runCliEmpty(t, []string{"version"})
		if code != 3 {
			t.Fatalf("Run(version): exit = %d, want 3", code)
		}
		if strings.Contains(out, "v0.0.0-test") {
			t.Errorf("Run(version): must not show version, got %q", out)
		}
		if !strings.Contains(errOut, "No help topic for 'version'") {
			t.Errorf("stderr should contain No help topic for 'version', got %q", errOut)
		}
	})
}

func TestCliUnknownCommand(t *testing.T) {
	code, _, errOut := runCliEmpty(t, []string{"frobnicate"})
	if code != 3 {
		t.Fatalf("Run(unknown) exit = %d, want 3", code)
	}
	if !strings.Contains(errOut, "No help topic for 'frobnicate'") {
		t.Errorf("stderr should contain No help topic for 'frobnicate', got %q", errOut)
	}
}

func TestCliDiffIsRemoved(t *testing.T) {
	for _, args := range [][]string{{"diff"}, {"diff", "--target", "win"}} {
		store := t.TempDir()
		home := t.TempDir()
		t.Setenv("HOME", home)
		code, _, _ := runCli(t, store, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
	}
}

func TestCliDryRunDispatchesTarget(t *testing.T) {
	newWinStore := func(t *testing.T) (string, string) {
		return setupStoreWithHome(t,
			map[string]string{"base.conf": "same\n", "win.conf": "new\n"},
			map[string]string{".base.conf": "same\n", ".win.conf": "old\n"},
			baseWinToml(),
		)
	}
	t.Run("push", func(t *testing.T) {
		tests := []struct {
			name string
			args []string
			want string
		}{
			{"no target resolves to default", []string{"push", "--dry-run"}, ""},
			{"space form", []string{"push", "--dry-run", "--target", "win"}, "win"},
			{"equals form", []string{"push", "--dry-run", "--target=win"}, "win"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				store, _ := newWinStore(t)
				code, out, _ := runCli(t, store, tt.args)
				if tt.want == "" {
					if code != 0 {
						t.Fatalf("Run(%v) exit = %d, want 0 (no applicable diff)", tt.args, code)
					}
					if out != "No changes.\n" {
						t.Errorf("Run(%v): stdout = %q, want No changes.", tt.args, out)
					}
					return
				}
				if code == 0 {
					t.Fatalf("Run(%v) exit = 0, want non-zero (win diff)", tt.args)
				}
				if !strings.Contains(out, "win.conf") {
					t.Errorf("Run(%v): output should contain win diff, got %q", tt.args, out)
				}
				if strings.Contains(out, "base.conf") {
					t.Errorf("Run(%v): must not contain diff-free entry, got %q", tt.args, out)
				}
			})
		}
	})
	t.Run("pull", func(t *testing.T) {
		tests := []struct {
			name string
			args []string
			want string
		}{
			{"no target resolves to default", []string{"pull", "--dry-run"}, ""},
			{"space form", []string{"pull", "--dry-run", "--target", "win"}, "win"},
			{"equals form", []string{"pull", "--dry-run", "--target=win"}, "win"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				store, _ := setupStoreWithHome(t,
					map[string]string{"base.conf": "same\n", "win.conf": "old\n"},
					map[string]string{".base.conf": "same\n", ".win.conf": "new\n"},
					baseWinToml(),
				)
				code, out, _ := runCli(t, store, tt.args)
				if tt.want == "" {
					if code != 0 {
						t.Fatalf("Run(%v) exit = %d, want 0", tt.args, code)
					}
					return
				}
				if code == 0 {
					t.Fatalf("Run(%v) exit = 0, want non-zero", tt.args)
				}
				if !strings.Contains(out, "win.conf") {
					t.Errorf("Run(%v): output should contain win diff, got %q", tt.args, out)
				}
			})
		}
	})
	t.Run("unknown target errors with hint", func(t *testing.T) {
		store, _ := newWinStore(t)
		code, _, errOut := runCli(t, store, []string{"push", "--dry-run", "--target", "linux"})
		if code == 0 {
			t.Fatal("unknown target: exit = 0, want non-zero")
		}
		for _, want := range []string{"linux", "default_target", "targets"} {
			if !strings.Contains(errOut, want) {
				t.Errorf("stderr should contain %q, got %q", want, errOut)
			}
		}
	})
}

func TestCliColorRequiresDryRun(t *testing.T) {
	for _, args := range [][]string{
		{"push", "--color=always"},
		{"pull", "--color=always"},
	} {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "same\n"},
			map[string]string{".vimrc": "same\n"},
			singleBaseToml(),
		)
		code, _, errOut := runCli(t, store, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
		if !strings.Contains(errOut, "--color requires --dry-run") {
			t.Errorf("Run(%v): stderr should contain --color requires --dry-run, got %q", args, errOut)
		}
		if got, err := os.ReadFile(filepath.Join(store, "vimrc")); err != nil || string(got) != "same\n" {
			t.Errorf("Store must be unchanged when --color without --dry-run")
		}
	}
}

func TestCliStandardUsageError(t *testing.T) {
	for _, args := range [][]string{
		{"push", "--target"},
		{"pull", "--target"},
		{"push", "--unknown"},
		{"pull", "--unknown"},
	} {
		code, out, errOut := runCliEmpty(t, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
		if !strings.Contains(errOut, "Incorrect Usage") {
			t.Errorf("Run(%v): stderr should contain Incorrect Usage, got %q", args, errOut)
		}
		if !strings.Contains(out, "USAGE:") {
			t.Errorf("Run(%v): stdout should contain USAGE:, got %q", args, out)
		}
	}
}

func TestCliEmptyAndExtraArgsStillRejected(t *testing.T) {
	for _, args := range [][]string{
		{"push", "--target="},
		{"pull", "--target="},
		{"push", "extra-positional"},
		{"pull", "extra-positional"},
		{"init", "extra"},
		{"push", "--", "--target", "win"},
		{"pull", "--", "--target", "win"},
		{"push", "--dry-run", "extra-positional"},
		{"pull", "--dry-run", "extra-positional"},
	} {
		code, _, _ := runCliEmpty(t, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
	}
}

func TestCliCommandHelp(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"push", "--help"}, []string{"USAGE:", "OPTIONS:", "push", "--target", "--dry-run", "--color"}},
		{[]string{"push", "-h"}, []string{"USAGE:", "OPTIONS:", "push", "--target", "--dry-run", "--color"}},
		{[]string{"pull", "--help"}, []string{"USAGE:", "OPTIONS:", "pull", "--target", "--dry-run", "--color"}},
		{[]string{"pull", "-h"}, []string{"USAGE:", "OPTIONS:", "pull", "--target", "--dry-run", "--color"}},
	}
	for _, tt := range tests {
		code, out, _ := runCliEmpty(t, tt.args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", tt.args, code)
		}
		for _, want := range tt.want {
			if !strings.Contains(out, want) {
				t.Errorf("Run(%v): output should contain %q, got %q", tt.args, want, out)
			}
		}
	}
}

func TestCliPushDispatchesTarget(t *testing.T) {
	entriesToml := baseWinToml()
	storeFiles := map[string]string{"base.conf": "base\n", "win.conf": "win\n"}
	tests := []struct {
		name string
		args []string
		want []string
		skip []string
	}{
		{"no target resolves to default", []string{"push"}, []string{".base.conf"}, []string{".win.conf"}},
		{"space form", []string{"push", "--target", "win"}, []string{".win.conf"}, []string{".base.conf"}},
		{"equals form", []string{"push", "--target=win"}, []string{".win.conf"}, []string{".base.conf"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, home := setupStoreWithHome(t, storeFiles, nil, entriesToml)
			code, _, errOut := runCli(t, store, tt.args)
			if code != 0 {
				t.Fatalf("Run(%v) exit = %d, want 0 (stderr=%q)", tt.args, code, errOut)
			}
			for _, f := range tt.want {
				if _, err := os.Stat(filepath.Join(home, f)); err != nil {
					t.Errorf("Run(%v): %s should be copied: %v", tt.args, f, err)
				}
			}
			for _, f := range tt.skip {
				if _, err := os.Stat(filepath.Join(home, f)); err == nil {
					t.Errorf("Run(%v): %s should NOT be copied", tt.args, f)
				}
			}
		})
	}
	t.Run("unknown target errors", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, storeFiles, nil, entriesToml)
		code, _, errOut := runCli(t, store, []string{"push", "--target", "linux"})
		if code == 0 {
			t.Fatal("unknown target: exit = 0, want non-zero")
		}
		for _, want := range []string{"linux", "default_target", "targets"} {
			if !strings.Contains(errOut, want) {
				t.Errorf("stderr should contain %q, got %q", want, errOut)
			}
		}
	})
}

func TestCliShortFlags(t *testing.T) {
	t.Run("push -t は --target と同じ", func(t *testing.T) {
		store, home := setupStoreWithHome(t,
			map[string]string{"base.conf": "base\n", "win.conf": "win\n"},
			nil,
			baseWinToml(),
		)
		code, _, errOut := runCli(t, store, []string{"push", "-t", "win"})
		if code != 0 {
			t.Fatalf("Run(push -t win) exit = %d, want 0 (stderr=%q)", code, errOut)
		}
		if _, err := os.Stat(filepath.Join(home, ".win.conf")); err != nil {
			t.Errorf(".win.conf should be copied: %v", err)
		}
		if _, err := os.Stat(filepath.Join(home, ".base.conf")); err == nil {
			t.Error(".base.conf should NOT be copied with -t win (single-target)")
		}
	})
	t.Run("pull -t は --target と同じ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			nil,
			map[string]string{".base.conf": "base\n", ".win.conf": "win\n"},
			baseWinToml(),
		)
		code, _, errOut := runCli(t, store, []string{"pull", "-t", "win"})
		if code != 0 {
			t.Fatalf("Run(pull -t win) exit = %d, want 0 (stderr=%q)", code, errOut)
		}
		if _, err := os.Stat(filepath.Join(store, "win.conf")); err != nil {
			t.Errorf("win.conf should be pulled: %v", err)
		}
		if _, err := os.Stat(filepath.Join(store, "base.conf")); err == nil {
			t.Error("base.conf should NOT be pulled with -t win")
		}
	})
	t.Run("push -n は --dry-run と同じ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "new\n"},
			map[string]string{".vimrc": "old\n"},
			singleBaseToml(),
		)
		code, out, _ := runCli(t, store, []string{"push", "-n"})
		if code == 0 {
			t.Fatal("Run(push -n) with changes: exit = 0, want non-zero")
		}
		if !strings.Contains(out, "---") {
			t.Errorf("output should contain diff, got %q", out)
		}
	})
	t.Run("pull -n -t の併用", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"base.conf": "same\n", "win.conf": "old\n"},
			map[string]string{".base.conf": "same\n", ".win.conf": "new\n"},
			baseWinToml(),
		)
		code, out, _ := runCli(t, store, []string{"pull", "-n", "-t", "win"})
		if code == 0 {
			t.Fatal("Run(pull -n -t win) with changes: exit = 0, want non-zero")
		}
		if !strings.Contains(out, "win.conf") {
			t.Errorf("output should contain win diff, got %q", out)
		}
	})
	t.Run("push -n -c は --color と同じ", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "same\n"},
			map[string]string{".vimrc": "same\n"},
			singleBaseToml(),
		)
		code, out, _ := runCli(t, store, []string{"push", "-n", "-c", "always"})
		if code != 0 {
			t.Fatalf("Run(push -n -c always) exit = %d, want 0", code)
		}
		if out != "No changes.\n" {
			t.Errorf("stdout = %q, want No changes.", out)
		}
	})
	t.Run("push -c 単独は --dry-run 必須エラー", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "same\n"},
			map[string]string{".vimrc": "same\n"},
			singleBaseToml(),
		)
		code, _, errOut := runCli(t, store, []string{"push", "-c", "always"})
		if code == 0 {
			t.Fatal("Run(push -c always): exit = 0, want non-zero")
		}
		if !strings.Contains(errOut, "--color requires --dry-run") {
			t.Errorf("stderr should contain --color requires --dry-run, got %q", errOut)
		}
	})
}

func TestCliTargetFlagErrors(t *testing.T) {
	for _, args := range [][]string{
		{"push", "--target"},
		{"push", "--target="},
		{"push", "--unknown"},
		{"push", "extra-positional"},
		{"pull", "--target"},
		{"pull", "--target="},
		{"pull", "--unknown"},
		{"pull", "extra-positional"},
		{"push", "--dry-run", "--target="},
		{"pull", "--dry-run", "--target="},
	} {
		if code, _, _ := runCliEmpty(t, args); code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
	}
}

func TestCliDryRunDelegatesExitCode(t *testing.T) {
	t.Run("push", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "new\n"},
			map[string]string{".vimrc": "old\n"},
			singleBaseToml(),
		)
		code, out, _ := runCli(t, store, []string{"push", "--dry-run"})
		if code != 1 {
			t.Fatalf("Run(push --dry-run) exit = %d, want 1", code)
		}
		if !strings.Contains(out, "---") {
			t.Errorf("output should contain diff, got %q", out)
		}
		if got, err := os.ReadFile(filepath.Join(store, "vimrc")); err != nil || string(got) != "new\n" {
			t.Error("Store must NOT be written on dry-run")
		}

		store, _ = setupStoreWithHome(t,
			map[string]string{"vimrc": "same\n"},
			map[string]string{".vimrc": "same\n"},
			singleBaseToml(),
		)
		if code, out, _ := runCli(t, store, []string{"push", "--dry-run"}); code != 0 {
			t.Errorf("Run(push --dry-run) without changes: exit = %d, want 0", code)
		} else if out != "No changes.\n" {
			t.Errorf("stdout = %q, want No changes.", out)
		}
	})
	t.Run("pull", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "old\n"},
			map[string]string{".vimrc": "new\n"},
			singleBaseToml(),
		)
		code, out, _ := runCli(t, store, []string{"pull", "--dry-run"})
		if code != 1 {
			t.Fatalf("Run(pull --dry-run) exit = %d, want 1", code)
		}
		if !strings.Contains(out, "---") {
			t.Errorf("output should contain diff, got %q", out)
		}

		store, _ = setupStoreWithHome(t,
			map[string]string{"vimrc": "same\n"},
			map[string]string{".vimrc": "same\n"},
			singleBaseToml(),
		)
		if code, _, _ := runCli(t, store, []string{"pull", "--dry-run"}); code != 0 {
			t.Errorf("Run(pull --dry-run) without changes: exit = %d, want 0", code)
		}
	})
}

func TestCliDryRunColorFlag(t *testing.T) {
	t.Run("既定はauto", func(t *testing.T) {
		for _, args := range [][]string{{"push", "--dry-run"}, {"pull", "--dry-run"}} {
			store, _ := setupStoreWithHome(t,
				map[string]string{"vimrc": "same\n"},
				map[string]string{".vimrc": "same\n"},
				singleBaseToml(),
			)
			if code, out, _ := runCli(t, store, args); code != 0 {
				t.Fatalf("Run(%v) exit = %d, want 0", args, code)
			} else if out != "No changes.\n" {
				t.Errorf("Run(%v): stdout = %q, want No changes.", args, out)
			}
		}
	})

	t.Run("always/neverを通す", func(t *testing.T) {
		for _, c := range []string{"always", "never"} {
			for _, base := range []string{"push", "pull"} {
				store, _ := setupStoreWithHome(t,
					map[string]string{"vimrc": "same\n"},
					map[string]string{".vimrc": "same\n"},
					singleBaseToml(),
				)
				args := []string{base, "--dry-run", "--color=" + c}
				if code, _, _ := runCli(t, store, args); code != 0 {
					t.Errorf("Run(%v) exit = %d, want 0", args, code)
				}
			}
		}
	})

	t.Run("不正値はexit1", func(t *testing.T) {
		for _, base := range []string{"push", "pull"} {
			args := []string{base, "--dry-run", "--color=foo"}
			if code, _, errOut := runCliEmpty(t, args); code == 0 {
				t.Error("exit = 0, want non-zero")
			} else if !strings.Contains(errOut, "invalid value for --color") {
				t.Errorf("stderr should contain invalid value, got %q", errOut)
			}
		}
	})
}

func TestCliPushErrorExitsNonZero(t *testing.T) {
	empty := t.TempDir()
	code, _, errOut := runCli(t, empty, []string{"push"})
	if code == 0 {
		t.Fatal("Run(push) without Store: exit = 0, want non-zero")
	}
	if !strings.Contains(errOut, "mdots.toml not found") {
		t.Errorf("stderr should contain not-found error, got %q", errOut)
	}
}

// Seam: CLIコマンド境界 (init 雛形作成)
func TestCliInitDispatches(t *testing.T) {
	cwd := t.TempDir()
	code, out, _ := runCli(t, cwd, []string{"init"})
	if code != 0 {
		t.Fatalf("Run(init) exit = %d, want 0", code)
	}
	for _, want := range []string{"created", "mdots.toml"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got %q", want, out)
		}
	}
	data, err := os.ReadFile(filepath.Join(cwd, "mdots.toml"))
	if err != nil {
		t.Fatalf("mdots.toml should be created: %v", err)
	}
	for _, want := range []string{"default_target"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("init template should contain %q, got:\n%s", want, data)
		}
	}
}

func TestCliInitHelp(t *testing.T) {
	for _, args := range [][]string{{"init", "--help"}, {"init", "-h"}} {
		cwd := t.TempDir()
		code, out, _ := runCli(t, cwd, args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", args, code)
		}
		for _, want := range []string{"USAGE:", "OPTIONS:", "init", "mdots.toml"} {
			if !strings.Contains(out, want) {
				t.Errorf("Run(%v): output should contain %q, got %q", args, want, out)
			}
		}
		if _, err := os.Stat(filepath.Join(cwd, "mdots.toml")); err == nil {
			t.Errorf("Run(%v): mdots.toml must not be created on --help", args)
		}
	}
}

func TestCliInitRejectsExtraArgs(t *testing.T) {
	cwd := t.TempDir()
	if code, _, _ := runCli(t, cwd, []string{"init", "extra"}); code == 0 {
		t.Error("Run(init extra): exit = 0, want non-zero")
	}
	if _, err := os.Stat(filepath.Join(cwd, "mdots.toml")); err == nil {
		t.Error("mdots.toml must not be created with extra args")
	}
}

func TestCliInitAlreadyExists(t *testing.T) {
	store, _ := setupStoreWithHome(t, nil, nil, "default_target = \"base\"\n")
	code, _, errOut := runCli(t, store, []string{"init"})
	if code == 0 {
		t.Fatal("Run(init) on existing Store: exit = 0, want non-zero")
	}
	if !strings.Contains(errOut, "mdots.toml already exists") {
		t.Errorf("stderr should contain already exists, got %q", errOut)
	}
}

// Seam: CLIコマンド境界 (targets 一覧表示・既定印)
func TestCliTargetsDispatches(t *testing.T) {
	store, _ := setupStoreWithHome(t, nil, nil,
		"default_target = \"win\"\n"+
			"[targets.wsl.\"~/.c\"]\nsrc = \"c-wsl\"\n"+
			"[targets.win.\"~/.c\"]\nsrc = \"c-win\"\n"+
			"[targets.win.\"~/.b\"]\nsrc = \"b-win\"\n",
	)
	code, out, _ := runCli(t, store, []string{"targets"})
	if code != 0 {
		t.Fatalf("Run(targets) exit = %d, want 0", code)
	}
	if out != "win (default)\nwsl\n" {
		t.Errorf("output = %q, want %q", out, "win (default)\nwsl\n")
	}
}

func TestCliTargetsEmpty(t *testing.T) {
	store, _ := setupStoreWithHome(t, nil, nil,
		"default_target = \"base\"\n",
	)
	code, out, _ := runCli(t, store, []string{"targets"})
	if code != 0 {
		t.Fatalf("Run(targets) exit = %d, want 0", code)
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestCliTargetsRejectsExtraArgs(t *testing.T) {
	store, _ := setupStoreWithHome(t, nil, nil,
		"default_target = \"base\"\n",
	)
	code, _, errOut := runCli(t, store, []string{"targets", "extra"})
	if code == 0 {
		t.Error("Run(targets extra): exit = 0, want non-zero")
	}
	if !strings.Contains(errOut, "unknown argument: extra") {
		t.Errorf("stderr should contain unknown argument, got %q", errOut)
	}
}

func TestCliTargetsWithoutStore(t *testing.T) {
	empty := t.TempDir()
	code, _, errOut := runCli(t, empty, []string{"targets"})
	if code == 0 {
		t.Fatal("Run(targets) without Store: exit = 0, want non-zero")
	}
	if !strings.Contains(errOut, "mdots.toml not found") {
		t.Errorf("stderr should contain not-found error, got %q", errOut)
	}
}

// Seam: CLIコマンド境界 (add 登録・既定解決)
func TestCliAddDispatches(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "set number\n"},
		"default_target = \"base\"\n",
	)
	code, out, _ := runCli(t, store, []string{"add", "~/.vimrc"})
	if code != 0 {
		t.Fatalf("Run(add ~/.vimrc) exit = %d, want 0", code)
	}
	if !strings.Contains(out, "~/.vimrc") || !strings.Contains(out, "base") || !strings.Contains(out, "dotfiles/base/.vimrc") {
		t.Errorf("output should contain key, default target and src in one line, got %q", out)
	}
	if strings.Count(strings.TrimSuffix(out, "\n"), "\n") != 0 {
		t.Errorf("output should be one line, got %q", out)
	}
	body, err := os.ReadFile(filepath.Join(store, "mdots.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "~/.vimrc") {
		t.Errorf("mdots.toml should contain new Entry, got:\n%s", body)
	}
}

func TestCliAddHelp(t *testing.T) {
	for _, args := range [][]string{{"add", "--help"}, {"add", "-h"}} {
		store, _ := setupStoreWithHome(t,
			nil,
			map[string]string{".vimrc": "x\n"},
			"default_target = \"base\"\n",
		)
		code, out, _ := runCli(t, store, args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", args, code)
		}
		for _, want := range []string{"USAGE:", "add"} {
			if !strings.Contains(out, want) {
				t.Errorf("Run(%v): output should contain %q, got %q", args, want, out)
			}
		}
		body, err := os.ReadFile(filepath.Join(store, "mdots.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), ".vimrc") && strings.Contains(string(body), "dotfiles") {
			t.Errorf("Run(%v): Add must not run on --help", args)
		}
	}
}

func TestCliAddRejectsMissingAndExtraArgs(t *testing.T) {
	t.Run("引数なしは拒否", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil, "default_target = \"base\"\n")
		code, _, _ := runCli(t, store, []string{"add"})
		if code == 0 {
			t.Error("Run(add): exit = 0, want non-zero")
		}
	})
	t.Run("余剰は拒否", func(t *testing.T) {
		store, _ := setupStoreWithHome(t, nil, nil, "default_target = \"base\"\n")
		code, _, errOut := runCli(t, store, []string{"add", "~/.a", "~/.b"})
		if code == 0 {
			t.Error("Run(add a b): exit = 0, want non-zero")
		}
		if !strings.Contains(errOut, "unknown argument: ~/.b") {
			t.Errorf("stderr should contain unknown argument, got %q", errOut)
		}
	})
}

// Seam: CLIコマンド境界 (add --target 登録)
func TestCliAddWithTargetDispatches(t *testing.T) {
	for _, args := range [][]string{
		{"add", "--target", "win", "~/.vimrc"},
		{"add", "--target=win", "~/.vimrc"},
		{"add", "-t", "win", "~/.vimrc"},
	} {
		store, _ := setupStoreWithHome(t,
			nil,
			map[string]string{".vimrc": "set number\n"},
			"default_target = \"base\"\n",
		)
		code, out, _ := runCli(t, store, args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", args, code)
		}
		if !strings.Contains(out, "~/.vimrc") || !strings.Contains(out, "win") || !strings.Contains(out, "dotfiles/win/.vimrc") {
			t.Errorf("Run(%v): output should contain key, target and src, got %q", args, out)
		}
		if strings.Count(strings.TrimSuffix(out, "\n"), "\n") != 0 {
			t.Errorf("Run(%v): output should be one line, got %q", args, out)
		}
	}
}

func TestCliAddRejectsEmptyTarget(t *testing.T) {
	store, _ := setupStoreWithHome(t, nil, nil, "default_target = \"base\"\n")
	code, _, _ := runCli(t, store, []string{"add", "--target=", "~/.vimrc"})
	if code == 0 {
		t.Error("Run(add --target=): exit = 0, want non-zero")
	}
}

func TestCliAddRejectsFlags(t *testing.T) {
	for _, args := range [][]string{
		{"add", "--dry-run", "~/.vimrc"},
		{"add", "--color=always", "~/.vimrc"},
		{"add", "--unknown", "~/.vimrc"},
		{"add", "~/.vimrc", "--unknown"},
	} {
		store, _ := setupStoreWithHome(t, nil, nil, "default_target = \"base\"\n")
		code, _, _ := runCli(t, store, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
	}
}

func TestCliAddDuplicateError(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		nil,
		map[string]string{".vimrc": "x\n"},
		"default_target = \"base\"\n[targets.base.\"~/.vimrc\"]\nsrc = \"dotfiles/base/.vimrc\"\n",
	)
	code, _, errOut := runCli(t, store, []string{"add", "~/.vimrc"})
	if code == 0 {
		t.Fatal("Run(add duplicate): exit = 0, want non-zero")
	}
	if !strings.Contains(errOut, "already registered") {
		t.Errorf("stderr should contain already registered, got %q", errOut)
	}
}
