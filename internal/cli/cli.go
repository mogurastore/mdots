// Package cli は mdots のコマンド骨格を定義する。
//
// CLIフレームワークの最終採用は urfave/cli v3（宣言ツリーで push/pull と
// --target/--dry-run/--color を定義する）。
//
// CLI表面（help/version/unknown/usage時の文面・exit）は枠組み既定に寄せる。
// 内部実行は app 直接呼出とし、エントリは薄く保つ.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mogurastore/mdots/internal/app"
	cliv3 "github.com/urfave/cli/v3"
	"golang.org/x/term"
)

// exitError は Action が呼び出し元 Run へ exit code を伝えるための内用エラーで、
// 出力は Action 側で済ませているため文面を持たない。
type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

// runner は1回の Run 呼び出しに対応する宣言ツリーと入出力を束ねる。
type runner struct {
	version string
	cwd     string
	stdout  io.Writer
	stderr  io.Writer
	stdin   io.Reader
	isTTY   func() bool
}

// Run は CLI表面の入口である。args は os.Args[1:] 相当、cwd は実行ディレクトリ。
// 戻り値はプロセスの exit code。
// フラグ解釈・help/version/unknown の表面は枠組み既定に任せ、
// 空値拒否は Flag.Validator、余剰引数拒否は Before に寄せる。
// 位置引数の不足・--color依存など本質検査だけを Action 側に残す。
func Run(args []string, cwd, version string, stdout, stderr io.Writer) int {
	r := &runner{
		version: version,
		cwd:     cwd,
		stdout:  stdout,
		stderr:  stderr,
		stdin:   os.Stdin,
		isTTY:   func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
	}

	return r.execute(args)
}

// RunWithStdin はテスト用に入力と端末判定を注入する入口である。
func RunWithStdin(args []string, cwd, version string, stdout, stderr io.Writer, stdin io.Reader, isTTY func() bool) int {
	if stdin == nil {
		stdin = os.Stdin
	}
	if isTTY == nil {
		isTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
	}
	r := &runner{version: version, cwd: cwd, stdout: stdout, stderr: stderr, stdin: stdin, isTTY: isTTY}
	return r.execute(args)
}

func (r *runner) execute(args []string) int {
	cmd := r.newCommand()
	if err := cmd.Run(context.Background(), append([]string{"mdots"}, args...)); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			return ee.code
		}
		var ec cliv3.ExitCoder
		if errors.As(err, &ec) {
			// unknown command 時の既定（No help topic、exit 3）。
			// 枠組み既定の ExitErrHandler はグローバル ErrWriter に出すため、
			// 注入先 stderr に出し直してテスト同一プロセス性を保つ。
			fmt.Fprintln(r.stderr, ec.Error())
			return ec.ExitCode()
		}
		// フラグ解釈失敗時の既定（Incorrect Usage＋help）は枠組み側で
		// 注入先に表示済みのため、ここでは文面を出さず exit 1 にする。
		return 1
	}
	return 0
}

// targetFlag は --target/-t の宣言である。--target <name> と --target=<name> の
// 両形式はフレームワークが解釈する。空値 (--target=) は Validator で拒否し、
// 枠組み既定の Incorrect Usage＋help になる。
func targetFlag() cliv3.Flag {
	return &cliv3.StringFlag{
		Name:    "target",
		Aliases: []string{"t"},
		Usage:   "対象Target (例: win, wsl)。--target=<name> 形式も可",
		Validator: func(v string) error {
			if v == "" {
				return fmt.Errorf("missing value for --target")
			}
			return nil
		},
	}
}

// colorFlag は --color/-c の宣言である。push/pull の dry-run 時の着色制御で auto|always|never を取る。
// 値域検査は Validator に移譲し、枠組み既定の Incorrect Usage＋help になる。
func colorFlag() cliv3.Flag {
	return &cliv3.StringFlag{
		Name:    "color",
		Aliases: []string{"c"},
		Value:   "auto",
		Usage:   "差分の着色 (auto|always|never)",
		Validator: func(v string) error {
			switch v {
			case "auto", "always", "never":
				return nil
			default:
				return fmt.Errorf("invalid value for --color: %s (want auto|always|never)", v)
			}
		},
	}
}

// dryRunFlag は --dry-run/-n の宣言である。push/pull で書き込まず差分を出力する。
func dryRunFlag() cliv3.Flag {
	return &cliv3.BoolFlag{Name: "dry-run", Aliases: []string{"n"}, Usage: "書き込まず差分を出力する"}
}

// interactiveFlag は --interactive/-i の宣言である。push/pull を専用モードで実行し、
// Target選択とdry-run有無を対話で選ぶ。他オプション併記時は無視して専用モードを優先する。
func interactiveFlag() cliv3.Flag {
	return &cliv3.BoolFlag{Name: "interactive", Aliases: []string{"i"}, Usage: "対話でTargetとdry-runを選ぶ"}
}

