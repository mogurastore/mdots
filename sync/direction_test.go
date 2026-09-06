package sync

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mogurastore/mdots/config"
)

// Seam: sync パッケージ公開境界 (diff の方向固定)
// 内容を入れ替えても Store→dest 方向で安定することを検証する。
func TestDiffDirectionFixed(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "old\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "new\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	out, _, err := DiffWithColor(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	if !strings.Contains(out, "--- a\n") || !strings.Contains(out, "+++ "+dest+"\n") {
		t.Errorf("diff must be Store->dest headers, got %q", out)
	}
}
