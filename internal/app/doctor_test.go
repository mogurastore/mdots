package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seam: 実行系境界 (doctor sharable検出)
// Store上のsrc同士のみを比較し、HOME・override・モードは無視する。
// 候補あり→stdout一覧でexit 1、なし→No sharable entries.でexit 0。
// 欠落はskip、ディレクトリ・読み込み失敗はエラー。自動修正はしない。

func TestDoctorDetectsSameDestDifferentSrcIdenticalContent(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{
			"dotfiles/win/.vimrc": "same\n",
			"dotfiles/wsl/.vimrc": "same\n",
		},
		map[string]string{".vimrc": "different-home\n"},
		"default_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"dotfiles/win/.vimrc\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"dotfiles/wsl/.vimrc\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Doctor(store, &out, &errOut); code != 1 {
		t.Fatalf("Doctor with sharable: exit = %d, want 1", code)
	}
	got := out.String()
	for _, want := range []string{"~/.vimrc", "dotfiles/win/.vimrc", "dotfiles/wsl/.vimrc"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout should contain %q, got %q", want, got)
		}
	}
}

func TestDoctorIgnoresSameDestDifferentContent(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{
			"a-src": "aaa\n",
			"b-src": "bbb\n",
		},
		nil,
		"default_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Doctor(store, &out, &errOut); code != 0 {
		t.Fatalf("Doctor with different content: exit = %d, want 0", code)
	}
	if out.String() != "No sharable entries.\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "No sharable entries.\n")
	}
}

func TestDoctorDetectsDifferentDestSameContent(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{
			"a-src": "same\n",
			"b-src": "same\n",
		},
		nil,
		"default_target = \"base\"\n"+
			"[targets.win.\"~/.a\"]\nsrc = \"a-src\"\n"+
			"[targets.win.\"~/.b\"]\nsrc = \"b-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Doctor(store, &out, &errOut); code != 1 {
		t.Fatalf("Doctor with different dest same content: exit = %d, want 1", code)
	}
	got := out.String()
	for _, want := range []string{"a-src", "b-src", "~/.a", "~/.b"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout should contain %q, got %q", want, got)
		}
	}
}

func TestDoctorDetectsCrossTargetDifferentDestSameContent(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{
			"dotfiles/win/AppData/Roaming/jj/config.toml": "same\n",
			"dotfiles/wsl/.config/jj/config.toml":         "same\n",
		},
		nil,
		"default_target = \"base\"\n"+
			"[targets.win.\"~/AppData/Roaming/jj/config.toml\"]\nsrc = \"dotfiles/win/AppData/Roaming/jj/config.toml\"\n"+
			"[targets.wsl.\"~/.config/jj/config.toml\"]\nsrc = \"dotfiles/wsl/.config/jj/config.toml\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Doctor(store, &out, &errOut); code != 1 {
		t.Fatalf("Doctor cross-target different dest: exit = %d, want 1", code)
	}
	got := out.String()
	for _, want := range []string{
		"dotfiles/win/AppData/Roaming/jj/config.toml",
		"dotfiles/wsl/.config/jj/config.toml",
		"~/AppData/Roaming/jj/config.toml",
		"~/.config/jj/config.toml",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout should contain %q, got %q", want, got)
		}
	}
}

func TestDoctorExcludesAlreadySharedSameSrc(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"shared": "same\n"},
		nil,
		"default_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"shared\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"shared\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Doctor(store, &out, &errOut); code != 0 {
		t.Fatalf("Doctor with same src: exit = %d, want 0", code)
	}
	if out.String() != "No sharable entries.\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "No sharable entries.\n")
	}
}

