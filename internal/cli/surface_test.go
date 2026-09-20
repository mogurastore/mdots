package cli

import (
	"strings"
	"testing"
)

// Seam: CLIコマンド境界 (Store不在時の exit＋文言)
// 全コマンドのexit＋文言をここに集約する。実行系の詳細は internal/app が担う。
func TestCliStoreNotFoundFriendlyError(t *testing.T) {
	empty := t.TempDir()
	for _, args := range [][]string{{"push"}, {"pull"}, {"push", "--dry-run"}, {"pull", "--dry-run"}, {"targets"}} {
		code, _, errOut := runCli(t, empty, args)
		if code == 0 {
			t.Errorf("Run(%v) without Store: exit = 0, want non-zero", args)
		}
		if !strings.Contains(errOut, "mdots.toml not found in "+empty) {
			t.Errorf("Run(%v): stderr should contain friendly message, got %q", args, errOut)
		}
	}
}
