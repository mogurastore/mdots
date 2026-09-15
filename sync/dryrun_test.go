package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mogurastore/mdots/config"
)

// Seam: sync パッケージ公開境界 (push/pull dry-run)
// RED: PushDryRun / PullDryRun は未実装のため失敗する。
func TestPushDryRunShowsDestToStore(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := PushDryRun(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("PushDryRun error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if !strings.Contains(out, "--- "+dest+"\n") || !strings.Contains(out, "+++ a\n") {
		t.Errorf("push dry-run must be dest->Store headers, got %q", out)
	}
	if !strings.Contains(out, "+ new") {
		t.Errorf("Store-side addition must render as +, got %q", out)
	}
}

func TestPullDryRunShowsStoreToDest(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "old\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "new\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := PullDryRun(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("PullDryRun error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if !strings.Contains(out, "--- a\n") || !strings.Contains(out, "+++ "+dest+"\n") {
		t.Errorf("pull dry-run must be Store->dest headers, got %q", out)
	}
	if !strings.Contains(out, "+ new") {
		t.Errorf("dest-side addition must render as +, got %q", out)
	}
}

func TestDryRunReportsMissingEitherSideAsNew(t *testing.T) {
	t.Run("push dest不在は新規予定・書き込まない", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		writeTestFile(t, filepath.Join(store, "a"), "new\n")
		dest := filepath.Join(destRoot, "a")
		entries := []config.Entry{{Src: "a", Dest: dest}}

		out, hasDiff, err := PushDryRun(store, entries, ColorNever)
		if err != nil {
			t.Fatalf("PushDryRun error: %v", err)
		}
		if !hasDiff || out == "" {
			t.Fatal("want new-file notice, got empty")
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Errorf("dest must NOT be created: stat err = %v", err)
		}
	})

	t.Run("pull Store不在は新規回収予定・書き込まない", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "new\n")
		entries := []config.Entry{{Src: "a", Dest: dest}}

		out, hasDiff, err := PullDryRun(store, entries, ColorNever)
		if err != nil {
			t.Fatalf("PullDryRun error: %v", err)
		}
		if !hasDiff || out == "" {
			t.Fatal("want new-file notice, got empty")
		}
		if _, err := os.Stat(filepath.Join(store, "a")); !os.IsNotExist(err) {
			t.Errorf("Store src must NOT be created: stat err = %v", err)
		}
	})

	t.Run("両方不在はエラー", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		entries := []config.Entry{{Src: "a", Dest: filepath.Join(destRoot, "a")}}
		if _, _, err := PushDryRun(store, entries, ColorNever); err == nil {
			t.Error("push: エラー expected, got nil")
		}
		if _, _, err := PullDryRun(store, entries, ColorNever); err == nil {
			t.Error("pull: エラー expected, got nil")
		}
	})

	t.Run("push Store不在はエラー", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "exists\n")
		entries := []config.Entry{{Src: "a", Dest: dest}}
		if _, _, err := PushDryRun(store, entries, ColorNever); err == nil {
			t.Error("push srcMissing: エラー expected, got nil")
		}
	})

	t.Run("pull dest不在はエラー", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		writeTestFile(t, filepath.Join(store, "a"), "exists\n")
		entries := []config.Entry{{Src: "a", Dest: filepath.Join(destRoot, "a")}}
		if _, _, err := PullDryRun(store, entries, ColorNever); err == nil {
			t.Error("pull destMissing: エラー expected, got nil")
		}
	})
}

func TestDryRunNoChange(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "same\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "same\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	if out, hasDiff, err := PushDryRun(store, entries, ColorNever); err != nil {
		t.Fatalf("PushDryRun error: %v", err)
	} else if hasDiff {
		t.Errorf("push hasDiff = true, want false (out=%q)", out)
	}
	if out, hasDiff, err := PullDryRun(store, entries, ColorNever); err != nil {
		t.Fatalf("PullDryRun error: %v", err)
	} else if hasDiff {
		t.Errorf("pull hasDiff = true, want false (out=%q)", out)
	}
}

func TestDryRunDoesNotWrite(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	if _, _, err := PushDryRun(store, entries, ColorNever); err != nil {
		t.Fatalf("PushDryRun error: %v", err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "old\n" {
		t.Errorf("push dry-run must NOT write dest: content = %q", got)
	}
	if _, _, err := PullDryRun(store, entries, ColorNever); err != nil {
		t.Fatalf("PullDryRun error: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(store, "a")); string(got) != "new\n" {
		t.Errorf("pull dry-run must NOT write Store: content = %q", got)
	}
}

func TestDryRunColorNeverHasNoANSI(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	for _, fn := range []func(string, []config.Entry, string) (string, bool, error){PushDryRun, PullDryRun} {
		out, hasDiff, err := fn(store, entries, ColorNever)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !hasDiff {
			t.Fatal("hasDiff = false, want true")
		}
		if strings.Contains(out, "\x1b[") {
			t.Errorf("color=never must not contain ANSI, got %q", out)
		}
		if !strings.Contains(out, "---") || !strings.Contains(out, "+++") {
			t.Errorf("output should contain ---/+++, got %q", out)
		}
	}
}

func TestDryRunColorAlwaysHasANSI(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := PushDryRun(store, entries, ColorAlways)
	if err != nil {
		t.Fatalf("PushDryRun error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("color=always must contain ANSI, got %q", out)
	}
}

func TestDryRunLimitsContext(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	var oldBody, newBody strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&oldBody, "line %d\n", i)
		if i == 10 {
			fmt.Fprintln(&newBody, "line 10 changed")
		} else {
			fmt.Fprintf(&newBody, "line %d\n", i)
		}
	}

	writeTestFile(t, filepath.Join(store, "a"), newBody.String())
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, oldBody.String())
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := PushDryRun(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("PushDryRun error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if strings.Contains(out, "line 1\n") {
		t.Errorf("context外の line 1 を含むべきでない, got %q", out)
	}
	if strings.Contains(out, "line 20\n") {
		t.Errorf("context外の line 20 を含むべきでない, got %q", out)
	}
	if !strings.Contains(out, "line 10 changed") {
		t.Errorf("変更行を含むべき, got %q", out)
	}
}

func TestDryRunSplitsHunks(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	var oldBody, newBody strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&oldBody, "line %d\n", i)
		if i == 5 || i == 25 {
			fmt.Fprintf(&newBody, "line %d changed\n", i)
		} else {
			fmt.Fprintf(&newBody, "line %d\n", i)
		}
	}

	writeTestFile(t, filepath.Join(store, "a"), newBody.String())
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, oldBody.String())
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := PushDryRun(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("PushDryRun error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if strings.Contains(out, "line 15\n") {
		t.Errorf("離れたハンクの中間行を含むべきでない, got %q", out)
	}
	if !strings.Contains(out, "line 5 changed") || !strings.Contains(out, "line 25 changed") {
		t.Errorf("両変更行を含むべき, got %q", out)
	}
}
