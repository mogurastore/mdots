package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mogurastore/mdots/internal/config"
)

// Seam: sync パッケージ公開境界 (push/pull dry-run の stat 表示)
// 実FSでファイル別の変更行数と方向ラベル・集計行を検証する。
func TestPushDryRunStatShowsFileChanges(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}

	out, hasDiff, err := PushDryRunStat(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("PushDryRunStat error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	if strings.Contains(out, "---") || strings.Contains(out, "+++") {
		t.Errorf("stat must not render a patch, got %q", out)
	}
	for _, want := range []string{" a | 2 +-", " 1 file changed, 1 insertion(+), 1 deletion(-)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got %q", want, out)
		}
	}
	if got, _ := os.ReadFile(dest); string(got) != "old\n" {
		t.Errorf("stat dry-run must NOT write dest: content = %q", got)
	}
}

func TestPullDryRunStatUsesNewSideLabel(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "old\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "one\ntwo\n")
	entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}

	out, hasDiff, err := PullDryRunStat(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("PullDryRunStat error: %v", err)
	}
	if !hasDiff {
		t.Fatal("hasDiff = false, want true")
	}
	for _, want := range []string{
		" " + dest + " | 3 ++-",
		" 1 file changed, 2 insertions(+), 1 deletion(-)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got %q", want, out)
		}
	}
}

func TestDryRunStatReportsNewFileAsInsertions(t *testing.T) {
	t.Run("push dest不在", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		writeTestFile(t, filepath.Join(store, "a"), "one\ntwo\n")
		entries := []config.Entry{{Src: "a", Dest: filepath.Join(destRoot, "a"), Override: true}}

		out, hasDiff, err := PushDryRunStat(store, entries, ColorNever)
		if err != nil {
			t.Fatalf("PushDryRunStat error: %v", err)
		}
		if !hasDiff {
			t.Fatal("hasDiff = false, want true")
		}
		for _, want := range []string{" a | 2 ++", " 1 file changed, 2 insertions(+)"} {
			if !strings.Contains(out, want) {
				t.Errorf("output should contain %q, got %q", want, out)
			}
		}
	})

	t.Run("pull Store不在", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "one\ntwo\n")
		entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}

		out, hasDiff, err := PullDryRunStat(store, entries, ColorNever)
		if err != nil {
			t.Fatalf("PullDryRunStat error: %v", err)
		}
		if !hasDiff {
			t.Fatal("hasDiff = false, want true")
		}
		for _, want := range []string{" " + dest + " | 2 ++", " 1 file changed, 2 insertions(+)"} {
			if !strings.Contains(out, want) {
				t.Errorf("output should contain %q, got %q", want, out)
			}
		}
	})
}

func TestDryRunStatNoChange(t *testing.T) {
	store, destRoot := setupSyncDirs(t)
	writeTestFile(t, filepath.Join(store, "a"), "same\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "same\n")
	entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}

	for _, fn := range []func(string, []config.Entry, string) (string, bool, error){PushDryRunStat, PullDryRunStat} {
		out, hasDiff, err := fn(store, entries, ColorNever)
		if err != nil {
			t.Fatalf("stat error: %v", err)
		}
		if hasDiff || out != "" {
			t.Errorf("no change: hasDiff=%v out=%q, want false and empty", hasDiff, out)
		}
	}
}

func TestDryRunStatColor(t *testing.T) {
	store, destRoot := setupSyncDirs(t)
	writeTestFile(t, filepath.Join(store, "a"), "new\n")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old\n")
	entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}

	if out, _, err := PushDryRunStat(store, entries, ColorNever); err != nil {
		t.Fatalf("ColorNever error: %v", err)
	} else if strings.Contains(out, "\x1b[") {
		t.Errorf("color=never must not contain ANSI, got %q", out)
	}
	if out, _, err := PushDryRunStat(store, entries, ColorAlways); err != nil {
		t.Fatalf("ColorAlways error: %v", err)
	} else if !strings.Contains(out, "\x1b[") {
		t.Errorf("color=always must contain ANSI, got %q", out)
	}
}

func TestDryRunStatCapsBarWidth(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	var oldBody, newBody strings.Builder
	for i := 0; i < 120; i++ {
		oldBody.WriteString("old\n")
		newBody.WriteString("new\n")
	}
	writeTestFile(t, filepath.Join(store, "a"), newBody.String())
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, oldBody.String())
	entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}

	out, _, err := PushDryRunStat(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("PushDryRunStat error: %v", err)
	}
	if !strings.Contains(out, "| 240 ") {
		t.Errorf("stat total should stay exact, got %q", out)
	}
	line := strings.Split(out, "\n")[0]
	bar := line[strings.LastIndex(line, " ")+1:]
	if len(bar) > 50 {
		t.Errorf("bar width = %d, want <= 50 (%q)", len(bar), line)
	}
}

func TestDryRunStatCountsLinesLikeGit(t *testing.T) {
	t.Run("既存の空ファイルへの追加は末尾の空行を数えない", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		writeTestFile(t, filepath.Join(store, "a"), "one\ntwo\n")
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "")
		entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}

		out, _, err := PushDryRunStat(store, entries, ColorNever)
		if err != nil {
			t.Fatalf("PushDryRunStat error: %v", err)
		}
		if !strings.Contains(out, " a | 2 ++") {
			t.Errorf("output should count 2 lines, got %q", out)
		}
	})

	t.Run("末尾改行の追加は1追加1削除", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		writeTestFile(t, filepath.Join(store, "a"), "one\n")
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "one")
		entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}

		out, _, err := PushDryRunStat(store, entries, ColorNever)
		if err != nil {
			t.Fatalf("PushDryRunStat error: %v", err)
		}
		if !strings.Contains(out, " a | 2 +-") {
			t.Errorf("output should count 1 addition and 1 deletion, got %q", out)
		}
	})
}

func TestDryRunStatScalesBarsCommonly(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), strings.Repeat("a\n", 100))
	writeTestFile(t, filepath.Join(store, "b"), strings.Repeat("b\n", 10))
	entries := []config.Entry{
		{Src: "a", Dest: filepath.Join(destRoot, "a"), Override: true},
		{Src: "b", Dest: filepath.Join(destRoot, "b"), Override: true},
	}

	out, _, err := PushDryRunStat(store, entries, ColorNever)
	if err != nil {
		t.Fatalf("PushDryRunStat error: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 2 stat lines + summary, got %q", out)
	}
	for i, want := range []int{50, 5} {
		parts := strings.SplitN(lines[i], " | ", 2)
		if len(parts) != 2 {
			t.Fatalf("line %d is not a stat line: %q", i, lines[i])
		}
		bar := parts[1][strings.LastIndex(parts[1], " ")+1:]
		if len(bar) != want {
			t.Errorf("line %d bar width = %d, want %d (%q)", i, len(bar), want, lines[i])
		}
	}
}
