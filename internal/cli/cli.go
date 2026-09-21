// Package cli は mdots のコマンド骨格を定義する。
//
// CLIフレームワークの最終採用は urfave/cli v3（宣言ツリーで push/pull と
// --target/--dry-run/--color を定義する）。
//
// CLI表面（help/version/unknown/usage時の文面・exit）は枠組み既定に寄せる。
// 内部実行は app 直接呼出とし、エントリは薄く保つ.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/mogurastore/mdots/internal/app"
	cliv3 "github.com/urfave/cli/v3"
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
}

// Run は CLI表面の入口である。args は os.Args[1:] 相当、cwd は実行ディレクトリ。
// 戻り値はプロセスの exit code。
// フラグ解釈・help/version/unknown の表面は枠組み既定に任せ、
// 空値拒否は Flag.Validator、余剰引数拒否は Before に寄せる。
// 位置引数の不足・--color依存など本質検査だけを Action 側に残す。
func Run(args []string, cwd, version string, stdout, stderr io.Writer) int {
	r := &runner{version: version, cwd: cwd, stdout: stdout, stderr: stderr}

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
			fmt.Fprintln(stderr, ec.Error())
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
				Name:  "push",
				Usage: "Storeからdestへファイルをコピーする",
				Flags: []cliv3.Flag{
					targetFlag(),
					dryRunFlag(),
					colorFlag(),
				},
				Before: r.rejectExtraArgs(0),
				Action: r.pushAction,
			},
			{
				Name:  "pull",
				Usage: "destからStoreへファイルを回収する",
				Flags: []cliv3.Flag{
					targetFlag(),
					dryRunFlag(),
					colorFlag(),
				},
				Before: r.rejectExtraArgs(0),
				Action: r.pullAction,
			},
			{
				Name:   "init",
				Usage:  "Storeにmdots.toml雛形を作る",
				Before: r.rejectExtraArgs(0),
				Action: r.initAction,
			},
			{
				Name:   "targets",
				Usage:  "定義済みTarget名の一覧を表示する",
				Before: r.rejectExtraArgs(0),
				Action: r.targetsAction,
			},
			{
				Name:   "doctor",
				Usage:  "共有化可能なEntryを検出する",
				Before: r.rejectExtraArgs(0),
				Action: r.doctorAction,
			},
			{
				Name:  "add",
				Usage: "未登録の既存ファイルを新規Entryとして登録する",
				Flags: []cliv3.Flag{
					targetFlag(),
				},
				// add は位置引数1件まで許容する。0件の不足は Action 側の
				// 本質検査（missing argument）に残し、2件目以降だけ拒否する。
				Before: r.rejectExtraArgs(1),
				Action: r.addAction,
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
	if err := app.Push(r.cwd, target, r.stderr); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	return nil
}

func (r *runner) pullAction(_ context.Context, cmd *cliv3.Command) error {
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
	if err := app.Pull(r.cwd, target, r.stderr); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	return nil
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
	dest := args.First()
	target := cmd.String("target")
	key, src, resolved, err := app.Add(r.cwd, dest, target)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	fmt.Fprintf(r.stdout, "added %s (target: %s, src: %s)\n", key, resolved, src)
	return nil
}
