package sync

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mogurastore/mdots/config"
)

// Seam: sync パッケージ公開境界 (push/pull の dry-run 方向)
// 同一fixtureで push と pull が対称反転することを検証する。
func TestDryRunPushPullDirectionSymmetric(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "old\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "new\n")
	entries := []config.Entry{{Src: "a", Dest: dest}}

	pushOut, _, err := DryRunPushWithColor(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("DryRunPush error: %v", err)
	}
	pullOut, _, err := DryRunPullWithColor(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("DryRunPull error: %v", err)
	}
	if pushOut == pullOut {
		t.Errorf("push/pull outputs must differ for same fixture,\ngot push=%q pull=%q", pushOut, pullOut)
	}
	if !strings.Contains(pushOut, "--- a\n") || !strings.Contains(pushOut, "+++ "+dest+"\n") {
		t.Errorf("push must be Store->dest headers, got %q", pushOut)
	}
	if !strings.Contains(pullOut, "--- "+dest+"\n") || !strings.Contains(pullOut, "+++ a\n") {
		t.Errorf("pull must be dest->Store headers, got %q", pullOut)
	}
}
