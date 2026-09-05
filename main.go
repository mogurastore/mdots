package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

const usage = "usage: mdots <push|pull|diff> [--target <name>]"

const globalHelp = `usage: mdots <push|pull|diff> [options]

dotfilesをファイルコピー（非symlink）で管理するCLI。
Store（mdots.yaml を含む管理リポジトリのルート）をカレントから親方向に探索する。

commands:
  push  Storeからdestへファイルをコピーする
  pull  destからStoreへファイルを回収する
  diff  Storeとdestの差分を表示する

global options:
  -h, --help     使い方を表示する
  --version      バージョンを表示する

run 'mdots <command> --help' for command-specific help.
`

const pushHelp = `usage: mdots push [--target <name>] [--dry-run]

Storeからdestへファイルをコピーする。
--target 未指定時はcommonのみ、指定時は common + 指定Target が対象。

options:
  --target <name>  対象Target (例: win, wsl)。--target=<name> 形式も可
  --dry-run        実際に書き込まず差分相当を出力する。差分ありは exit 1
  -h, --help       使い方を表示する
`

const pullHelp = `usage: mdots pull [--target <name>] [--dry-run]

destからStoreへファイルを回収する。
--target 未指定時はcommonのみ、指定時は common + 指定Target が対象。

options:
  --target <name>  対象Target (例: win, wsl)。--target=<name> 形式も可
  --dry-run        実際に書き込まず差分相当を出力する。差分ありは exit 1
  -h, --help       使い方を表示する
`

const diffHelp = `usage: mdots diff [--target <name>]

Storeとdestの差分を diff -u 風に出力する。
差分なしは無出力・exit 0、差分ありは差分を出力し exit 1。
--target 未指定時はcommonのみ、指定時は common + 指定Target が対象。

options:
  --target <name>  対象Target (例: win, wsl)。--target=<name> 形式も可
  -h, --help       使い方を表示する
`

func commandHelp(cmd string) string {
	switch cmd {
	case "push":
		return pushHelp
	case "pull":
		return pullHelp
	case "diff":
		return diffHelp
	default:
		return globalHelp
	}
}

func run(args []string, cwd string) int {
	return runWithWriters(args, cwd, os.Stdout, os.Stderr)
}

func runWithWriters(args []string, cwd string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, globalHelp)
		return 1
	}
	switch args[0] {
	case "--help", "-h":
		fmt.Fprint(stdout, globalHelp)
		return 0
	case "--version", "-V", "version":
		fmt.Fprintf(stdout, "mdots %s\n", version)
		return 0
	}
	cmd := args[0]
	if cmd != "push" && cmd != "pull" && cmd != "diff" {
		fmt.Fprintf(stderr, "unknown command: %s\n", cmd)
		fmt.Fprint(stderr, globalHelp)
		return 1
	}
	allowDryRun := cmd == "push" || cmd == "pull"
	opts, err := parseCmdArgs(args[1:], allowDryRun)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprint(stderr, commandHelp(cmd))
		return 1
	}
	if opts.help {
		fmt.Fprint(stdout, commandHelp(cmd))
		return 0
	}
	switch cmd {
	case "push":
		if opts.dryRun {
			return runPushDryRun(cwd, opts.target, stdout, stderr)
		}
		if err := runPush(cwd, opts.target); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "pull":
		if opts.dryRun {
			return runPullDryRun(cwd, opts.target, stdout, stderr)
		}
		if err := runPull(cwd, opts.target); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "diff":
		out, hasDiff, err := runDiff(cwd, opts.target)
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
	return 1
}

// cmdOptions は push/pull/diff のコマンドフラグを表す。
type cmdOptions struct {
	target string
	dryRun bool
	help   bool
}

// parseCmdArgs は push/pull/diff のフラグを解釈する。
// `--target <name>` と `--target=<name>`、`--dry-run`、`-h/--help` を受け付ける。
// diff では --dry-run を拒否する (allowDryRun=false)。
func parseCmdArgs(args []string, allowDryRun bool) (cmdOptions, error) {
	var opts cmdOptions
	seen := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--target":
			if i+1 >= len(args) {
				return cmdOptions{}, fmt.Errorf("missing value for --target")
			}
			opts.target = args[i+1]
			i++
			seen = true
		case strings.HasPrefix(a, "--target="):
			opts.target = strings.TrimPrefix(a, "--target=")
			seen = true
		case a == "--dry-run":
			if !allowDryRun {
				return cmdOptions{}, fmt.Errorf("--dry-run is only supported for push/pull")
			}
			opts.dryRun = true
		case a == "--help" || a == "-h":
			opts.help = true
		default:
			return cmdOptions{}, fmt.Errorf("unknown argument: %s", a)
		}
	}
	if seen && opts.target == "" {
		return cmdOptions{}, fmt.Errorf("missing value for --target")
	}
	return opts, nil
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
