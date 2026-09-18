package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mogurastore/mdots/config"
	gd "github.com/amterp/go-delta"
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

func colorizeBold(s string, useColor bool) string {
	if !useColor {
		return s
	}
	return "\x1b[1m" + s + "\x1b[0m"
}

func colorizeYellow(s string, useColor bool) string {
	if !useColor {
		return s
	}
	return "\x1b[33m" + s + "\x1b[0m"
}

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

// PushDryRun は push のプレビューを dest→Store 方向で返す。書き込みは行わない。
// pushで追加される行が+になるよう dest を old、Store を new とする。
// 両方存在する Entry は inline diff を出し、片方不在の Entry は新規作成予定として報告する。
// override=false でコピー先が存在する Entry は無出力・差分なし扱いとし、skip 報告はしない。
// 両方不在・ディレクトリはエラーで中断する。
func PushDryRun(storeRoot string, entries []config.Entry, colorMode string) (string, bool, error) {
	return diffDryRun(storeRoot, entries, colorMode, "push", false)
}

// PullDryRun は pull のプレビューを Store→dest 方向で返す。書き込みは行わない。
// pullで取り込まれる行が+になるよう Store を old、dest を new とする。
// override=false でコピー先が存在する Entry は無出力・差分なし扱いとし、skip 報告はしない。
func PullDryRun(storeRoot string, entries []config.Entry, colorMode string) (string, bool, error) {
	return diffDryRun(storeRoot, entries, colorMode, "pull", true)
}

// diffDryRun は push/pull の差分コアである。isPull=false は dest を old・
// Store を new とし、true は Store を old・dest を new とする。
func diffDryRun(storeRoot string, entries []config.Entry, colorMode string, op string, isPull bool) (string, bool, error) {
	useColor := resolveUseColor(colorMode)
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
			writeNewFileNotice(&sb, e.Src, e.Dest, "(new file: "+e.Src+" would be created in Store)\n", useColor)
			hasDiff = true
			continue
		}
		if destMissing {
			if isPull {
				return "", false, fmt.Errorf("%s %s: %w", op, e.Src, destErr)
			}
			if srcInfo.IsDir() {
				return "", false, fmt.Errorf("%s %s: src is a directory: %s", op, e.Src, srcPath)
			}
			writeNewFileNotice(&sb, e.Dest, e.Src, "(new file: "+e.Dest+" would be created)\n", useColor)
			hasDiff = true
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
		var d string
		var same bool
		if isPull {
			d, same, err = diffContent(srcPath, destPath, e.Src, e.Dest, colorMode)
		} else {
			d, same, err = diffContent(destPath, srcPath, e.Dest, e.Src, colorMode)
		}
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
