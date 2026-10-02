package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: CLIコマンド境界 (push/pull --interactive 専用モード)

func runCliInteractive(t *testing.T, cwd string, args []string, stdinBody string, tty bool) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := RunWithStdin(args, cwd, "v0.0.0-test", &out, &errOut, strings.NewReader(stdinBody), func() bool { return tty })
	return code, out.String(), errOut.String()
}

func TestInteractiveRequiresTTY(t *testing.T) {
	for _, args := range [][]string{{"push", "-i"}, {"pull", "--interactive"}} {
		store, _ := setupStoreWithHome(t,
			map[string]string{"vimrc": "same\n"},
			map[string]string{".vimrc": "same\n"},
			singleBaseToml(),
		)
		code, _, errOut := runCliInteractive(t, store, args, "", false)
		if code == 0 {
			t.Fatalf("Run(%v) non-tty: exit = 0, want non-zero", args)
		}
		if !strings.Contains(errOut, "interactive requires a terminal") {
			t.Errorf("Run(%v) non-tty: stderr should contain terminal error, got %q", args, errOut)
		}
	}
}

func TestInteractiveSelectsTargetAndExecutes(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"base.conf": "base\n", "win.conf": "win\n"},
		nil,
		baseWinToml(),
	)
	// 2=win を選び、実行(N)する
	code, out, errOut := runCliInteractive(t, store, []string{"push", "-i"}, "2\nN\n", true)
	if code != 0 {
		t.Fatalf("Run(push -i) exit = %d, want 0 (stderr=%q out=%q)", code, errOut, out)
	}
	if !strings.Contains(out, "2) win") {
		t.Errorf("stdout should list targets like targets command, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".win.conf")); err != nil {
		t.Errorf("win should be copied, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".base.conf")); err == nil {
		t.Errorf("base should NOT be copied when win selected")
	}
}

func TestInteractiveDryRunDoesNotWrite(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"base.conf": "same\n", "win.conf": "new\n"},
		map[string]string{".base.conf": "same\n", ".win.conf": "old\n"},
		baseWinToml(),
	)
	// 2=win を選び、dry-run(y)する。差分ありなので exit 1。
	code, out, _ := runCliInteractive(t, store, []string{"push", "-i"}, "2\ny\n", true)
	if code == 0 {
		t.Fatalf("Run(push -i dry-run with diff) exit = 0, want non-zero (out=%q)", out)
	}
	if !strings.Contains(out, "win.conf") {
		t.Errorf("dry-run output should contain win diff, got %q", out)
	}
	if got, _ := os.ReadFile(filepath.Join(home, ".win.conf")); string(got) != "old\n" {
		t.Errorf("dry-run must not write dest, got %q", string(got))
	}
}

func TestInteractiveIgnoresOtherOptions(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"base.conf": "base\n", "win.conf": "win\n"},
		nil,
		baseWinToml(),
	)
	// --target win / --dry-run / --color併記でも無視し、対話の 1=base 実行(N)を優先する
	code, _, errOut := runCliInteractive(t, store,
		[]string{"push", "-i", "--target", "win", "--dry-run", "--color", "always"},
		"1\nN\n", true)
	if code != 0 {
		t.Fatalf("Run(push -i with other flags) exit = %d, want 0 (stderr=%q)", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(home, ".base.conf")); err != nil {
		t.Errorf("base should be copied (interactive answer wins), got %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".win.conf")); err == nil {
		t.Errorf("win must NOT be copied when interactive selects base")
	}
}

func TestInteractiveInvalidSelection(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"vimrc": "same\n"},
		map[string]string{".vimrc": "same\n"},
		singleBaseToml(),
	)
	code, _, _ := runCliInteractive(t, store, []string{"pull", "-i"}, "9\n", true)
	if code == 0 {
		t.Fatal("Run(pull -i invalid number): exit = 0, want non-zero")
	}
}

func TestInteractiveEmptySelectsDefault(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{"base.conf": "base\n", "win.conf": "win\n"},
		nil,
		baseWinToml(),
	)
	// 空入力は (default) 印の base へ解決し、実行する
	code, _, errOut := runCliInteractive(t, store, []string{"push", "--interactive"}, "\nN\n", true)
	if code != 0 {
		t.Fatalf("Run(push -i empty) exit = %d, want 0 (stderr=%q)", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(home, ".base.conf")); err != nil {
		t.Errorf("empty input should select default base, got %v", err)
	}
}

func TestInteractivePullExecutes(t *testing.T) {
	storeFiles := map[string]string{"base.conf": "old\n", "win.conf": "old\n"}
	homeFiles := map[string]string{".base.conf": "same-base\n", ".win.conf": "new-win\n"}
	store, _ := setupStoreWithHome(t, storeFiles, homeFiles, baseWinToml())
	// pull -i で win を選び実行すると Store側 win.conf が new-win になる
	code, _, errOut := runCliInteractive(t, store, []string{"pull", "-i"}, "2\nN\n", true)
	if code != 0 {
		t.Fatalf("Run(pull -i) exit = %d, want 0 (stderr=%q)", code, errOut)
	}
	if got, _ := os.ReadFile(filepath.Join(store, "win.conf")); string(got) != "new-win\n" {
		t.Errorf("pull interactive should collect win, got %q", string(got))
	}
}
