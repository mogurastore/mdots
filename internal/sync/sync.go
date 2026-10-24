package sync

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	gd "github.com/amterp/go-delta"
	"github.com/mogurastore/mdots/internal/config"
	"golang.org/x/term"
)

// Push は Store の src を dest へファイルコピーする。
// Entry の src は Store 相対、dest は ~ 展開される配置先パス。
// 親ディレクトリは mkdir -p、パーミッションは元ファイルに追従。
// override=false でコピー先に既存ファイルがある場合は上書きせず skip して継続し、
// 不在時のみ新規作成する。省略時（true）は従来通り上書きする。
// src/dest がディレクトリの場合はエラーで中断する。src 不在もエラーで中断する。
// skip 時は内容・権限を変更しない。戻り値は skip した Entry 群である。
func Push(storeRoot string, entries []config.Entry) ([]config.Entry, error) {
	var skipped []config.Entry
	for _, e := range entries {
		srcPath := filepath.Join(storeRoot, e.Src)
		destPath, err := config.ExpandDest(e.Dest)
		if err != nil {
			return skipped, fmt.Errorf("push %s: dest expand: %w", e.Src, err)
		}
		srcInfo, srcErr := os.Stat(srcPath)
		if srcErr != nil {
			return skipped, fmt.Errorf("push %s: %w", e.Src, srcErr)
		}
		if srcInfo.IsDir() {
			return skipped, fmt.Errorf("push %s: src is a directory: %s", e.Src, srcPath)
		}
		destInfo, destErr := os.Stat(destPath)
		if destErr != nil && !os.IsNotExist(destErr) {
			return skipped, fmt.Errorf("push %s: %w", e.Src, destErr)
		}
		if destErr == nil {
			if destInfo.IsDir() {
				return skipped, fmt.Errorf("push %s: dest is a directory: %s", e.Src, destPath)
			}
			if !e.Override {
				skipped = append(skipped, e)
				continue
			}
		}
		if err := copyFile(srcPath, destPath); err != nil {
			return skipped, fmt.Errorf("push %s: %w", e.Src, err)
		}
	}
	return skipped, nil
}

// ColorAuto etc は push/pull --dry-run の --color 値を表す。auto は FORCE_COLOR > NO_COLOR > tty 判定。
const (
	ColorAuto   = "auto"
	ColorAlways = "always"
	ColorNever  = "never"
)

// resolveUseColor は自前ヘッダ用の着色判定で、go-delta の auto と同じ優先順位にする。
func resolveUseColor(colorMode string) bool {
	switch colorMode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	default:
		if _, ok := os.LookupEnv("FORCE_COLOR"); ok {
			return true
		}
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			return false
		}
		return term.IsTerminal(int(os.Stdout.Fd()))
	}
}

