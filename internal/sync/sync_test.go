package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mogurastore/mdots/internal/config"
)

// setupSyncDirs は Store/dest の実FS準備の定型を集約する。
func setupSyncDirs(t *testing.T) (store, destRoot string) {
	t.Helper()
	return t.TempDir(), t.TempDir()
}

// writeTestFile はテスト用ファイル書き込みの定型を集約する。
func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// isolateHome は dest の ~ 展開先を隔離する定型を集約する。
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// Seam: sync パッケージ公開境界 (push)
// Store/src → dest のファイルコピーを t.TempDir() の実FSで検証する。
func TestPushCopiesStoreToDest(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "vimrc"), "set number\n")
	dest := filepath.Join(destRoot, ".vimrc")
	entries := []config.Entry{{Src: "vimrc", Dest: dest, Override: true}}

	if _, err := Push(store, entries); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("dest read error: %v", err)
	}
	if string(got) != "set number\n" {
		t.Errorf("dest content = %q, want %q", got, "set number\n")
	}
}

func TestPushCreatesParentDirs(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "wezterm.lua"), "return {}\n")
	dest := filepath.Join(destRoot, ".config", "wezterm", "wezterm.lua")
	entries := []config.Entry{{Src: "wezterm.lua", Dest: dest, Override: true}}

	if _, err := Push(store, entries); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("dest not created: %v", err)
	}
}

func TestPushExpandsHomeDest(t *testing.T) {
	store, _ := setupSyncDirs(t)
	home := isolateHome(t)

	writeTestFile(t, filepath.Join(store, "bashrc"), "export X=1\n")
	entries := []config.Entry{{Src: "bashrc", Dest: "~/.bashrc", Override: true}}

	if _, err := Push(store, entries); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	if err != nil {
		t.Fatalf("expanded dest read error: %v", err)
	}
	if string(got) != "export X=1\n" {
		t.Errorf("dest content = %q, want %q", got, "export X=1\n")
	}
}

func TestPushRejectsDirectories(t *testing.T) {
	t.Run("srcがディレクトリはエラー", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		if err := os.MkdirAll(filepath.Join(store, "mydir"), 0o755); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "mydir", Dest: filepath.Join(destRoot, "mydir"), Override: true}}
		if _, err := Push(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})

	t.Run("destがディレクトリはエラー", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		writeTestFile(t, filepath.Join(store, "a"), "a")
		destDir := filepath.Join(destRoot, "existing")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "a", Dest: destDir, Override: true}}
		if _, err := Push(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})
}

func TestPushOverwritesExisting(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	writeTestFile(t, filepath.Join(store, "a"), "new")
	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "old")
	entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}

	if _, err := Push(store, entries); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("dest content = %q, want %q", got, "new")
	}
}

func TestPushPreservesPermission(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	srcPath := filepath.Join(store, "run.sh")
	writeTestFile(t, srcPath, "#!/bin/sh\n")
	if err := os.Chmod(srcPath, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(destRoot, "run.sh")
	entries := []config.Entry{{Src: "run.sh", Dest: dest, Override: true}}

	if _, err := Push(store, entries); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("dest perm = %o, want 755", info.Mode().Perm())
	}
}

// Seam: sync パッケージ公開境界 (pull)
// dest → Store/src のファイルコピーを t.TempDir() の実FSで検証する。
func TestPullCopiesDestToStore(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	dest := filepath.Join(destRoot, ".vimrc")
	writeTestFile(t, dest, "edited\n")
	entries := []config.Entry{{Src: "vimrc", Dest: dest, Override: true}}

	if _, err := Pull(store, entries); err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(store, "vimrc"))
	if err != nil {
		t.Fatalf("store read error: %v", err)
	}
	if string(got) != "edited\n" {
		t.Errorf("store content = %q, want %q", got, "edited\n")
	}
}

