// Package app は mdots の実行系（Store解決・コピー・登録・差分）を束ねる。
//
// 表面の定義は cli パッケージに置き、副作用のある処理は本パッケージに寄せる。
// config/sync の詳細には触れず、入口の編成だけを行う。
package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mogurastore/mdots/internal/config"
	"github.com/mogurastore/mdots/internal/sync"
)

// Add は未登録の既存ファイルを新規Entryとして登録する。
// Store発見（カレント直下のみ）→dest正規化→Target解決→src算出→Load→意味検査→
// fragment生成→元ファイル追記→完成形再Load→atomic置換の順に行い、
// ファイルのコピーは行わない。回収は pull が行う。
// target 省略時は default_target へ登録し、明示時は指定 Target へ登録する。
// 省略時に default_target が空・欠落ならエラーにする（未定義を指す場合も新規作成として許容する）。
// 同一 Target 内の同一 dest 再登録はエラー、跨 Target の同一 dest は追記を許す。
// いずれかの検証で失敗したときは登録せず、mdots.tomlを変更しない。
// override は書かない（省略時 true として解決される）。
// 戻り値は配置先キー・Store相対src・解決先Targetである。
func Add(cwd, rawDest, target string) (string, string, string, error) {
	store, err := config.FindStore(cwd)
	if err != nil {
		return "", "", "", err
	}
	key, err := config.NormalizeDest(rawDest)
	if err != nil {
		return "", "", "", err
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.toml"))
	if err != nil {
		return "", "", "", err
	}
	resolved := target
	if resolved == "" {
		if cfg.DefaultTarget == "" {
			return "", "", "", fmt.Errorf("default_target is not set (hint: set default_target or use --target; check targets in mdots.toml)")
		}
		resolved = cfg.DefaultTarget
	}
	src, err := config.SrcForDestWithTarget(key, resolved)
	if err != nil {
		return "", "", "", err
	}
	destPath, err := config.ExpandDest(key)
	if err != nil {
		return "", "", "", err
	}
	destInfo, destErr := os.Stat(destPath)
	if destErr != nil {
		return "", "", "", fmt.Errorf("add %s: %w", key, destErr)
	}
	if destInfo.IsDir() {
		return "", "", "", fmt.Errorf("add %s: dest is a directory: %s", key, destPath)
	}
	srcPath := filepath.Join(store, src)
	if srcInfo, srcErr := os.Stat(srcPath); srcErr == nil {
		if srcInfo.IsDir() {
			return "", "", "", fmt.Errorf("add %s: src is a directory: %s", key, src)
		}
		return "", "", "", fmt.Errorf("add %s: src already exists in Store: %s", key, src)
	} else if !os.IsNotExist(srcErr) {
		return "", "", "", fmt.Errorf("add %s: %w", key, srcErr)
	}
	if err := cfg.AddTarget(key, resolved, src); err != nil {
		return "", "", "", err
	}
	if err := config.AppendTargetEntry(filepath.Join(store, "mdots.toml"), key, resolved, src); err != nil {
		return "", "", "", err
	}
	return key, src, resolved, nil
}

// resolveEntries は Store 発見・設定読込・Target 解決をまとめて行い、
// push/pull で共有する。解決は単一 Target のみで、省略時は default_target へ解決する
// （配置先ソート順）。未定義名・欠落した default_target はエラーにする。
func resolveEntries(cwd string, target string) (string, []config.Entry, error) {
	store, err := config.FindStore(cwd)
	if err != nil {
		return "", nil, err
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.toml"))
	if err != nil {
		return "", nil, err
	}
	entries, err := cfg.Resolve(target)
	if err != nil {
		return "", nil, err
	}
	return store, entries, nil
}

// reportSkipped は override=false による skip を警告として報告する。
// 全体は成功（exit 0）で終える。報告先は呼び出し元から注入された stderr とする。
// 文面は CONTEXT.md の override 用語に従い overwrite/force を避ける。
func reportSkipped(stderr io.Writer, skipped []config.Entry, op string) {
	for _, s := range skipped {
		fmt.Fprintf(stderr, "skipped: %s (override=false, %s would not override)\n", s.Dest, op)
	}
}

// reportCopied はコピー成功を stdout に報告する。方向に合わせて src/dest の順を変える。
// push は Store src -> dest、pull は dest -> Store src である。
// コピー0件（該当なし・全skip）は No changes. を出す。
func reportCopied(stdout io.Writer, entries []config.Entry, skipped []config.Entry, op string) {
	skippedSet := make(map[string]struct{}, len(skipped))
	for _, s := range skipped {
		skippedSet[s.Dest] = struct{}{}
	}
	copied := 0
	for _, e := range entries {
		if _, ok := skippedSet[e.Dest]; ok {
			continue
		}
		copied++
		if op == "push" {
			fmt.Fprintf(stdout, "copied %s -> %s\n", e.Src, e.Dest)
		} else {
			fmt.Fprintf(stdout, "copied %s -> %s\n", e.Dest, e.Src)
		}
	}
	if copied == 0 {
		fmt.Fprintln(stdout, "No changes.")
	}
}

// runCopy は push/pull のコピー系の共有本体である。方向の違いは copyFn と op に寄せ、
// 各 action（Push/Pull）は薄い委譲に留める。
// 成功分は stdout に報告し、skip は警告として stderr に報告し、全体は成功で終える。
func runCopy(cwd string, target string, op string, stdout, stderr io.Writer, copyFn func(string, []config.Entry) ([]config.Entry, error)) error {
	store, entries, err := resolveEntries(cwd, target)
	if err != nil {
		return err
	}
	skipped, err := copyFn(store, entries)
	if err != nil {
		return err
	}
	reportCopied(stdout, entries, skipped, op)
	reportSkipped(stderr, skipped, op)
	return nil
}

// Push は単一 Target の Entry を Store から dest へコピーする。
// target 省略時は default_target へ解決する。未定義名・欠落した default_target はエラーにする。
// override=false の既存 dest は保護して skip 継続し、警告を stderr に出す。
// 成功分は stdout に copied <src> -> <dest> を出す。
func Push(cwd string, target string, stdout, stderr io.Writer) error {
	return runCopy(cwd, target, "push", stdout, stderr, sync.Push)
}

// Init はカレント直下に mdots.toml 雛形を作る。
// 既にあるときは config.Init が already exists エラーを返す。
func Init(cwd string) error {
	_, err := config.Init(cwd)
	return err
}

// Sort は Store の mdots.toml を正規形にソートする。
// 意味ソート（Load→正規形再生成）のためコメントは落とす。
// 既に正規形なら書き換えない。成功時は無言で終える。
func Sort(cwd string) error {
	store, err := config.FindStore(cwd)
	if err != nil {
		return err
	}
	_, err = config.SortFile(filepath.Join(store, "mdots.toml"))
	return err
}

// Targets は定義済みTarget名を重複排除・ソートし、既定 Target に印を付けて返す。
// 印の書式は "<名> (default)" とする。Store不在・toml不正時はエラーを返す。
func Targets(cwd string) ([]string, error) {
	store, err := config.FindStore(cwd)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.toml"))
	if err != nil {
		return nil, err
	}
	return cfg.TargetsMarked(), nil
}

// Pull は単一 Target の Entry を dest から Store へ回収する。
// target 省略時は default_target へ解決する。未定義名・欠落した default_target はエラーにする。
// override=false の既存 Store は保護して skip 継続し、警告を stderr に出す。
// 成功分は stdout に copied <dest> -> <src> を出す。
func Pull(cwd string, target string, stdout, stderr io.Writer) error {
	return runCopy(cwd, target, "pull", stdout, stderr, sync.Pull)
}

// emitDiff は差分出力と exit 対応を一本化する。差分ありは出力して 1、なしは No changes. を出して 0。
func emitDiff(stdout io.Writer, out string, hasDiff bool) int {
	if hasDiff {
		fmt.Fprint(stdout, out)
		return 1
	}
	fmt.Fprintln(stdout, "No changes.")
	return 0
}

// PushDryRun は push の差分を stdout に出し、書き込みは行わない。
// target 省略時は default_target へ解決する。差分ありは exit 1、差分なしは exit 0。
func PushDryRun(cwd string, target, color string, stdout, stderr io.Writer) int {
	store, entries, err := resolveEntries(cwd, target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	out, hasDiff, err := sync.PushDryRun(store, entries, color)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return emitDiff(stdout, out, hasDiff)
}

// PullDryRun は pull の差分を stdout に出し、書き込みは行わない。
// target 省略時は default_target へ解決する。差分ありは exit 1、差分なしは exit 0。
func PullDryRun(cwd string, target, color string, stdout, stderr io.Writer) int {
	store, entries, err := resolveEntries(cwd, target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	out, hasDiff, err := sync.PullDryRun(store, entries, color)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return emitDiff(stdout, out, hasDiff)
}