// newCommand はコマンド宣言ツリーを組み立てる。help/usage/unknown表示は
// 枠組み既定に任せ、--target の定義だけを宣言する。
// フラグ解釈失敗時（未知フラグ・値なし・Validator拒否）は OnUsageError 既定（nil）の
// Incorrect Usage＋help表示になる。
// 余分な位置引数の拒否は各サブコマンドの Before（rejectExtraArgs）に1箇所化する。
func (r *runner) newCommand() *cliv3.Command {
	return &cliv3.Command{
		Name:    "mdots",
		Usage:   "dotfilesをファイルコピー（非symlink）で管理するCLI。",
		Version: r.version,
		// 組み込みの help サブコマンドは表面にないため抑止する。
		// version 表示・help表示・unknown 時の扱いは枠組み標準に任せる。
		HideHelpCommand: true,
		Writer:          r.stdout,
		ErrWriter:       r.stderr,
		// ExitCoder（unknown command 時の exit 3等）で os.Exit しないよう
		// 無操作化し、Run 側で exit code を回収する（テスト同一プロセス性のため）。
		ExitErrHandler: func(context.Context, *cliv3.Command, error) {},
		Commands: []*cliv3.Command{
			{
				Name:      "add",
				Usage:     "未登録の既存ファイルを新規Entryとして登録する",
				ArgsUsage: "[--target <name>] <dest>...",
				Flags: []cliv3.Flag{
					targetFlag(),
				},
				// add は複数位置引数を受け付ける。0件の不足は Action 側の
				// 本質検査（missing argument）に残す。
				Action: r.addAction,
			},
			{
				Name:   "doctor",
				Usage:  "Store上で内容が一致するsrcを検出する",
				Before: r.rejectExtraArgs(0),
				Action: r.doctorAction,
			},
			{
				Name:   "init",
				Usage:  "Storeにmdots.toml雛形を作る",
				Before: r.rejectExtraArgs(0),
				Action: r.initAction,
			},
			{
				Name:  "pull",
				Usage: "destからStoreへファイルを回収する",
				Flags: []cliv3.Flag{
					targetFlag(),
					dryRunFlag(),
					colorFlag(),
					interactiveFlag(),
				},
				Before: r.rejectExtraArgs(0),
				Action: r.pullAction,
			},
			{
				Name:  "push",
				Usage: "Storeからdestへファイルをコピーする",
				Flags: []cliv3.Flag{
					targetFlag(),
					dryRunFlag(),
					colorFlag(),
					interactiveFlag(),
				},
				Before: r.rejectExtraArgs(0),
				Action: r.pushAction,
			},
			{
				Name:   "sort",
				Usage:  "mdots.tomlを正規形にソートする",
				Before: r.rejectExtraArgs(0),
				Action: r.sortAction,
			},
			{
				Name:   "targets",
				Usage:  "定義済みTarget名の一覧を表示する",
				Before: r.rejectExtraArgs(0),
				Action: r.targetsAction,
			},
		},
	}
}

// rejectExtraArgs は余分な位置引数を枠組み形式（Incorrect Usage＋help）で拒否する
// Before である。max 件まで許容し、max+1 件目を名指しする。
// 枠組みの Before は Incorrect Usage を自動表示しないため、ここで表示して
// exit 用エラーで中断する。--help は Before より前段で処理されるため優先される。
// なお bare `--` 以降は位置引数としてここで拒否される。いずれも exit 1 である。
func (r *runner) rejectExtraArgs(max int) cliv3.BeforeFunc {
	return func(_ context.Context, cmd *cliv3.Command) (context.Context, error) {
		args := cmd.Args()
		if args.Len() > max {
			extra := args.Get(max)
			fmt.Fprintf(r.stderr, "Incorrect Usage: unknown argument: %s\n\n", extra)
			_ = cliv3.ShowSubcommandHelp(cmd)
			return nil, &exitError{code: 1}
		}
		return nil, nil
	}
}

func (r *runner) pushAction(_ context.Context, cmd *cliv3.Command) error {
	if cmd.Bool("interactive") {
		return r.runInteractive("push")
	}
	target := cmd.String("target")
	if cmd.Bool("dry-run") {
		color := cmd.String("color")
		if code := app.PushDryRun(r.cwd, target, color, r.stdout, r.stderr); code != 0 {
			return &exitError{code: code}
		}
		return nil
	}
	if cmd.IsSet("color") {
		fmt.Fprintln(r.stderr, "--color requires --dry-run")
		return &exitError{code: 1}
	}
	if err := app.Push(r.cwd, target, r.stdout, r.stderr); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	return nil
}

