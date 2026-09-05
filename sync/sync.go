package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mogurastore/mdots/config"
)

// Push は Store の src を dest へファイルコピーする。
// Entry の src は Store 相対、dest は ~ 展開される配置先パス。
// 親ディレクトリは mkdir -p、パーミッションは元ファイルに追従、上書きは無確認。
// src/dest がディレクトリの場合はエラーになる。src 不在もエラーで中断する。
func Push(storeRoot string, entries []config.Entry) error {
	for _, e := range entries {
		srcPath := filepath.Join(storeRoot, e.Src)
		destPath, err := config.ExpandDest(e.Dest)
		if err != nil {
			return fmt.Errorf("push %s: dest expand: %w", e.Src, err)
		}
		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("push %s: %w", e.Src, err)
		}
	}
	return nil
}

// Diff は Store の src と dest の差分を diff -u 風の文字列で返す。
// 差分がない Entry は出力に含めない。差分が1件でもあれば hasDiff=true。
// src/dest が不在・ディレクトリの場合はエラーで中断する。
func Diff(storeRoot string, entries []config.Entry) (output string, hasDiff bool, err error) {
	var sb strings.Builder
	for _, e := range entries {
		destPath, err := config.ExpandDest(e.Dest)
		if err != nil {
			return "", false, fmt.Errorf("diff %s: dest expand: %w", e.Src, err)
		}
		srcPath := filepath.Join(storeRoot, e.Src)
		d, same, err := diffFile(srcPath, destPath, e.Src, e.Dest)
		if err != nil {
			return "", false, fmt.Errorf("diff %s: %w", e.Src, err)
		}
		if !same {
			sb.WriteString(d)
			hasDiff = true
		}
	}
	return sb.String(), hasDiff, nil
}

// diffFile は2ファイルの unified diff を返す。同一内容なら same=true。
func diffFile(srcPath, destPath, srcLabel, destLabel string) (diffText string, same bool, err error) {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return "", false, err
	}
	if srcInfo.IsDir() {
		return "", false, fmt.Errorf("src is a directory: %s", srcPath)
	}
	destInfo, err := os.Stat(destPath)
	if err != nil {
		return "", false, err
	}
	if destInfo.IsDir() {
		return "", false, fmt.Errorf("dest is a directory: %s", destPath)
	}
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return "", false, err
	}
	destData, err := os.ReadFile(destPath)
	if err != nil {
		return "", false, err
	}
	if string(srcData) == string(destData) {
		return "", true, nil
	}
	var sb strings.Builder
	sb.WriteString("--- " + srcLabel + "\n")
	sb.WriteString("+++ " + destLabel + "\n")
	sb.WriteString(unifiedBody(splitLines(string(srcData)), splitLines(string(destData))))
	return sb.String(), false, nil
}

