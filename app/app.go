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

	"github.com/mogurastore/mdots/config"
	"github.com/mogurastore/mdots/sync"
)

// Add は未登録の既存ファイルを新規Entryとして登録する。
// Store発見（カレント直下のみ）→dest正規化→src算出→Load→意味検査→
// fragment生成→元ファイル追記→完成形再Load→atomic置換の順に行い、
// ファイルのコピーは行わない。回収は pull が行う。
// target 指定時は dotfiles/<target>/... に写像し、targets形式で登録する。
// いずれかの検証で失敗したときは登録せず、mdots.tomlを変更しない。
// 素のsrc済み・同一Targetの再登録はエラーとする。
// targets形式で別Targetが未登録なら追記マージする。
func Add(cwd, rawDest, target string) (string, string, error) {
	store, err := config.FindStore(cwd)
	if err != nil {
		return "", "", err
	}
	key, err := config.NormalizeDest(rawDest)
	if err != nil {
		return "", "", err
	}
	src, err := config.SrcForDestWithTarget(key, target)
	if err != nil {
		return "", "", err
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.toml"))
	if err != nil {
		return "", "", err
	}
	destPath, err := config.ExpandDest(key)
	if err != nil {
		return "", "", err
	}
	destInfo, destErr := os.Stat(destPath)
	if destErr != nil {
		return "", "", fmt.Errorf("add %s: %w", key, destErr)
	}
	if destInfo.IsDir() {
		return "", "", fmt.Errorf("add %s: dest is a directory: %s", key, destPath)
	}
	srcPath := filepath.Join(store, src)
	if srcInfo, srcErr := os.Stat(srcPath); srcErr == nil {
		if srcInfo.IsDir() {
			return "", "", fmt.Errorf("add %s: src is a directory: %s", key, src)
		}
		return "", "", fmt.Errorf("add %s: src already exists in Store: %s", key, src)
	} else if !os.IsNotExist(srcErr) {
		return "", "", fmt.Errorf("add %s: %w", key, srcErr)
	}
	if target == "" {
		if err := cfg.Add(key, src); err != nil {
			return "", "", err
		}
		if err := config.AppendPlainEntry(filepath.Join(store, "mdots.toml"), key, src); err != nil {
			return "", "", err
		}
	} else {
		if err := cfg.AddTarget(key, target, src); err != nil {
			return "", "", err
		}
		if err := config.AppendTargetEntry(filepath.Join(store, "mdots.toml"), key, target, src); err != nil {
			return "", "", err
		}
	}
	return key, src, nil
}

// resolveEntries は Store 発見・設定読込・Target 解決をまとめて行い、
// push/pull で共有する。解決規則は config.Resolve に寄せる
// （指定なしは常時＋一致Targetのみ、配置先ソート順）。
func resolveEntries(cwd string, target string) (string, []config.Entry, error) {
	store, err := config.FindStore(cwd)
	if err != nil {
		return "", nil, err
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.toml"))
	if err != nil {
		return "", nil, err
	}
	return store, cfg.Resolve(target), nil
}

// reportSkipped は override=false による skip を警告として報告する。
// 全体は成功（exit 0）で終える。報告先は呼び出し元から注入された stderr とする。
// 文面は CONTEXT.md の override 用語に従い overwrite/force を避ける。
func reportSkipped(stderr io.Writer, skipped []config.Entry, op string) {
	for _, s := range skipped {
		fmt.Fprintf(stderr, "skipped: %s (override=false, %s would not override)\n", s.Dest, op)
	}
}

// runCopy は push/pull のコピー系の共有本体である。方向の違いは copyFn と op に寄せ、
// 各 action（Push/Pull）は薄い委譲に留める。
// skip は警告として stderr に報告し、全体は成功で終える。
func runCopy(cwd string, target string, op string, stderr io.Writer, copyFn func(string, []config.Entry) ([]config.Entry, error)) error {
	store, entries, err := resolveEntries(cwd, target)
	if err != nil {
		return err
	}
	skipped, err := copyFn(store, entries)
	if err != nil {
		return err
	}
	reportSkipped(stderr, skipped, op)
	return nil
}

// Push は指定なし＋指定Target の Entry を Store から dest へコピーする。
// target 未指定時は指定なし Entry のみが対象になる。
// override=false の既存 dest は保護して skip 継続し、警告を stderr に出す。
func Push(cwd string, target string, stderr io.Writer) error {
	return runCopy(cwd, target, "push", stderr, sync.Push)
}

// Init はカレント直下に mdots.toml 雛形を作る。
// 既にあるときは config.Init が already exists エラーを返す。
func Init(cwd string) error {
	_, err := config.Init(cwd)
	return err
}

// Targets は定義済みTarget名を重複排除・ソートして返す。
// Store不在・toml不正時はエラーを返す。
func Targets(cwd string) ([]string, error) {
	store, err := config.FindStore(cwd)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.toml"))
	if err != nil {
		return nil, err
	}
	return cfg.Targets(), nil
}

// Pull は指定なし＋指定Target の Entry を dest から Store へ回収する。
// target 未指定時は指定なし Entry のみが対象になる。
// override=false の既存 Store は保護して skip 継続し、警告を stderr に出す。
func Pull(cwd string, target string, stderr io.Writer) error {
	return runCopy(cwd, target, "pull", stderr, sync.Pull)
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
// 差分ありは exit 1、差分なしは exit 0。
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
// 差分ありは exit 1、差分なしは exit 0。
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
