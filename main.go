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

const pushUsage = "usage: mdots push [--target <name>]"

func run(args []string, cwd string) int {
	if len(args) == 0 || args[0] != "push" {
		fmt.Fprintln(os.Stderr, pushUsage)
		return 1
	}
	target, err := parsePushArgs(args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, pushUsage)
		return 1
	}
	if err := runPush(cwd, target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// parsePushArgs は push の --target フラグを解釈する。
// `--target <name>` と `--target=<name>` を受け付ける。
func parsePushArgs(args []string) (string, error) {
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
