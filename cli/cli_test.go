package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// Seam: CLIコマンド境界 (新CLIパッケージ: コマンド定義・Writer注入・exit)
// 実行系はfake Executorに差し替え、外部挙動（exit codeと出力文面）のみを検証する。

type fakeExecutor struct {
	pushTarget string
	pushCalls  int
	pushErr    error

	pullTarget string
	pullCalls  int
	pullErr    error

	pushDryRunCode   int
	pushDryRunOut    string
	pushDryRunCalls  int
	pushDryRunTarget string
	pushDryRunColor  string

	pullDryRunCode   int
	pullDryRunOut    string
	pullDryRunCalls  int
	pullDryRunTarget string
	pullDryRunColor  string

	initCalls int
	initErr   error

	targetsCalls int
	targetsOut   []string
	targetsErr   error
}

func (f *fakeExecutor) Push(cwd, target string) error {
	f.pushCalls++
	f.pushTarget = target
	return f.pushErr
}

func (f *fakeExecutor) Pull(cwd, target string) error {
	f.pullCalls++
	f.pullTarget = target
	return f.pullErr
}

func (f *fakeExecutor) PushDryRun(cwd, target, color string, stdout, stderr io.Writer) int {
	f.pushDryRunCalls++
	f.pushDryRunTarget = target
	f.pushDryRunColor = color
	io.WriteString(stdout, f.pushDryRunOut)
	return f.pushDryRunCode
}

func (f *fakeExecutor) PullDryRun(cwd, target, color string, stdout, stderr io.Writer) int {
	f.pullDryRunCalls++
	f.pullDryRunTarget = target
	f.pullDryRunColor = color
	io.WriteString(stdout, f.pullDryRunOut)
	return f.pullDryRunCode
}

func (f *fakeExecutor) Init(cwd string) error {
	f.initCalls++
	return f.initErr
}

func (f *fakeExecutor) Targets(cwd string) ([]string, error) {
	f.targetsCalls++
	return f.targetsOut, f.targetsErr
}