func TestPullCreatesParentDirsOnStoreSide(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	dest := filepath.Join(destRoot, ".config", "wezterm", "wezterm.lua")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, dest, "return {}\n")
	entries := []config.Entry{{Src: filepath.Join(".config", "wezterm", "wezterm.lua"), Dest: dest, Override: true}}

	if _, err := Pull(store, entries); err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store, ".config", "wezterm", "wezterm.lua")); err != nil {
		t.Errorf("store src not created: %v", err)
	}
}

func TestPullExpandsHomeDest(t *testing.T) {
	store, _ := setupSyncDirs(t)
	home := isolateHome(t)

	writeTestFile(t, filepath.Join(home, ".bashrc"), "export X=1\n")
	entries := []config.Entry{{Src: "bashrc", Dest: "~/.bashrc", Override: true}}

	if _, err := Pull(store, entries); err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(store, "bashrc"))
	if err != nil {
		t.Fatalf("store read error: %v", err)
	}
	if string(got) != "export X=1\n" {
		t.Errorf("store content = %q, want %q", got, "export X=1\n")
	}
}

func TestPullRejectsDirectories(t *testing.T) {
	t.Run("destがディレクトリはエラー", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		destDir := filepath.Join(destRoot, "existing")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "a", Dest: destDir, Override: true}}
		if _, err := Pull(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})

	t.Run("Store側がディレクトリはエラー", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "a")
		if err := os.MkdirAll(filepath.Join(store, "a"), 0o755); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}
		if _, err := Pull(store, entries); err == nil {
			t.Error("エラー expected, got nil")
		}
	})
}

func TestPullMissingDestIsError(t *testing.T) {
	store, destRoot := setupSyncDirs(t)
	entries := []config.Entry{{Src: "missing", Dest: filepath.Join(destRoot, "missing"), Override: true}}
	if _, err := Pull(store, entries); err == nil {
		t.Error("エラー expected, got nil")
	}
}

func TestPullOverwritesExisting(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	dest := filepath.Join(destRoot, "a")
	writeTestFile(t, dest, "new")
	srcPath := filepath.Join(store, "a")
	writeTestFile(t, srcPath, "old")
	entries := []config.Entry{{Src: "a", Dest: dest, Override: true}}

	if _, err := Pull(store, entries); err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	got, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("store content = %q, want %q", got, "new")
	}
}

func TestPullPreservesPermission(t *testing.T) {
	store, destRoot := setupSyncDirs(t)

	dest := filepath.Join(destRoot, "run.sh")
	writeTestFile(t, dest, "#!/bin/sh\n")
	if err := os.Chmod(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []config.Entry{{Src: "run.sh", Dest: dest, Override: true}}

	if _, err := Pull(store, entries); err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	info, err := os.Stat(filepath.Join(store, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("store perm = %o, want 755", info.Mode().Perm())
	}
}

func TestPushMissingSrcIsError(t *testing.T) {
	store, destRoot := setupSyncDirs(t)
	entries := []config.Entry{{Src: "missing", Dest: filepath.Join(destRoot, "missing"), Override: true}}
	if _, err := Push(store, entries); err == nil {
		t.Error("エラー expected, got nil")
	}
}

// Seam: sync パッケージ公開境界 (push の override 保護)
// 実FSで既存あり＋falseのskip継続・新規作成・権限不変・ディレクトリ時エラーを検証する。
func TestPushOverrideProtectsExisting(t *testing.T) {
	t.Run("既存＋falseは内容・権限不変でskip継続する", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		writeTestFile(t, filepath.Join(store, "a"), "new")
		if err := os.Chmod(filepath.Join(store, "a"), 0o755); err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "old")
		if err := os.Chmod(dest, 0o600); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(store, "b"), "b-new")
		destB := filepath.Join(destRoot, "b")
		entries := []config.Entry{
			{Src: "a", Dest: dest, Override: false},
			{Src: "b", Dest: destB, Override: true},
		}

		skipped, err := Push(store, entries)
		if err != nil {
			t.Fatalf("Push error: %v", err)
		}
		if len(skipped) != 1 || skipped[0].Src != "a" {
			t.Errorf("skipped = %+v, want [a]", skipped)
		}
		if got, _ := os.ReadFile(dest); string(got) != "old" {
			t.Errorf("protected dest content = %q, want %q", got, "old")
		}
		if info, _ := os.Stat(dest); info.Mode().Perm() != 0o600 {
			t.Errorf("protected dest perm = %o, want 600", info.Mode().Perm())
		}
		if got, _ := os.ReadFile(destB); string(got) != "b-new" {
			t.Errorf("other entry must continue: content = %q", got)
		}
	})

	t.Run("不在時はfalseでも新規作成する", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		writeTestFile(t, filepath.Join(store, "a"), "new\n")
		dest := filepath.Join(destRoot, "a")
		entries := []config.Entry{{Src: "a", Dest: dest, Override: false}}

		skipped, err := Push(store, entries)
		if err != nil {
			t.Fatalf("Push error: %v", err)
		}
		if len(skipped) != 0 {
			t.Errorf("new file must not be skipped: %+v", skipped)
		}
		if got, _ := os.ReadFile(dest); string(got) != "new\n" {
			t.Errorf("dest content = %q, want %q", got, "new\n")
		}
	})

	t.Run("ディレクトリ時はfalseでもエラー中断する", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		writeTestFile(t, filepath.Join(store, "a"), "a")
		destDir := filepath.Join(destRoot, "existing")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "a", Dest: destDir, Override: false}}
		if _, err := Push(store, entries); err == nil {
			t.Error("dest dir: エラー expected, got nil")
		}
	})
}

