package main

import (
	"fmt"
	"os"
	"path/filepath"

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

func run(args []string, cwd string) int {
	if len(args) != 1 || args[0] != "push" {
		fmt.Fprintln(os.Stderr, "usage: mdots push")
		return 1
	}
	if err := runPush(cwd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runPush は common の Entry のみを Store から dest へコピーする。
func runPush(cwd string) error {
	store, err := config.FindStore(cwd)
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.yaml"))
	if err != nil {
		return err
	}
	entries := config.FilterByTarget(cfg.Entries, "")
	return sync.Push(store, entries)
}
