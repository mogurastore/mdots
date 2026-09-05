package sync

import (
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
