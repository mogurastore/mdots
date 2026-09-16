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
// runPush/runPull 系に残し、コマンド骨格だけを cli パッケージに置く。
type cliExecutor struct{}

func (cliExecutor) Push(cwd, target string) error { return runPush(cwd, target) }

func (cliExecutor) Pull(cwd, target string) error { return runPull(cwd, target) }

func (cliExecutor) PushDryRun(cwd, target, color string, stdout, stderr io.Writer) int {
	return runPushDryRun(cwd, target, color, stdout, stderr)
}

func (cliExecutor) PullDryRun(cwd, target, color string, stdout, stderr io.Writer) int {
	return runPullDryRun(cwd, target, color, stdout, stderr)
}

func (cliExecutor) Init(cwd string) error { return runInit(cwd) }

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
// 全体は成功（exit 0）で終える。報告先は既存のエラー出力流儀に合わせ stderr とする。
// 文面は CONTEXT.md の override 用語に従い overwrite/force を避ける。
func reportSkipped(skipped []config.Entry, op string) {
	for _, s := range skipped {
		fmt.Fprintf(os.Stderr, "skipped: %s (override=false, %s would not override)\n", s.Dest, op)
	}
}

// runCopy は push/pull のコピー系の共有本体である。方向の違いは copyFn と op に寄せ、
// 各 action（runPush/runPull）は薄い委譲に留める。
// skip は警告として stderr に報告し、全体は成功で終える。
func runCopy(cwd string, target string, op string, copyFn func(string, []config.Entry) ([]config.Entry, error)) error {
	store, entries, err := resolveEntries(cwd, target)
	if err != nil {
		return err
	}
	skipped, err := copyFn(store, entries)
	if err != nil {
		return err
	}
	reportSkipped(skipped, op)
	return nil
}

// runPush は指定なし＋指定Target の Entry を Store から dest へコピーする。
// target 未指定時は指定なし Entry のみが対象になる。
// override=false の既存 dest は保護して skip 継続し、警告を stderr に出す。
func runPush(cwd string, target string) error {
	return runCopy(cwd, target, "push", sync.Push)
}

// runInit はカレント直下に mdots.toml 雛形を作る。
// 既にあるときは config.Init が already exists エラーを返す。
func runInit(cwd string) error {
	_, err := config.Init(cwd)
	return err
}

// runPull は指定なし＋指定Target の Entry を dest から Store へ回収する。
// target 未指定時は指定なし Entry のみが対象になる。
// override=false の既存 Store は保護して skip 継続し、警告を stderr に出す。
func runPull(cwd string, target string) error {
	return runCopy(cwd, target, "pull", sync.Pull)
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

// runPushDryRun は push の差分を stdout に出し、書き込みは行わない。
// 差分ありは exit 1、差分なしは exit 0。
func runPushDryRun(cwd string, target, color string, stdout, stderr io.Writer) int {
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

// runPullDryRun は pull の差分を stdout に出し、書き込みは行わない。
// 差分ありは exit 1、差分なしは exit 0。
func runPullDryRun(cwd string, target, color string, stdout, stderr io.Writer) int {
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