func runCli(t *testing.T, ex Executor, args []string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, t.TempDir(), "v0.0.0-test", ex, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestCliGlobalHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}} {
		ex := &fakeExecutor{}
		code, out, _ := runCli(t, ex, args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", args, code)
		}
		for _, want := range []string{"USAGE:", "COMMANDS:", "GLOBAL OPTIONS:", "push", "pull", "init"} {
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
	ex := &fakeExecutor{}
	code, out, errOut := runCli(t, ex, nil)
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
		ex := &fakeExecutor{}
		code, out, _ := runCli(t, ex, args)
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
		ex := &fakeExecutor{}
		code, out, errOut := runCli(t, ex, []string{"-V"})
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
		ex := &fakeExecutor{}
		code, out, errOut := runCli(t, ex, []string{"version"})
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
	ex := &fakeExecutor{}
	code, _, errOut := runCli(t, ex, []string{"frobnicate"})
	if code != 3 {
		t.Fatalf("Run(unknown) exit = %d, want 3", code)
	}
	if !strings.Contains(errOut, "No help topic for 'frobnicate'") {
		t.Errorf("stderr should contain No help topic for 'frobnicate', got %q", errOut)
	}
}

func TestCliDiffIsRemoved(t *testing.T) {
	for _, args := range [][]string{{"diff"}, {"diff", "--target", "win"}} {
		ex := &fakeExecutor{}
		code, _, _ := runCli(t, ex, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
		if ex.pushCalls+ex.pullCalls+ex.pushDryRunCalls+ex.pullDryRunCalls+ex.initCalls != 0 {
			t.Errorf("Run(%v): executor must not run", args)
		}
	}
}

func TestCliDryRunDispatchesTarget(t *testing.T) {
	t.Run("push", func(t *testing.T) {
		tests := []struct {
			name string
			args []string
			want string
		}{
			{"no target", []string{"push", "--dry-run"}, ""},
			{"space form", []string{"push", "--dry-run", "--target", "win"}, "win"},
			{"equals form", []string{"push", "--dry-run", "--target=win"}, "win"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ex := &fakeExecutor{}
				code, _, _ := runCli(t, ex, tt.args)
				if code != 0 {
					t.Fatalf("Run(%v) exit = %d, want 0", tt.args, code)
				}
				if ex.pushDryRunCalls != 1 {
					t.Fatalf("PushDryRun calls = %d, want 1", ex.pushDryRunCalls)
				}
				if ex.pushDryRunTarget != tt.want {
					t.Errorf("PushDryRun target = %q, want %q", ex.pushDryRunTarget, tt.want)
				}
				if ex.pushCalls != 0 {
					t.Errorf("Push must not run on dry-run")
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
			{"no target", []string{"pull", "--dry-run"}, ""},
			{"space form", []string{"pull", "--dry-run", "--target", "win"}, "win"},
			{"equals form", []string{"pull", "--dry-run", "--target=win"}, "win"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ex := &fakeExecutor{}
				code, _, _ := runCli(t, ex, tt.args)
				if code != 0 {
					t.Fatalf("Run(%v) exit = %d, want 0", tt.args, code)
				}
				if ex.pullDryRunCalls != 1 {
					t.Fatalf("PullDryRun calls = %d, want 1", ex.pullDryRunCalls)
				}
				if ex.pullDryRunTarget != tt.want {
					t.Errorf("PullDryRun target = %q, want %q", ex.pullDryRunTarget, tt.want)
				}
				if ex.pullCalls != 0 {
					t.Errorf("Pull must not run on dry-run")
				}
			})
		}
	})
}

func TestCliColorRequiresDryRun(t *testing.T) {
	for _, args := range [][]string{
		{"push", "--color=always"},
		{"pull", "--color=always"},
	} {
		ex := &fakeExecutor{}
		code, _, errOut := runCli(t, ex, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
		if !strings.Contains(errOut, "--color requires --dry-run") {
			t.Errorf("Run(%v): stderr should contain --color requires --dry-run, got %q", args, errOut)
		}
		if ex.pushCalls+ex.pullCalls+ex.pushDryRunCalls+ex.pullDryRunCalls+ex.initCalls != 0 {
			t.Errorf("Run(%v): executor must not run", args)
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
		ex := &fakeExecutor{}
		code, out, errOut := runCli(t, ex, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
		if !strings.Contains(errOut, "Incorrect Usage") {
			t.Errorf("Run(%v): stderr should contain Incorrect Usage, got %q", args, errOut)
		}
		if !strings.Contains(out, "USAGE:") {
			t.Errorf("Run(%v): stdout should contain USAGE:, got %q", args, out)
		}
		if ex.pushCalls+ex.pullCalls+ex.pushDryRunCalls+ex.pullDryRunCalls+ex.initCalls != 0 {
			t.Errorf("Run(%v): executor must not run", args)
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
		ex := &fakeExecutor{}
		code, _, _ := runCli(t, ex, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
		if ex.pushCalls+ex.pullCalls+ex.pushDryRunCalls+ex.pullDryRunCalls+ex.initCalls != 0 {
			t.Errorf("Run(%v): executor must not run", args)
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
		ex := &fakeExecutor{}
		code, out, _ := runCli(t, ex, tt.args)
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
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no target", []string{"push"}, ""},
		{"space form", []string{"push", "--target", "win"}, "win"},
		{"equals form", []string{"push", "--target=win"}, "win"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := &fakeExecutor{}
			code, _, _ := runCli(t, ex, tt.args)
			if code != 0 {
				t.Fatalf("Run(%v) exit = %d, want 0", tt.args, code)
			}
			if ex.pushCalls != 1 {
				t.Fatalf("Push calls = %d, want 1", ex.pushCalls)
			}
			if ex.pushTarget != tt.want {
				t.Errorf("Push target = %q, want %q", ex.pushTarget, tt.want)
			}
		})
	}
}

func TestCliShortFlags(t *testing.T) {
	t.Run("push -t は --target と同じ", func(t *testing.T) {
		ex := &fakeExecutor{}
		code, _, _ := runCli(t, ex, []string{"push", "-t", "win"})
		if code != 0 {
			t.Fatalf("Run(push -t win) exit = %d, want 0", code)
		}
		if ex.pushCalls != 1 || ex.pushTarget != "win" {
			t.Errorf("Push calls = %d target = %q, want 1/win", ex.pushCalls, ex.pushTarget)
		}
	})
	t.Run("pull -t は --target と同じ", func(t *testing.T) {
		ex := &fakeExecutor{}
		code, _, _ := runCli(t, ex, []string{"pull", "-t", "win"})
		if code != 0 {
			t.Fatalf("Run(pull -t win) exit = %d, want 0", code)
		}
		if ex.pullCalls != 1 || ex.pullTarget != "win" {
			t.Errorf("Pull calls = %d target = %q, want 1/win", ex.pullCalls, ex.pullTarget)
		}
	})
	t.Run("push -n は --dry-run と同じ", func(t *testing.T) {
		ex := &fakeExecutor{}
		code, _, _ := runCli(t, ex, []string{"push", "-n"})
		if code != 0 {
			t.Fatalf("Run(push -n) exit = %d, want 0", code)
		}
		if ex.pushDryRunCalls != 1 || ex.pushCalls != 0 {
			t.Errorf("must delegate to PushDryRun only")
		}
	})
	t.Run("pull -n -t の併用", func(t *testing.T) {
		ex := &fakeExecutor{}
		code, _, _ := runCli(t, ex, []string{"pull", "-n", "-t", "win"})
		if code != 0 {
			t.Fatalf("Run(pull -n -t win) exit = %d, want 0", code)
		}
		if ex.pullDryRunCalls != 1 || ex.pullDryRunTarget != "win" {
			t.Errorf("PullDryRun calls = %d target = %q, want 1/win", ex.pullDryRunCalls, ex.pullDryRunTarget)
		}
	})
	t.Run("push -n -c は --color と同じ", func(t *testing.T) {
		ex := &fakeExecutor{}
		code, _, _ := runCli(t, ex, []string{"push", "-n", "-c", "always"})
		if code != 0 {
			t.Fatalf("Run(push -n -c always) exit = %d, want 0", code)
		}
		if ex.pushDryRunCalls != 1 || ex.pushDryRunColor != "always" {
			t.Errorf("PushDryRun calls = %d color = %q, want 1/always", ex.pushDryRunCalls, ex.pushDryRunColor)
		}
	})
	t.Run("push -c 単独は --dry-run 必須エラー", func(t *testing.T) {
		ex := &fakeExecutor{}
		code, _, _ := runCli(t, ex, []string{"push", "-c", "always"})
		if code == 0 {
			t.Fatal("Run(push -c always): exit = 0, want non-zero")
		}
		if ex.pushCalls+ex.pullCalls+ex.pushDryRunCalls+ex.pullDryRunCalls != 0 {
			t.Error("executor must not run")
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
		ex := &fakeExecutor{}
		if code, _, _ := runCli(t, ex, args); code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
	}
}

func TestCliDryRunDelegatesExitCode(t *testing.T) {
	t.Run("push", func(t *testing.T) {
		ex := &fakeExecutor{pushDryRunCode: 1, pushDryRunOut: "--- a\n+++ b\n"}
		code, out, _ := runCli(t, ex, []string{"push", "--dry-run"})
		if code != 1 {
			t.Fatalf("Run(push --dry-run) exit = %d, want 1", code)
		}
		if ex.pushDryRunCalls != 1 || ex.pushCalls+ex.pullCalls+ex.pullDryRunCalls != 0 {
			t.Errorf("must delegate to PushDryRun only")
		}
		if !strings.Contains(out, "---") {
			t.Errorf("output should contain diff, got %q", out)
		}

		ex = &fakeExecutor{pushDryRunCode: 0}
		if code, _, _ := runCli(t, ex, []string{"push", "--dry-run"}); code != 0 {
			t.Errorf("Run(push --dry-run) without changes: exit = %d, want 0", code)
		}
	})
	t.Run("pull", func(t *testing.T) {
		ex := &fakeExecutor{pullDryRunCode: 1, pullDryRunOut: "--- a\n+++ b\n"}
		code, out, _ := runCli(t, ex, []string{"pull", "--dry-run"})
		if code != 1 {
			t.Fatalf("Run(pull --dry-run) exit = %d, want 1", code)
		}
		if ex.pullDryRunCalls != 1 || ex.pushCalls+ex.pullCalls+ex.pushDryRunCalls != 0 {
			t.Errorf("must delegate to PullDryRun only")
		}
		if !strings.Contains(out, "---") {
			t.Errorf("output should contain diff, got %q", out)
		}

		ex = &fakeExecutor{pullDryRunCode: 0}
		if code, _, _ := runCli(t, ex, []string{"pull", "--dry-run"}); code != 0 {
			t.Errorf("Run(pull --dry-run) without changes: exit = %d, want 0", code)
		}
	})
}

func TestCliDryRunColorFlag(t *testing.T) {
	t.Run("既定はauto", func(t *testing.T) {
		for _, args := range [][]string{{"push", "--dry-run"}, {"pull", "--dry-run"}} {
			ex := &fakeExecutor{}
			if code, _, _ := runCli(t, ex, args); code != 0 {
				t.Fatalf("Run(%v) exit = %d, want 0", args, code)
			}
			var got string
			if args[0] == "push" {
				got = ex.pushDryRunColor
			} else {
				got = ex.pullDryRunColor
			}
			if got != "auto" {
				t.Errorf("Run(%v) default color = %q, want auto", args, got)
			}
		}
	})

	t.Run("always/neverを通す", func(t *testing.T) {
		for _, c := range []string{"always", "never"} {
			for _, base := range []string{"push", "pull"} {
				ex := &fakeExecutor{}
				args := []string{base, "--dry-run", "--color=" + c}
				if code, _, _ := runCli(t, ex, args); code != 0 {
					t.Errorf("Run(%v) exit = %d, want 0", args, code)
				}
				var got string
				if base == "push" {
					got = ex.pushDryRunColor
				} else {
					got = ex.pullDryRunColor
				}
				if got != c {
					t.Errorf("Run(%v) color = %q, want %q", args, got, c)
				}
			}
		}
	})

	t.Run("不正値はexit1", func(t *testing.T) {
		for _, base := range []string{"push", "pull"} {
			ex := &fakeExecutor{}
			args := []string{base, "--dry-run", "--color=foo"}
			if code, _, errOut := runCli(t, ex, args); code == 0 {
				t.Error("exit = 0, want non-zero")
			} else if !strings.Contains(errOut, "invalid value for --color") {
				t.Errorf("stderr should contain invalid value, got %q", errOut)
			}
			if ex.pushDryRunCalls+ex.pullDryRunCalls != 0 {
				t.Error("executor must not run on invalid color")
			}
		}
	})
}

func TestCliExecutorErrorExitsNonZero(t *testing.T) {
	ex := &fakeExecutor{pushErr: errors.New("boom")}
	code, _, errOut := runCli(t, ex, []string{"push"})
	if code == 0 {
		t.Fatal("Run(push) with executor error: exit = 0, want non-zero")
	}
	if !strings.Contains(errOut, "boom") {
		t.Errorf("stderr should contain executor error, got %q", errOut)
	}
}

// Seam: CLIコマンド境界 (init 雛形作成)
// 成功・help・余分引数・実行エラーの外部挙動のみを検証する。
func TestCliInitDispatches(t *testing.T) {
	ex := &fakeExecutor{}
	code, out, _ := runCli(t, ex, []string{"init"})
	if code != 0 {
		t.Fatalf("Run(init) exit = %d, want 0", code)
	}
	if ex.initCalls != 1 {
		t.Fatalf("Init calls = %d, want 1", ex.initCalls)
	}
	for _, want := range []string{"created", "mdots.toml"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got %q", want, out)
		}
	}
}

func TestCliInitHelp(t *testing.T) {
	for _, args := range [][]string{{"init", "--help"}, {"init", "-h"}} {
		ex := &fakeExecutor{}
		code, out, _ := runCli(t, ex, args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", args, code)
		}
		for _, want := range []string{"USAGE:", "OPTIONS:", "init", "mdots.toml"} {
			if !strings.Contains(out, want) {
				t.Errorf("Run(%v): output should contain %q, got %q", args, want, out)
			}
		}
		if ex.initCalls != 0 {
			t.Errorf("Run(%v): Init must not run on --help", args)
		}
	}
}

func TestCliInitRejectsExtraArgs(t *testing.T) {
	ex := &fakeExecutor{}
	if code, _, _ := runCli(t, ex, []string{"init", "extra"}); code == 0 {
		t.Error("Run(init extra): exit = 0, want non-zero")
	}
	if ex.initCalls != 0 {
		t.Error("Init must not run with extra args")
	}
}

func TestCliInitExecutorError(t *testing.T) {
	ex := &fakeExecutor{initErr: errors.New("mdots.toml already exists in /tmp/x")}
	code, _, errOut := runCli(t, ex, []string{"init"})
	if code == 0 {
		t.Fatal("Run(init) with executor error: exit = 0, want non-zero")
	}
	if !strings.Contains(errOut, "mdots.toml already exists") {
		t.Errorf("stderr should contain already exists, got %q", errOut)
	}
}

// Seam: CLIコマンド境界 (targets 一覧表示)
// 定義済みTarget名の一覧表示の外部挙動のみを検証する。
func TestCliTargetsDispatches(t *testing.T) {
	ex := &fakeExecutor{targetsOut: []string{"win", "wsl"}}
	code, out, _ := runCli(t, ex, []string{"targets"})
	if code != 0 {
		t.Fatalf("Run(targets) exit = %d, want 0", code)
	}
	if ex.targetsCalls != 1 {
		t.Fatalf("Targets calls = %d, want 1", ex.targetsCalls)
	}
	if out != "win\nwsl\n" {
		t.Errorf("output = %q, want %q", out, "win\nwsl\n")
	}
}

func TestCliTargetsEmpty(t *testing.T) {
	ex := &fakeExecutor{}
	code, out, _ := runCli(t, ex, []string{"targets"})
	if code != 0 {
		t.Fatalf("Run(targets) exit = %d, want 0", code)
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestCliTargetsRejectsExtraArgs(t *testing.T) {
	ex := &fakeExecutor{}
	code, _, errOut := runCli(t, ex, []string{"targets", "extra"})
	if code == 0 {
		t.Error("Run(targets extra): exit = 0, want non-zero")
	}
	if !strings.Contains(errOut, "unknown argument: extra") {
		t.Errorf("stderr should contain unknown argument, got %q", errOut)
	}
	if ex.targetsCalls != 0 {
		t.Error("Targets must not run with extra args")
	}
}

func TestCliTargetsExecutorError(t *testing.T) {
	ex := &fakeExecutor{targetsErr: errors.New("mdots.toml not found in /tmp/x")}
	code, _, errOut := runCli(t, ex, []string{"targets"})
	if code == 0 {
		t.Fatal("Run(targets) with executor error: exit = 0, want non-zero")
	}
	if !strings.Contains(errOut, "mdots.toml not found") {
		t.Errorf("stderr should contain executor error, got %q", errOut)
	}
}
