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
// 実体は単一の差分コア（欠落時ポリシーのみ引数化）への薄い委譲である。
func Diff(storeRoot string, entries []config.Entry) (output string, hasDiff bool, err error) {
	return diffCore(storeRoot, entries, policyStrict)
}

// missingPolicy は差分コアにおける欠落時の扱いを表す。
// 出力・exitの振る舞いは変えず、欠落時ポリシーの違いのみを引数化する。
type missingPolicy int

const (
	// policyStrict は Diff 用：src/dest のいずれの欠落もエラーにする。
	policyStrict missingPolicy = iota
	// policyPushDryRun は push の dry-run 用：dest 不在を新規作成予定として報告する。
	policyPushDryRun
	// policyPullDryRun は pull の dry-run 用：Store の src 不在を新規回収予定として報告する。
	policyPullDryRun
)

// opLabel は差分コアのエラー接頭辞をポリシーから導く。
// 従来の Diff/dry-run の文面をそのまま保つ。
func (p missingPolicy) opLabel() string {
	switch p {
	case policyPushDryRun:
		return "dry-run push"
	case policyPullDryRun:
		return "dry-run pull"
	default:
		return "diff"
	}
}

// writeNewFileNotice は欠落時の新規作成予定報告を一本化する。文面自体は従来通り。
func writeNewFileNotice(sb *strings.Builder, srcLabel, destLabel, body string) {
	sb.WriteString("--- " + srcLabel + "\n")
	sb.WriteString("+++ " + destLabel + "\n")
	sb.WriteString(body)
}

// diffCore は dry-run 系と Diff の単一の差分コアである。
// 両方存在する Entry は unified diff を出し、不在の扱いだけを policy で切り替える。
// 自前の unified diff 実装・出力文面・exitの対応（hasDiff）は変えない。
func diffCore(storeRoot string, entries []config.Entry, policy missingPolicy) (string, bool, error) {
	op := policy.opLabel()
	var sb strings.Builder
	hasDiff := false
	for _, e := range entries {
		destPath, err := config.ExpandDest(e.Dest)
		if err != nil {
			return "", false, fmt.Errorf("%s %s: dest expand: %w", op, e.Src, err)
		}
		srcPath := filepath.Join(storeRoot, e.Src)
		srcInfo, srcErr := os.Stat(srcPath)
		destInfo, destErr := os.Stat(destPath)
		switch policy {
		case policyPushDryRun:
			if srcErr != nil {
				return "", false, fmt.Errorf("%s %s: %w", op, e.Src, srcErr)
			}
			if srcInfo.IsDir() {
				return "", false, fmt.Errorf("%s %s: src is a directory: %s", op, e.Src, srcPath)
			}
			if destErr != nil {
				if !os.IsNotExist(destErr) {
					return "", false, fmt.Errorf("%s %s: %w", op, e.Src, destErr)
				}
				writeNewFileNotice(&sb, e.Src, e.Dest, "(new file: "+e.Dest+" would be created)\n")
				hasDiff = true
				continue
			}
			if destInfo.IsDir() {
				return "", false, fmt.Errorf("%s %s: dest is a directory: %s", op, e.Src, destPath)
			}
		case policyPullDryRun:
			if destErr != nil {
				return "", false, fmt.Errorf("%s %s: %w", op, e.Src, destErr)
			}
			if destInfo.IsDir() {
				return "", false, fmt.Errorf("%s %s: dest is a directory: %s", op, e.Src, destPath)
			}
			if srcErr != nil {
				if !os.IsNotExist(srcErr) {
					return "", false, fmt.Errorf("%s %s: %w", op, e.Src, srcErr)
				}
				writeNewFileNotice(&sb, e.Src, e.Dest, "(new file: "+e.Src+" would be created in Store)\n")
				hasDiff = true
				continue
			}
			if srcInfo.IsDir() {
				return "", false, fmt.Errorf("%s %s: Store src is a directory: %s", op, e.Src, srcPath)
			}
		default:
			if srcErr != nil {
				return "", false, fmt.Errorf("%s %s: %w", op, e.Src, srcErr)
			}
			if srcInfo.IsDir() {
				return "", false, fmt.Errorf("%s %s: src is a directory: %s", op, e.Src, srcPath)
			}
			if destErr != nil {
				return "", false, fmt.Errorf("%s %s: %w", op, e.Src, destErr)
			}
			if destInfo.IsDir() {
				return "", false, fmt.Errorf("%s %s: dest is a directory: %s", op, e.Src, destPath)
			}
		}
		d, same, err := diffContent(srcPath, destPath, e.Src, e.Dest)
		if err != nil {
			return "", false, fmt.Errorf("%s %s: %w", op, e.Src, err)
		}
		if !same {
			sb.WriteString(d)
			hasDiff = true
		}
	}
	return sb.String(), hasDiff, nil
}

// diffContent は存在確認済みの2ファイルの unified diff を返す。同一内容なら same=true。
// stat は呼び出し側の差分コアで済ませているため、ここでは読み込みと比較のみ行う。
func diffContent(srcPath, destPath, srcLabel, destLabel string) (diffText string, same bool, err error) {
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
// 実体は単一の差分コアへの薄い委譲である。
func DryRunPush(storeRoot string, entries []config.Entry) (output string, hasDiff bool, err error) {
	return diffCore(storeRoot, entries, policyPushDryRun)
}

// DryRunPull は pull の差分相当を返す。実際の書き込みは行わない。
// 両方存在する Entry は Diff と同じ unified diff を出し、
// Store の src 不在の Entry は新規回収予定として報告する。
// dest 不在・ディレクトリは pull と同様にエラーで中断する。
// 実体は単一の差分コアへの薄い委譲である。
func DryRunPull(storeRoot string, entries []config.Entry) (output string, hasDiff bool, err error) {
	return diffCore(storeRoot, entries, policyPullDryRun)
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
