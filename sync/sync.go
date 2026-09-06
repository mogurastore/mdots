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

// missingPolicy は差分コアにおける欠落時の扱いを表す。
// 出力・exitの振る舞いは変えず、欠落時ポリシーの違いのみを引数化する。
type missingPolicy int

const (
	// policyPushDryRun は push の dry-run 用：dest 不在を新規作成予定として報告する。
	policyPushDryRun missingPolicy = iota
	// policyPullDryRun は pull の dry-run 用：Store の src 不在を新規回収予定として報告する。
	policyPullDryRun
)

// opLabel は差分コアのエラー接頭辞をポリシーから導く。
func (p missingPolicy) opLabel() string {
	switch p {
	case policyPushDryRun:
		return "dry-run push"
	default:
		return "dry-run pull"
	}
}

// ColorAuto etc は dry-run の --color 値を表す。auto は FORCE_COLOR > NO_COLOR > tty 判定。
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

// diffCore は dry-run 系の単一の差分コアである。
// 両方存在する Entry は go-delta による inline diff を出し、不在の扱いだけを policy で切り替える。
func diffCore(storeRoot string, entries []config.Entry, policy missingPolicy, colorMode string) (string, bool, error) {
	op := policy.opLabel()
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
				writeNewFileNotice(&sb, e.Src, e.Dest, "(new file: "+e.Dest+" would be created)\n", useColor)
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
				writeNewFileNotice(&sb, e.Src, e.Dest, "(new file: "+e.Src+" would be created in Store)\n", useColor)
				hasDiff = true
				continue
			}
			if srcInfo.IsDir() {
				return "", false, fmt.Errorf("%s %s: Store src is a directory: %s", op, e.Src, srcPath)
			}
		}
		d, same, err := diffContent(srcPath, destPath, e.Src, e.Dest, colorMode)
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

// DryRunPush は push の差分相当を返す。実際の書き込みは行わない。
// 両方存在する Entry は inline diff を出し、
// dest 不在の Entry は新規作成予定として報告する。
// src 不在・ディレクトリは push と同様にエラーで中断する。
// 実体は単一の差分コアへの薄い委譲である。
func DryRunPush(storeRoot string, entries []config.Entry) (output string, hasDiff bool, err error) {
	return diffCore(storeRoot, entries, policyPushDryRun, ColorAuto)
}

// DryRunPushWithColor は色指定付きの push 差分相当を返す。
func DryRunPushWithColor(storeRoot string, entries []config.Entry, colorMode string) (output string, hasDiff bool, err error) {
	return diffCore(storeRoot, entries, policyPushDryRun, colorMode)
}

// DryRunPull は pull の差分相当を返す。実際の書き込みは行わない。
// 両方存在する Entry は inline diff を出し、
// Store の src 不在の Entry は新規回収予定として報告する。
// dest 不在・ディレクトリは pull と同様にエラーで中断する。
// 実体は単一の差分コアへの薄い委譲である。
func DryRunPull(storeRoot string, entries []config.Entry) (output string, hasDiff bool, err error) {
	return diffCore(storeRoot, entries, policyPullDryRun, ColorAuto)
}

// DryRunPullWithColor は色指定付きの pull 差分相当を返す。
func DryRunPullWithColor(storeRoot string, entries []config.Entry, colorMode string) (output string, hasDiff bool, err error) {
	return diffCore(storeRoot, entries, policyPullDryRun, colorMode)
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