func TestDoctorIgnoresOverrideAndModeDifferences(t *testing.T) {
	store, home := setupStoreWithHome(t,
		map[string]string{
			"a-src": "same\n",
			"b-src": "same\n",
		},
		map[string]string{".vimrc": "home-differs\n"},
		"default_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\noverride = false\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	// モード差があっても内容一致ならsharableとする。
	if err := os.Chmod(filepath.Join(store, "a-src"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(store, "b-src"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = home
	var out, errOut bytes.Buffer
	if code := Doctor(store, &out, &errOut); code != 1 {
		t.Fatalf("Doctor ignoring override/mode: exit = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "~/.vimrc") {
		t.Errorf("stdout should contain dest, got %q", out.String())
	}
}

func TestDoctorReportsPartialMatchesGroupedByContent(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{
			"a-src": "same\n",
			"b-src": "same\n",
			"c-src": "other\n",
		},
		nil,
		"default_target = \"base\"\n"+
			"[targets.a.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.b.\"~/.vimrc\"]\nsrc = \"b-src\"\n"+
			"[targets.c.\"~/.vimrc\"]\nsrc = \"c-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Doctor(store, &out, &errOut); code != 1 {
		t.Fatalf("Doctor partial match: exit = %d, want 1", code)
	}
	got := out.String()
	for _, want := range []string{"a-src", "b-src"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout should contain matching %q, got %q", want, got)
		}
	}
	if strings.Contains(got, "c-src") {
		t.Errorf("stdout must not contain non-matching c-src, got %q", got)
	}
}

func TestDoctorSkipsMissingFilesQuietly(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{"a-src": "same\n"},
		nil,
		"default_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"missing-src\"\n",
	)
	var out, errOut bytes.Buffer
	if code := Doctor(store, &out, &errOut); code != 0 {
		t.Fatalf("Doctor with missing file: exit = %d, want 0", code)
	}
	if out.String() != "No sharable entries.\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "No sharable entries.\n")
	}
	if errOut.String() != "" {
		t.Errorf("stderr should be empty on skip, got %q", errOut.String())
	}
}

func TestDoctorErrorsOnDirectoryAndReadFailures(t *testing.T) {
	t.Run("ディレクトリはエラー", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			map[string]string{"b-src": "same\n"},
			nil,
			"default_target = \"base\"\n"+
				"[targets.win.\"~/.vimrc\"]\nsrc = \"dir-src\"\n"+
				"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
		)
		if err := os.MkdirAll(filepath.Join(store, "dir-src"), 0o755); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		if code := Doctor(store, &out, &errOut); code == 0 {
			t.Fatal("Doctor with directory: exit = 0, want non-zero")
		} else if errOut.String() == "" {
			t.Error("stderr should contain error for directory")
		}
	})

	t.Run("両方欠落はskipでエラーにしない", func(t *testing.T) {
		store, _ := setupStoreWithHome(t,
			nil,
			nil,
			"default_target = \"base\"\n"+
				"[targets.win.\"~/.vimrc\"]\nsrc = \"missing-a\"\n"+
				"[targets.wsl.\"~/.vimrc\"]\nsrc = \"missing-b\"\n",
		)
		var out, errOut bytes.Buffer
		if code := Doctor(store, &out, &errOut); code != 0 {
			t.Fatalf("Doctor all missing: exit = %d, want 0", code)
		}
		if out.String() != "No sharable entries.\n" {
			t.Errorf("stdout = %q, want %q", out.String(), "No sharable entries.\n")
		}
	})
}

func TestDoctorOutputAndNoAutoFix(t *testing.T) {
	store, _ := setupStoreWithHome(t,
		map[string]string{
			"a-src": "same\n",
			"b-src": "same\n",
		},
		nil,
		"default_target = \"base\"\n"+
			"[targets.win.\"~/.vimrc\"]\nsrc = \"a-src\"\n"+
			"[targets.wsl.\"~/.vimrc\"]\nsrc = \"b-src\"\n",
	)
	beforeToml, err := os.ReadFile(filepath.Join(store, "mdots.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Doctor(store, &out, &errOut); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if errOut.String() != "" {
		t.Errorf("stderr should be empty on candidates, got %q", errOut.String())
	}
	afterToml, err := os.ReadFile(filepath.Join(store, "mdots.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeToml) != string(afterToml) {
		t.Errorf("doctor must not modify mdots.toml:\nbefore:\n%s\nafter:\n%s", beforeToml, afterToml)
	}
	for _, p := range []string{"a-src", "b-src"} {
		got, err := os.ReadFile(filepath.Join(store, p))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "same\n" {
			t.Errorf("%s must be unchanged, got %q", p, got)
		}
	}
}

func TestDoctorWithoutStoreFails(t *testing.T) {
	empty := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Doctor(empty, &out, &errOut); code == 0 {
		t.Fatal("Doctor without Store: exit = 0, want non-zero")
	} else if !strings.Contains(errOut.String(), "mdots.toml not found in "+empty) {
		t.Errorf("stderr should contain not-found error, got %q", errOut.String())
	}
}
