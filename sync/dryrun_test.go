package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mogurastore/mdots/config"
)

// Seam: sync パッケージ公開境界 (push/pull の dry-run)
// 実FS上で書き込みなし・差分相当出力の外部挙動のみを検証する。
// Store/dest 準備の定型は sync_test.go のヘルパに集約している。
func TestDryRunPushShowsDiffWithoutWriting(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := DryRunPush(store, entries)
	if err != nil {
		t.Fatalf("DryRunPush error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if !strings.Contains(out, "---") || !strings.Contains(out, "+++") {
		t.Errorf("output should contain ---/+++, got %q", out)
	}
	if got, _ := os.ReadFile(dest); string(got) != "old\n" {
		t.Errorf("dest must NOT be written: content = %q, want %q", got, "old\n")
	}
}

func TestDryRunPushReportsNewFileWithoutCreating(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := DryRunPush(store, entries)
	if err != nil {
		t.Fatalf("DryRunPush error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true (new file)")
	}
	if out == "" {
		t.Error("output empty, want new-file notice")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("dest must NOT be created on dry-run: stat err = %v", err)
	}
}

func TestDryRunPushMissingSrcIsError(t *testing.T) {
	store, destRoot := setupSyncDirs(t)
	entries := []config.Entry{{Src: "missing", Dest: filepath.Join(destRoot, "missing")}}
	if _, _, err := DryRunPush(store, entries); err == nil {
		t.Error("エラー expected, got nil")
	}
}

func TestDryRunPullShowsDiffWithoutWriting(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	srcPath := filepath.Join(store, "a")
	writeTestFile(t, srcPath, "old\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "new\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := DryRunPull(store, entries)
	if err != nil {
		t.Fatalf("DryRunPull error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if !strings.Contains(out, "---") || !strings.Contains(out, "+++") {
		t.Errorf("output should contain ---/+++, got %q", out)
	}
	if got, _ := os.ReadFile(srcPath); string(got) != "old\n" {
		t.Errorf("Store must NOT be written: content = %q, want %q", got, "old\n")
	}
}

func TestDryRunPullReportsNewStoreFileWithoutCreating(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "new\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := DryRunPull(store, entries)
	if err != nil {
		t.Fatalf("DryRunPull error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true (new Store file)")
	}
	if out == "" {
		t.Error("output empty, want new-file notice")
	}
	if _, err := os.Stat(filepath.Join(store, "a")); !os.IsNotExist(err) {
		t.Errorf("Store src must NOT be created on dry-run: stat err = %v", err)
	}
}

func TestDryRunPullMissingDestIsError(t *testing.T) {
	store, destRoot := setupSyncDirs(t)
	entries := []config.Entry{{Src: "a", Dest: filepath.Join(destRoot, "missing")}}
	if _, _, err := DryRunPull(store, entries); err == nil {
		t.Error("エラー expected, got nil")
	}
}

func TestDryRunPushColorNeverHasNoANSI(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := DryRunPushWithColor(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("DryRunPushWithColor error: %v", err)
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

func TestDryRunPushColorAlwaysHasANSI(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, hasDiff, err := DryRunPushWithColor(store, entries, ColorAlways)
	if err != nil {
		t.Fatalf("DryRunPushWithColor error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("color=always must contain ANSI, got %q", out)
	}
}

func TestDryRunPushLimitsContext(t *testing.T) {
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

	out, hasDiff, err := DryRunPushWithColor(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("DryRunPushWithColor error: %v", err)
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

func TestDryRunPushSplitsHunks(t *testing.T) {
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

	out, hasDiff, err := DryRunPushWithColor(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("DryRunPushWithColor error: %v", err)
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