func (r *runner) pullAction(_ context.Context, cmd *cliv3.Command) error {
	if cmd.Bool("interactive") {
		return r.runInteractive("pull")
	}
	target := cmd.String("target")
	if cmd.Bool("dry-run") {
		color := cmd.String("color")
		if code := app.PullDryRun(r.cwd, target, color, r.stdout, r.stderr); code != 0 {
			return &exitError{code: code}
		}
		return nil
	}
	if cmd.IsSet("color") {
		fmt.Fprintln(r.stderr, "--color requires --dry-run")
		return &exitError{code: 1}
	}
	if err := app.Pull(r.cwd, target, r.stdout, r.stderr); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	return nil
}

// runInteractive は push/pull の専用モードである。他オプション併記時は無視して
// こちらを優先する。targets と同一ソースの一覧から番号選択し、dry-run有無を選んで実行する。
func (r *runner) runInteractive(op string) error {
	if !r.isTTY() {
		fmt.Fprintln(r.stderr, "interactive requires a terminal")
		return &exitError{code: 1}
	}
	names, err := app.Targets(r.cwd)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	if len(names) == 0 {
		fmt.Fprintln(r.stderr, "no targets defined")
		return &exitError{code: 1}
	}
	for i, name := range names {
		fmt.Fprintf(r.stdout, "%d) %s\n", i+1, name)
	}
	target, dryRun, err := r.askInteractiveTarget(names)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	if dryRun {
		var code int
		if op == "push" {
			code = app.PushDryRun(r.cwd, target, "auto", r.stdout, r.stderr)
		} else {
			code = app.PullDryRun(r.cwd, target, "auto", r.stdout, r.stderr)
		}
		if code != 0 {
			return &exitError{code: code}
		}
		return nil
	}
	var runErr error
	if op == "push" {
		runErr = app.Push(r.cwd, target, r.stdout, r.stderr)
	} else {
		runErr = app.Pull(r.cwd, target, r.stdout, r.stderr)
	}
	if runErr != nil {
		fmt.Fprintln(r.stderr, runErr)
		return &exitError{code: 1}
	}
	return nil
}

// askInteractiveTarget は番号選択とdry-run確認を行う。空入力は既定印へ解決する。
// 戻り値のtargetは (default) 印を除いた素のTarget名である。
func (r *runner) askInteractiveTarget(names []string) (string, bool, error) {
	reader := bufio.NewReader(r.stdin)
	defaultIdx := 0
	for i, name := range names {
		if strings.HasSuffix(name, " (default)") {
			defaultIdx = i
			break
		}
	}
	fmt.Fprintf(r.stdout, "Select target [1-%d] (default %d): ", len(names), defaultIdx+1)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", false, err
	}
	line = strings.TrimSpace(line)
	idx := defaultIdx
	if line != "" {
		n, convErr := strconv.Atoi(line)
		if convErr != nil || n < 1 || n > len(names) {
			return "", false, fmt.Errorf("invalid selection %q: choose 1-%d", line, len(names))
		}
		idx = n - 1
	}
	target := strings.TrimSuffix(names[idx], " (default)")

	fmt.Fprint(r.stdout, "dry-run? [y/N]: ")
	dryLine, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", false, err
	}
	dryLine = strings.ToLower(strings.TrimSpace(dryLine))
	dryRun := dryLine == "y" || dryLine == "yes"
	return target, dryRun, nil
}

func (r *runner) initAction(_ context.Context, _ *cliv3.Command) error {
	if err := app.Init(r.cwd); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	fmt.Fprintln(r.stdout, "created mdots.toml")
	return nil
}

func (r *runner) targetsAction(_ context.Context, _ *cliv3.Command) error {
	names, err := app.Targets(r.cwd)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	for _, name := range names {
		fmt.Fprintln(r.stdout, name)
	}
	return nil
}

func (r *runner) sortAction(_ context.Context, _ *cliv3.Command) error {
	if err := app.Sort(r.cwd); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	return nil
}

func (r *runner) doctorAction(_ context.Context, _ *cliv3.Command) error {
	if code := app.Doctor(r.cwd, r.stdout, r.stderr); code != 0 {
		return &exitError{code: code}
	}
	return nil
}

func (r *runner) addAction(_ context.Context, cmd *cliv3.Command) error {
	args := cmd.Args()
	if !args.Present() {
		fmt.Fprintln(r.stderr, "missing argument")
		return &exitError{code: 1}
	}
	dests := args.Slice()
	target := cmd.String("target")
	results, err := app.AddMultiple(r.cwd, dests, target)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	for _, res := range results {
		fmt.Fprintf(r.stdout, "added %s (target: %s, src: %s)\n", res.Key, res.Resolved, res.Src)
	}
	return nil
}