// splitLines は末尾改行を保持しない行分割を行う。
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// unifiedBody は2つの行列から @@ ハンク付きの unified diff 本体を作る。
// LCS ベースの単純な差分で、dotfiles 程度の小規模ファイルを想定する。
func unifiedBody(srcLines, destLines []string) string {
	n, m := len(srcLines), len(destLines)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if srcLines[i] == destLines[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	type op struct {
		kind string
		line string
	}
	var ops []op
	for i, j := 0, 0; i < n || j < m; {
		switch {
		case i < n && j < m && srcLines[i] == destLines[j]:
			ops = append(ops, op{" ", srcLines[i]})
			i++
			j++
		case j >= m || (i < n && dp[i+1][j] >= dp[i][j+1]):
			ops = append(ops, op{"-", srcLines[i]})
			i++
		default:
			ops = append(ops, op{"+", destLines[j]})
			j++
		}
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", 1, n, 1, m))
	for _, o := range ops {
		sb.WriteString(o.kind + o.line + "\n")
	}
	return sb.String()
}

// DryRunPush は push の差分相当を返す。実際の書き込みは行わない。
// 両方存在する Entry は Diff と同じ unified diff を出し、
// dest 不在の Entry は新規作成予定として報告する。
// src 不在・ディレクトリは push と同様にエラーで中断する。
func DryRunPush(storeRoot string, entries []config.Entry) (output string, hasDiff bool, err error) {
	return dryRun(storeRoot, entries, "push")
}

// DryRunPull は pull の差分相当を返す。実際の書き込みは行わない。
// 両方存在する Entry は Diff と同じ unified diff を出し、
// Store の src 不在の Entry は新規回収予定として報告する。
// dest 不在・ディレクトリは pull と同様にエラーで中断する。
func DryRunPull(storeRoot string, entries []config.Entry) (output string, hasDiff bool, err error) {
	return dryRun(storeRoot, entries, "pull")
}

// dryRun は direction ("push"/"pull") に応じた欠落時の扱いで差分相当を作る。
func dryRun(storeRoot string, entries []config.Entry, direction string) (string, bool, error) {
	var sb strings.Builder
	hasDiff := false
	for _, e := range entries {
		destPath, err := config.ExpandDest(e.Dest)
		if err != nil {
			return "", false, fmt.Errorf("dry-run %s %s: dest expand: %w", direction, e.Src, err)
		}
		srcPath := filepath.Join(storeRoot, e.Src)
		if direction == "push" {
			srcInfo, err := os.Stat(srcPath)
			if err != nil {
				return "", false, fmt.Errorf("dry-run push %s: %w", e.Src, err)
			}
			if srcInfo.IsDir() {
				return "", false, fmt.Errorf("dry-run push %s: src is a directory: %s", e.Src, srcPath)
			}
			if destInfo, err := os.Stat(destPath); err != nil {
				if !os.IsNotExist(err) {
					return "", false, fmt.Errorf("dry-run push %s: %w", e.Src, err)
				}
				sb.WriteString("--- " + e.Src + "\n")
				sb.WriteString("+++ " + e.Dest + "\n")
				sb.WriteString("(new file: " + e.Dest + " would be created)\n")
				hasDiff = true
				continue
			} else if destInfo.IsDir() {
				return "", false, fmt.Errorf("dry-run push %s: dest is a directory: %s", e.Src, destPath)
			}
		} else {
			destInfo, err := os.Stat(destPath)
			if err != nil {
				return "", false, fmt.Errorf("dry-run pull %s: %w", e.Src, err)
			}
			if destInfo.IsDir() {
				return "", false, fmt.Errorf("dry-run pull %s: dest is a directory: %s", e.Src, destPath)
			}
			if srcInfo, err := os.Stat(srcPath); err != nil {
				if !os.IsNotExist(err) {
					return "", false, fmt.Errorf("dry-run pull %s: %w", e.Src, err)
				}
				sb.WriteString("--- " + e.Src + "\n")
				sb.WriteString("+++ " + e.Dest + "\n")
				sb.WriteString("(new file: " + e.Src + " would be created in Store)\n")
				hasDiff = true
				continue
			} else if srcInfo.IsDir() {
				return "", false, fmt.Errorf("dry-run pull %s: Store src is a directory: %s", e.Src, srcPath)
			}
		}
		d, same, err := diffFile(srcPath, destPath, e.Src, e.Dest)
		if err != nil {
			return "", false, fmt.Errorf("dry-run %s %s: %w", direction, e.Src, err)
		}
		if !same {
			sb.WriteString(d)
			hasDiff = true
		}
	}
	return sb.String(), hasDiff, nil
}

// Pull は dest を Store の src へファイルコピーする。
// Entry の src は Store 相対、dest は ~ 展開される配置先パス。
// Push と同じく親ディレクトリは mkdir -p、パーミッションは元ファイルに追従、上書きは無確認。
// dest が不在・ディレクトリの場合はエラーで中断する。Store 側がディレクトリの場合もエラーになる。
func Pull(storeRoot string, entries []config.Entry) error {
	for _, e := range entries {
		destPath, err := config.ExpandDest(e.Dest)
		if err != nil {
			return fmt.Errorf("pull %s: dest expand: %w", e.Src, err)
		}
		srcPath := filepath.Join(storeRoot, e.Src)
		if err := copyFile(destPath, srcPath); err != nil {
			return fmt.Errorf("pull %s: %w", e.Src, err)
		}
	}
	return nil
}

func copyFile(srcPath, destPath string) error {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("src is a directory: %s", srcPath)
	}
	if destInfo, err := os.Stat(destPath); err == nil && destInfo.IsDir() {
		return fmt.Errorf("dest is a directory: %s", destPath)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if err := os.Chmod(destPath, srcInfo.Mode().Perm()); err != nil {
		return err
	}
	return nil
}