func colorize(s, code string, useColor bool) string {
	if !useColor {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func colorizeBold(s string, useColor bool) string { return colorize(s, "1", useColor) }

func colorizeYellow(s string, useColor bool) string { return colorize(s, "33", useColor) }

func colorizeGreen(s string, useColor bool) string { return colorize(s, "32", useColor) }

func colorizeRed(s string, useColor bool) string { return colorize(s, "31", useColor) }

// writeNewFileNotice は欠落時の新規作成予定報告を一本化する。文面自体は従来通り。
func writeNewFileNotice(sb *strings.Builder, srcLabel, destLabel, body string, useColor bool) {
	sb.WriteString(colorizeBold("--- "+srcLabel+"\n", useColor))
	sb.WriteString(colorizeBold("+++ "+destLabel+"\n", useColor))
	sb.WriteString(colorizeYellow(body, useColor))
}

// diffContent は存在確認済みの2ファイルの差分を返す。同一内容なら same=true。
// stat は呼び出し側の差分コアで済ませているため、ここでは読み込みと比較のみ行う。
// 本体は go-delta の inline 差分（前後3行・word強調）で、---/+++ ヘッダは互換のため自前で付ける。
func diffContent(srcPath, destPath, srcLabel, destLabel string, colorMode string) (diffText string, same bool, err error) {
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
	useColor := resolveUseColor(colorMode)
	var opts []gd.Option
	opts = append(opts, gd.WithContextLines(3), gd.WithLayout(gd.LayoutInline))
	switch colorMode {
	case ColorAlways:
		opts = append(opts, gd.WithColor(true))
	case ColorNever:
		opts = append(opts, gd.WithColor(false))
	}
	body := gd.DiffWith(string(srcData), string(destData), opts...)
	if body == "" {
		return "", true, nil
	}
	var sb strings.Builder
	sb.WriteString(colorizeBold("--- "+srcLabel+"\n", useColor))
	sb.WriteString(colorizeBold("+++ "+destLabel+"\n", useColor))
	sb.WriteString(body)
	if !strings.HasSuffix(sb.String(), "\n") {
		sb.WriteString("\n")
	}
	return sb.String(), false, nil
}

// PushDryRun は push の dry-run 差分を dest→Store 方向で返す。書き込みは行わない。
// pushで追加される行が+になるよう dest を old、Store を new とする。
// 両方存在する Entry は inline diff を出し、片方不在の Entry は新規作成予定として報告する。
// override=false でコピー先が存在する Entry は無出力・差分なし扱いとし、skip 報告はしない。
// 両方不在・ディレクトリはエラーで中断する。
func PushDryRun(storeRoot string, entries []config.Entry, colorMode string) (string, bool, error) {
	return diffDryRun(storeRoot, entries, colorMode, "push", false, false)
}

// PushDryRunStat は push の dry-run 差分をファイル別の変更行数で返す。
// 差分の方向と override/欠落の扱いは PushDryRun と同じで、書き込みは行わない。
func PushDryRunStat(storeRoot string, entries []config.Entry, colorMode string) (string, bool, error) {
	return diffDryRun(storeRoot, entries, colorMode, "push", false, true)
}

// PullDryRun は pull の dry-run 差分を Store→dest 方向で返す。書き込みは行わない。
// pullで取り込まれる行が+になるよう Store を old、dest を new とする。
// override=false でコピー先が存在する Entry は無出力・差分なし扱いとし、skip 報告はしない。
func PullDryRun(storeRoot string, entries []config.Entry, colorMode string) (string, bool, error) {
	return diffDryRun(storeRoot, entries, colorMode, "pull", true, false)
}

// PullDryRunStat は pull の dry-run 差分をファイル別の変更行数で返す。
// 差分の方向と override/欠落の扱いは PullDryRun と同じで、書き込みは行わない。
func PullDryRunStat(storeRoot string, entries []config.Entry, colorMode string) (string, bool, error) {
	return diffDryRun(storeRoot, entries, colorMode, "pull", true, true)
}

// diffDryRun は push/pull の差分コアである。isPull=false は dest を old・
// Store を new とし、true は Store を old・dest を new とする。
// stat=true では patch の代わりにファイル別の変更行数を返す。
func diffDryRun(storeRoot string, entries []config.Entry, colorMode string, op string, isPull bool, stat bool) (string, bool, error) {
	useColor := resolveUseColor(colorMode)
	var sb strings.Builder
	var stats []fileStat
	hasDiff := false
	for _, e := range entries {
		destPath, err := config.ExpandDest(e.Dest)
		if err != nil {
			return "", false, fmt.Errorf("%s %s: dest expand: %w", op, e.Src, err)
		}
		srcPath := filepath.Join(storeRoot, e.Src)
		srcInfo, srcErr := os.Stat(srcPath)
		destInfo, destErr := os.Stat(destPath)
		srcMissing := srcErr != nil && os.IsNotExist(srcErr)
		destMissing := destErr != nil && os.IsNotExist(destErr)
		if srcErr != nil && !srcMissing {
			return "", false, fmt.Errorf("%s %s: %w", op, e.Src, srcErr)
		}
		if destErr != nil && !destMissing {
			return "", false, fmt.Errorf("%s %s: %w", op, e.Src, destErr)
		}
		if srcMissing && destMissing {
			return "", false, fmt.Errorf("%s %s: %w", op, e.Src, srcErr)
		}
		if srcMissing {
			if !isPull {
				return "", false, fmt.Errorf("%s %s: %w", op, e.Src, srcErr)
			}
			if destInfo.IsDir() {
				return "", false, fmt.Errorf("%s %s: dest is a directory: %s", op, e.Src, destPath)
			}
			if stat {
				added, err := countFileLines(destPath)
				if err != nil {
					return "", false, fmt.Errorf("%s %s: %w", op, e.Src, err)
				}
				stats = append(stats, fileStat{label: e.Dest, added: added})
			} else {
				writeNewFileNotice(&sb, e.Src, e.Dest, "(new file: "+e.Src+" would be created in Store)\n", useColor)
				hasDiff = true
			}
			continue
		}
		if destMissing {
			if isPull {
				return "", false, fmt.Errorf("%s %s: %w", op, e.Src, destErr)
			}
			if srcInfo.IsDir() {
				return "", false, fmt.Errorf("%s %s: src is a directory: %s", op, e.Src, srcPath)
			}
			if stat {
				added, err := countFileLines(srcPath)
				if err != nil {
					return "", false, fmt.Errorf("%s %s: %w", op, e.Src, err)
				}
				stats = append(stats, fileStat{label: e.Src, added: added})
			} else {
				writeNewFileNotice(&sb, e.Dest, e.Src, "(new file: "+e.Dest+" would be created)\n", useColor)
				hasDiff = true
			}
			continue
		}
		if srcInfo.IsDir() {
			return "", false, fmt.Errorf("%s %s: src is a directory: %s", op, e.Src, srcPath)
		}
		if destInfo.IsDir() {
			return "", false, fmt.Errorf("%s %s: dest is a directory: %s", op, e.Src, destPath)
		}
		if !e.Override {
			continue
		}
		oldPath, newPath := destPath, srcPath
		oldLabel, newLabel := e.Dest, e.Src
		if isPull {
			oldPath, newPath = srcPath, destPath
			oldLabel, newLabel = e.Src, e.Dest
		}
		if stat {
			added, removed, same, err := fileChangeStats(oldPath, newPath)
			if err != nil {
				return "", false, fmt.Errorf("%s %s: %w", op, e.Src, err)
			}
			if !same {
				stats = append(stats, fileStat{label: newLabel, added: added, removed: removed})
			}
			continue
		}
		d, same, err := diffContent(oldPath, newPath, oldLabel, newLabel, colorMode)
		if err != nil {
			return "", false, fmt.Errorf("%s %s: %w", op, e.Src, err)
		}
		if !same {
			sb.WriteString(d)
			hasDiff = true
		}
	}
	if stat {
		if len(stats) == 0 {
			return "", false, nil
		}
		return renderStat(stats, colorMode), true, nil
	}
	return sb.String(), hasDiff, nil
}

// fileStat は stat 表示の1ファイル分の集計である。
type fileStat struct {
	label   string
	added   int
	removed int
}

// statBarMaxWidth は最大の変更量を持つファイルのバー幅である。
// 変更行数が多い場合は全ファイル共通の比率で縮小する。
const statBarMaxWidth = 50

// countFileLines は新規ファイルの追加行数として数える行数を返す。
func countFileLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return len(splitStatLines(string(data))), nil
}

// fileChangeStats は2ファイルの追加・削除行数を返す。同一内容なら same=true。
func fileChangeStats(oldPath, newPath string) (added, removed int, same bool, err error) {
	oldData, err := os.ReadFile(oldPath)
	if err != nil {
		return 0, 0, false, err
	}
	newData, err := os.ReadFile(newPath)
	if err != nil {
		return 0, 0, false, err
	}
	if bytes.Equal(oldData, newData) {
		return 0, 0, true, nil
	}
	added, removed = lineDiffCounts(splitStatLines(string(oldData)), splitStatLines(string(newData)))
	return added, removed, false, nil
}

// splitStatLines は行末改行を行に含めたまま分割する。
// "a\nb\n" は2行、"a\nb" も2行、空文字列は0行である。
func splitStatLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// lineDiffCounts は Myers の編集距離から追加・削除行数を求める。
func lineDiffCounts(old, new []string) (added, removed int) {
	n, m := len(old), len(new)
	if n == 0 {
		return m, 0
	}
	if m == 0 {
		return 0, n
	}
	max := n + m
	v := make([]int, 2*max+1)
	offset := max
	for d := 0; d <= max; d++ {
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && old[x] == new[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				removed = (d + n - m) / 2
				return d - removed, removed
			}
		}
	}
	return m, n
}

