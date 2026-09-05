package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mogurastore/mdots/config"
	"github.com/mogurastore/mdots/sync"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], cwd))
}

const usage = "usage: mdots <push|pull> [--target <name>]"

func run(args []string, cwd string) int {
	if len(args) == 0 || (args[0] != "push" && args[0] != "pull") {
		fmt.Fprintln(os.Stderr, usage)
		return 1
	}
	cmd := args[0]
	target, err := parseTargetArgs(args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usage)
		return 1
	}
	var runErr error
	if cmd == "push" {
		runErr = runPush(cwd, target)
	} else {
		runErr = runPull(cwd, target)
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		return 1
	}
	return 0
}

// parseTargetArgs は push/pull の --target フラグを解釈する。
// `--target <name>` と `--target=<name>` を受け付ける。
func parseTargetArgs(args []string) (string, error) {
	var target string
	seen := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--target":
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for --target")
			}
			target = args[i+1]
			i++
			seen = true
		case strings.HasPrefix(a, "--target="):
			target = strings.TrimPrefix(a, "--target=")
			seen = true
		default:
			return "", fmt.Errorf("unknown argument: %s", a)
		}
	}
	if seen && target == "" {
		return "", fmt.Errorf("missing value for --target")
	}
	return target, nil
}

// runPush は common + 指定Target の Entry を Store から dest へコピーする。
// target 未指定時は common のみが対象になる。
func runPush(cwd string, target string) error {
	store, err := config.FindStore(cwd)
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.yaml"))
	if err != nil {
		return err
	}
	entries := config.FilterByTarget(cfg.Entries, target)
	return sync.Push(store, entries)
}

// runPull は common + 指定Target の Entry を dest から Store へ回収する。
// target 未指定時は common のみが対象になる。
func runPull(cwd string, target string) error {
	store, err := config.FindStore(cwd)
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.yaml"))
	if err != nil {
		return err
	}
	entries := config.FilterByTarget(cfg.Entries, target)
	return sync.Pull(store, entries)
}
