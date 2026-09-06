package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mogurastore/mdots/config"
)

// Seam: sync パッケージ公開境界 (diff)
// 実FS上で書き込みなし・差分出力の外部挙動のみを検証する。
// Store/dest 準備の定型は sync_test.go のヘルパに集約している。
func TestDiffShowsStoreToDestFixed(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := DiffWithColor(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if !strings.Contains(out, "--- a\n") || !strings.Contains(out, "+++ "+dest+"\n") {
		t.Errorf("diff must be Store->dest headers, got %q", out)
	}
	if got, _ := os.ReadFile(dest); string(got) != "old\n" {
		t.Errorf("dest must NOT be written: content = %q", got)
	}
}

func TestDiffReportsMissingEitherSideAsNew(t *testing.T) {
	t.Run("dest不在は新規予定", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		writeTestFile(t, filepath.Join(store, "a"), "new\n")
		dest := filepath.Join(destRoot, "a")
		entries := []config.Entry{{Src: "a", Dest: dest}}

		out, hasDiff, err := Diff(store, entries)
		if err != nil {
			t.Fatalf("Diff error: %v", err)
		}
		if !hasDiff || out == "" {
			t.Fatal("want new-file notice, got empty")
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Errorf("dest must NOT be created: stat err = %v", err)
		}
	})

	t.Run("Store不在は新規回収予定", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "new\n")
		entries := []config.Entry{{Src: "a", Dest: dest}}

		out, hasDiff, err := Diff(store, entries)
		if err != nil {
			t.Fatalf("Diff error: %v", err)
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
		if _, _, err := Diff(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})
}

func TestDiffNoChange(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "same\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "same\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := Diff(store, entries)
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	if hasDiff {
		t.Errorf("hasDiff = true, want false (out=%q)", out)
	}
}

func TestDiffColorNeverHasNoANSI(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := DiffWithColor(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("DiffWithColor error: %v", err)
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

func TestDiffColorAlwaysHasANSI(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := DiffWithColor(store, entries, ColorAlways)
	if err != nil {
		t.Fatalf("DiffWithColor error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("color=always must contain ANSI, got %q", out)
	}
}

func TestDiffLimitsContext(t *testing.T) {
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

	out, hasDiff, err := DiffWithColor(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("DiffWithColor error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	// 前後3行に絞られるため先頭・末尾行は含まれない
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

func TestDiffSplitsHunks(t *testing.T) {
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

	out, hasDiff, err := DiffWithColor(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("DiffWithColor error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	// 離れた2変更の中間行は省略され、両変更は残る
	if strings.Contains(out, "line 15\n") {
		t.Errorf("離れたハンクの中間行を含むべきでない, got %q", out)
	}
	if !strings.Contains(out, "line 5 changed") || !strings.Contains(out, "line 25 changed") {
		t.Errorf("両変更行を含むべき, got %q", out)
	}
}