// renderStat はファイル別の変更行数と集計行を git diff --stat 風に返す。
func renderStat(stats []fileStat, colorMode string) string {
	useColor := resolveUseColor(colorMode)
	labelWidth, totalWidth, maxTotal := 0, 1, 0
	totalAdded, totalRemoved := 0, 0
	for _, s := range stats {
		if w := utf8.RuneCountInString(s.label); w > labelWidth {
			labelWidth = w
		}
		total := s.added + s.removed
		if w := len(strconv.Itoa(total)); w > totalWidth {
			totalWidth = w
		}
		if total > maxTotal {
			maxTotal = total
		}
		totalAdded += s.added
		totalRemoved += s.removed
	}

	var sb strings.Builder
	for _, s := range stats {
		total := s.added + s.removed
		added, removed := scaleStatBar(s.added, s.removed, maxTotal)
		bar := colorizeGreen(strings.Repeat("+", added), useColor) +
			colorizeRed(strings.Repeat("-", removed), useColor)
		padding := strings.Repeat(" ", labelWidth-utf8.RuneCountInString(s.label))
		fmt.Fprintf(&sb, " %s%s | %*d %s\n", s.label, padding, totalWidth, total, bar)
	}
	fmt.Fprintf(&sb, " %d %s changed", len(stats), pluralize(len(stats), "file"))
	if totalAdded > 0 {
		fmt.Fprintf(&sb, ", %d %s(+)", totalAdded, pluralize(totalAdded, "insertion"))
	}
	if totalRemoved > 0 {
		fmt.Fprintf(&sb, ", %d %s(-)", totalRemoved, pluralize(totalRemoved, "deletion"))
	}
	sb.WriteString("\n")
	return sb.String()
}