// Seam: sync パッケージ公開境界 (pull の override 保護)
// 実FSで既存あり＋falseのskip継続・新規作成・権限不変・ディレクトリ時エラーを検証する。
func TestPullOverrideProtectsExisting(t *testing.T) {
	t.Run("既存＋falseは内容・権限不変でskip継続する", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "new")
		if err := os.Chmod(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		srcPath := filepath.Join(store, "a")
		writeTestFile(t, srcPath, "old")
		if err := os.Chmod(srcPath, 0o600); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(destRoot, "b"), "b-new")
		entries := []config.Entry{
			{Src: "a", Dest: dest, Override: false},
			{Src: "b", Dest: filepath.Join(destRoot, "b"), Override: true},
		}

		skipped, err := Pull(store, entries)
		if err != nil {
			t.Fatalf("Pull error: %v", err)
		}
		if len(skipped) != 1 || skipped[0].Src != "a" {
			t.Errorf("skipped = %+v, want [a]", skipped)
		}
		if got, _ := os.ReadFile(srcPath); string(got) != "old" {
			t.Errorf("protected store content = %q, want %q", got, "old")
		}
		if info, _ := os.Stat(srcPath); info.Mode().Perm() != 0o600 {
			t.Errorf("protected store perm = %o, want 600", info.Mode().Perm())
		}
		if got, _ := os.ReadFile(filepath.Join(store, "b")); string(got) != "b-new" {
			t.Errorf("other entry must continue: content = %q", got)
		}
	})

	t.Run("不在時はfalseでも新規作成する", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "new\n")
		entries := []config.Entry{{Src: "a", Dest: dest, Override: false}}

		skipped, err := Pull(store, entries)
		if err != nil {
			t.Fatalf("Pull error: %v", err)
		}
		if len(skipped) != 0 {
			t.Errorf("new file must not be skipped: %+v", skipped)
		}
		if got, _ := os.ReadFile(filepath.Join(store, "a")); string(got) != "new\n" {
			t.Errorf("store content = %q, want %q", got, "new\n")
		}
	})

	t.Run("ディレクトリ時はfalseでもエラー中断する", func(t *testing.T) {
		store, destRoot := setupSyncDirs(t)
		dest := filepath.Join(destRoot, "a")
		writeTestFile(t, dest, "a")
		if err := os.MkdirAll(filepath.Join(store, "a"), 0o755); err != nil {
			t.Fatal(err)
		}
		entries := []config.Entry{{Src: "a", Dest: dest, Override: false}}
		if _, err := Pull(store, entries); err == nil {
			t.Error("store dir: エラー expected, got nil")
		}
	})
}
