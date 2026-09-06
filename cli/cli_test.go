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

	diffCode   int
	diffOut    string
	diffCalls  int
	diffTarget string
	diffColor  string

	initCalls int
	initErr   error
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

func (f *fakeExecutor) Diff(cwd, target, color string, stdout, stderr io.Writer) int {
	f.diffCalls++
	f.diffTarget = target
	f.diffColor = color
	io.WriteString(stdout, f.diffOut)
	return f.diffCode
}

func (f *fakeExecutor) Init(cwd string) error {
	f.initCalls++
	return f.initErr
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
		for _, want := range []string{"USAGE:", "COMMANDS:", "GLOBAL OPTIONS:", "push", "pull", "diff", "init"} {
			if !strings.Contains(out, want) {
				t.Errorf("Run(%v): output should contain %q, got %q", args, want, out)
			}
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

func TestCliDiffDispatchesTarget(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no target", []string{"diff"}, ""},
		{"space form", []string{"diff", "--target", "win"}, "win"},
		{"equals form", []string{"diff", "--target=win"}, "win"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := &fakeExecutor{}
			code, _, _ := runCli(t, ex, tt.args)
			if code != 0 {
				t.Fatalf("Run(%v) exit = %d, want 0", tt.args, code)
			}
			if ex.diffCalls != 1 {
				t.Fatalf("Diff calls = %d, want 1", ex.diffCalls)
			}
			if ex.diffTarget != tt.want {
				t.Errorf("Diff target = %q, want %q", ex.diffTarget, tt.want)
			}
		})
	}
}

func TestCliDryRunFlagsRemoved(t *testing.T) {
	for _, args := range [][]string{
		{"push", "--dry-run"},
		{"pull", "--dry-run"},
		{"push", "--color=always"},
		{"pull", "--color=always"},
	} {
		ex := &fakeExecutor{}
		code, _, _ := runCli(t, ex, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
		if ex.pushCalls+ex.pullCalls+ex.initCalls+ex.diffCalls != 0 {
			t.Errorf("Run(%v): executor must not run", args)
		}
	}
}

func TestCliStandardUsageError(t *testing.T) {
	for _, args := range [][]string{
		{"push", "--target"},
		{"pull", "--target"},
		{"diff", "--target"},
		{"push", "--unknown"},
		{"pull", "--unknown"},
		{"diff", "--unknown"},
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
		if ex.pushCalls+ex.pullCalls+ex.initCalls+ex.diffCalls != 0 {
			t.Errorf("Run(%v): executor must not run", args)
		}
	}
}

func TestCliEmptyAndExtraArgsStillRejected(t *testing.T) {
	for _, args := range [][]string{
		{"push", "--target="},
		{"pull", "--target="},
		{"diff", "--target="},
		{"push", "extra-positional"},
		{"pull", "extra-positional"},
		{"diff", "extra-positional"},
		{"init", "extra"},
		{"push", "--", "--target", "win"},
		{"pull", "--", "--target", "win"},
		{"diff", "--", "--target", "win"},
	} {
		ex := &fakeExecutor{}
		code, _, _ := runCli(t, ex, args)
		if code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
		if ex.pushCalls+ex.pullCalls+ex.initCalls+ex.diffCalls != 0 {
			t.Errorf("Run(%v): executor must not run", args)
		}
	}
}

func TestCliCommandHelp(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"push", "--help"}, []string{"USAGE:", "OPTIONS:", "push", "--target"}},
		{[]string{"push", "-h"}, []string{"USAGE:", "OPTIONS:", "push", "--target"}},
		{[]string{"pull", "--help"}, []string{"USAGE:", "OPTIONS:", "pull", "--target"}},
		{[]string{"pull", "-h"}, []string{"USAGE:", "OPTIONS:", "pull", "--target"}},
		{[]string{"diff", "--help"}, []string{"USAGE:", "OPTIONS:", "diff", "--target", "--color"}},
		{[]string{"diff", "-h"}, []string{"USAGE:", "OPTIONS:", "diff", "--target", "--color"}},
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