// scaleStatBar は最大の変更量を statBarMaxWidth に合わせ、全ファイル共通の比率で縮小する。
func scaleStatBar(added, removed, maxTotal int) (int, int) {
	if maxTotal <= statBarMaxWidth {
		return added, removed
	}
	a := added * statBarMaxWidth / maxTotal
	r := removed * statBarMaxWidth / maxTotal
	if added > 0 && a == 0 {
		a = 1
	}
	if removed > 0 && r == 0 {
		r = 1
	}
	for a+r > statBarMaxWidth {
		if a >= r && a > 1 {
			a--
		} else if r > 1 {
			r--
		} else {
			break
		}
	}
	return a, r
}

func pluralize(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// Pull は dest を Store の src へファイルコピーする。
// Entry の src は Store 相対、dest は ~ 展開される配置先パス。
// Push と同じく親ディレクトリは mkdir -p、パーミッションは元ファイルに追従。
// override=false で Store 側に既存ファイルがある場合は上書きせず skip して継続し、
// 不在時のみ新規作成する。省略時（true）は従来通り上書きする。
// dest が不在・ディレクトリの場合はエラーで中断する。Store 側がディレクトリの場合もエラーになる。
// skip 時は内容・権限を変更しない。戻り値は skip した Entry 群である。
func Pull(storeRoot string, entries []config.Entry) ([]config.Entry, error) {
	var skipped []config.Entry
	for _, e := range entries {
		destPath, err := config.ExpandDest(e.Dest)
		if err != nil {
			return skipped, fmt.Errorf("pull %s: dest expand: %w", e.Src, err)
		}
		srcPath := filepath.Join(storeRoot, e.Src)
		destInfo, destErr := os.Stat(destPath)
		if destErr != nil {
			return skipped, fmt.Errorf("pull %s: %w", e.Src, destErr)
		}
		if destInfo.IsDir() {
			return skipped, fmt.Errorf("pull %s: dest is a directory: %s", e.Src, destPath)
		}
		srcInfo, srcErr := os.Stat(srcPath)
		if srcErr != nil && !os.IsNotExist(srcErr) {
			return skipped, fmt.Errorf("pull %s: %w", e.Src, srcErr)
		}
		if srcErr == nil {
			if srcInfo.IsDir() {
				return skipped, fmt.Errorf("pull %s: src is a directory: %s", e.Src, srcPath)
			}
			if !e.Override {
				skipped = append(skipped, e)
				continue
			}
		}
		if err := copyFile(destPath, srcPath); err != nil {
			return skipped, fmt.Errorf("pull %s: %w", e.Src, err)
		}
	}
	return skipped, nil
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
