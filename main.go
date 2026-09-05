package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mogurastore/mdots/cli"
	"github.com/mogurastore/mdots/config"
	"github.com/mogurastore/mdots/sync"
)

// version はリリース時に ldflags -X main.version=<tag> で埋め込む。
// 未指定時の既定値は dev。
var version = "dev"

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], cwd))
}

func run(args []string, cwd string) int {
	return runWithWriters(args, cwd, os.Stdout, os.Stderr)
}

// runWithWriters は CLI の薄い入口である。表面の定義は cli パッケージに置き、
// 内部実行は既存ロジックに委譲する。
func runWithWriters(args []string, cwd string, stdout, stderr io.Writer) int {
	return cli.Run(args, cwd, version, cliExecutor{}, stdout, stderr)
}

// cliExecutor は cli.Executor への委譲口である。実行系の本体は本ファイルの
// runPush/runPull/runDiff 系に残し、コマンド骨格だけを cli パッケージに置く。
type cliExecutor struct{}

func (cliExecutor) Push(cwd, target string) error { return runPush(cwd, target) }

func (cliExecutor) Pull(cwd, target string) error { return runPull(cwd, target) }

func (cliExecutor) Diff(cwd, target string) (string, bool, error) { return runDiff(cwd, target) }

func (cliExecutor) PushDryRun(cwd, target string, stdout, stderr io.Writer) int {
	return runPushDryRun(cwd, target, stdout, stderr)
}

func (cliExecutor) PullDryRun(cwd, target string, stdout, stderr io.Writer) int {
	return runPullDryRun(cwd, target, stdout, stderr)
}

// resolveEntries は Store 発見・設定読込・Target フィルタをまとめて行い、
// push/pull/diff とその dry-run で共有する。
func resolveEntries(cwd string, target string) (string, []config.Entry, error) {
	store, err := config.FindStore(cwd)
	if err != nil {
		return "", nil, err
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.yaml"))
	if err != nil {
		return "", nil, err
	}
	return store, config.FilterByTarget(cfg.Entries, target), nil
}

// runPush は common + 指定Target の Entry を Store から dest へコピーする。
// target 未指定時は common のみが対象になる。
func runPush(cwd string, target string) error {
	store, entries, err := resolveEntries(cwd, target)
	if err != nil {
		return err
	}
	return sync.Push(store, entries)
}

// runPull は common + 指定Target の Entry を dest から Store へ回収する。
// target 未指定時は common のみが対象になる。
func runPull(cwd string, target string) error {
	store, entries, err := resolveEntries(cwd, target)
	if err != nil {
		return err
	}
	return sync.Pull(store, entries)
}

// runDiff は common + 指定Target の Entry について Store/src と dest の
// 差分を diff -u 風の文字列で返す。差分があれば hasDiff=true。
// target 未指定時は common のみが対象になる。
func runDiff(cwd string, target string) (string, bool, error) {
	store, entries, err := resolveEntries(cwd, target)
	if err != nil {
		return "", false, err
	}
	return sync.Diff(store, entries)
}

// runPushDryRun は push の差分相当を stdout に出し、書き込みは行わない。
// 差分ありは exit 1、差分なしは exit 0。
func runPushDryRun(cwd string, target string, stdout, stderr io.Writer) int {
	store, entries, err := resolveEntries(cwd, target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	out, hasDiff, err := sync.DryRunPush(store, entries)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if hasDiff {
		fmt.Fprint(stdout, out)
		return 1
	}
	return 0
}

// runPullDryRun は pull の差分相当を stdout に出し、書き込みは行わない。
// 差分ありは exit 1、差分なしは exit 0。
func runPullDryRun(cwd string, target string, stdout, stderr io.Writer) int {
	store, entries, err := resolveEntries(cwd, target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	out, hasDiff, err := sync.DryRunPull(store, entries)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if hasDiff {
		fmt.Fprint(stdout, out)
		return 1
	}
	return 0
}