func TestCliPushPullHelpHasNoDryRun(t *testing.T) {
	for _, args := range [][]string{{"push", "--help"}, {"pull", "--help"}} {
		ex := &fakeExecutor{}
		code, out, _ := runCli(t, ex, args)
		if code != 0 {
			t.Fatalf("Run(%v) exit = %d, want 0", args, code)
		}
		if strings.Contains(out, "--dry-run") || strings.Contains(out, "--color") {
			t.Errorf("Run(%v): must not contain dry-run/color, got %q", args, out)
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

func TestCliTargetFlagErrors(t *testing.T) {
	for _, args := range [][]string{
		{"push", "--target"},
		{"push", "--target="},
		{"push", "--unknown"},
		{"push", "extra-positional"},
		{"pull", "--target"},
		{"pull", "--target="},
		{"pull", "--unknown"},
		{"diff", "--target"},
		{"diff", "--target="},
		{"diff", "--unknown"},
		{"diff", "extra-positional"},
	} {
		ex := &fakeExecutor{}
		if code, _, _ := runCli(t, ex, args); code == 0 {
			t.Errorf("Run(%v): exit = 0, want non-zero", args)
		}
	}
}

func TestCliDiffDelegatesExitCode(t *testing.T) {
	ex := &fakeExecutor{diffCode: 1, diffOut: "--- a\n+++ b\n"}
	code, out, _ := runCli(t, ex, []string{"diff"})
	if code != 1 {
		t.Fatalf("Run(diff) exit = %d, want 1", code)
	}
	if ex.diffCalls != 1 || ex.pushCalls+ex.pullCalls != 0 {
		t.Errorf("diff must delegate to Diff only (diff=%d push=%d pull=%d)", ex.diffCalls, ex.pushCalls, ex.pullCalls)
	}
	if !strings.Contains(out, "---") {
		t.Errorf("output should contain diff, got %q", out)
	}

	ex = &fakeExecutor{diffCode: 0}
	if code, _, _ := runCli(t, ex, []string{"diff"}); code != 0 {
		t.Errorf("Run(diff) without changes: exit = %d, want 0", code)
	}
}

func TestCliDiffColorFlag(t *testing.T) {
	t.Run("既定はauto", func(t *testing.T) {
		ex2 := &recordingExecutor{}
		if code, _, _ := runCli(t, ex2, []string{"diff"}); code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if ex2.diffColor != "auto" {
			t.Errorf("default color = %q, want auto", ex2.diffColor)
		}
	})

	t.Run("always/neverを通す", func(t *testing.T) {
		for _, c := range []string{"always", "never"} {
			ex := &recordingExecutor{}
			if code, _, _ := runCli(t, ex, []string{"diff", "--color=" + c}); code != 0 {
				t.Errorf("Run(diff --color=%s) exit = %d, want 0", c, code)
			}
			if ex.diffColor != c {
				t.Errorf("color = %q, want %q", ex.diffColor, c)
			}
		}
	})

	t.Run("不正値はexit1", func(t *testing.T) {
		ex := &fakeExecutor{}
		if code, _, errOut := runCli(t, ex, []string{"diff", "--color=foo"}); code == 0 {
			t.Error("exit = 0, want non-zero")
		} else if !strings.Contains(errOut, "invalid value for --color") {
			t.Errorf("stderr should contain invalid value, got %q", errOut)
		}
		if ex.diffCalls != 0 {
			t.Error("executor must not run on invalid color")
		}
	})
}

type recordingExecutor struct {
	fakeExecutor
	diffColor string
}

func (f *recordingExecutor) Diff(cwd, target, color string, stdout, stderr io.Writer) int {
	f.diffColor = color
	return f.fakeExecutor.Diff(cwd, target, color, stdout, stderr)
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
