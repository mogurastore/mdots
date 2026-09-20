package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mogurastore/mdots/internal/cli"
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

// runWithWriters は CLI の薄い入口である。表面の定義と実行系は
// internal/cli・internal/app パッケージに置き、本ファイルは組立と Run のみに留める。
func runWithWriters(args []string, cwd string, stdout, stderr io.Writer) int {
	return cli.Run(args, cwd, version, stdout, stderr)
}
