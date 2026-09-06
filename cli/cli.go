// Package cli は mdots のコマンド骨格を定義する。
//
// CLIフレームワークの最終採用は urfave/cli v3（宣言ツリーで push/pull/diff と
// --target を定義する）。
//
// CLI表面（help/version/unknown/usage時の文面・exit）は枠組み既定に寄せる。
// 内部実行は Executor 委譲とし、エントリは薄く保つ。
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	cliv3 "github.com/urfave/cli/v3"
)

// Executor は push/pull/diff/init の内部実行系への委譲口である。
// 表面（文面・exit）は本パッケージが保ち、副作用のある処理だけを委譲する。
type Executor interface {
	Push(cwd, target string) error
	Pull(cwd, target string) error
	Diff(cwd, target, color string, stdout, stderr io.Writer) int
	Init(cwd string) error
}

// exitError は Action が呼び出し元 Run へ exit code を伝えるための内用エラーで、
// 出力は Action 側で済ませているため文面を持たない。
type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

// runner は1回の Run 呼び出しに対応する宣言ツリーと入出力を束ねる。
type runner struct {
	version string
	exec    Executor
	cwd     string
	stdout  io.Writer
	stderr  io.Writer
}

// Run は CLI表面の入口である。args は os.Args[1:] 相当、cwd は実行ディレクトリ。
// 戻り値はプロセスの exit code。
// フラグ解釈・help/version/unknown の表面は枠組み既定に任せ、
// 空値・余剰引数の拒否だけを Action 側で残す。
func Run(args []string, cwd, version string, ex Executor, stdout, stderr io.Writer) int {
	r := &runner{version: version, exec: ex, cwd: cwd, stdout: stdout, stderr: stderr}

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

// targetFlag は --target の宣言である。--target <name> と --target=<name> の
// 両形式はフレームワークが解釈する。
func targetFlag() cliv3.Flag {
	return &cliv3.StringFlag{Name: "target", Usage: "対象Target (例: win, wsl)。--target=<name> 形式も可"}
}

// colorFlag は --color の宣言である。diff の着色制御で auto|always|never を取る。
func colorFlag() cliv3.Flag {
	return &cliv3.StringFlag{Name: "color", Value: "auto", Usage: "差分の着色 (auto|always|never)"}
}

// parseColor は --color 値を検証する。不正時は文面を出して exit 用エラーを返す。
func (r *runner) parseColor(cmd *cliv3.Command) (string, error) {
	color := cmd.String("color")
	switch color {
	case "auto", "always", "never":
		return color, nil
	default:
		fmt.Fprintf(r.stderr, "invalid value for --color: %s (want auto|always|never)\n", color)
		return "", &exitError{code: 1}
	}
}

// newCommand はコマンド宣言ツリーを組み立てる。help/usage/unknown表示は
// 枠組み既定に任せ、--target の定義だけを宣言する。
// フラグ解釈失敗時（未知フラグ・値なし）は OnUsageError 既定（nil）の
// Incorrect Usage＋help表示になる。
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
				},
				Action: r.pushAction,
			},
			{
				Name:  "pull",
				Usage: "destからStoreへファイルを回収する",
				Flags: []cliv3.Flag{
					targetFlag(),
				},
				Action: r.pullAction,
			},
			{
				Name:  "diff",
				Usage: "Storeとdestの差分をdest→Store方向に出力する",
				Flags: []cliv3.Flag{
					targetFlag(),
					colorFlag(),
				},
				Action: r.diffAction,
			},
			{
				Name:   "init",
				Usage:  "Storeにmdots.toml雛形を作る",
				Action: r.initAction,
			},
		},
	}
}

// argError は余分な位置引数を拒否する。
// なお --help と不正トークンの併用時は help が不正より前にある場合に限り
// help 優先となり（逆順は利用エラー）、bare `--` 以降は位置引数として
// ここで拒否される。いずれも exit 1 である。
func (r *runner) argError(arg string) error {
	fmt.Fprintf(r.stderr, "unknown argument: %s\n", arg)
	return &exitError{code: 1}
}

func (r *runner) targetOf(cmd *cliv3.Command) (string, error) {
	target := cmd.String("target")
	if cmd.IsSet("target") && target == "" {
		// --target= のように空値で指定された場合。
		fmt.Fprintln(r.stderr, "missing value for --target")
		return "", &exitError{code: 1}
	}
	return target, nil
}

// targetArgs は余分な位置引数の検査と --target の解釈をまとめて行う。
func (r *runner) targetArgs(cmd *cliv3.Command) (string, error) {
	if cmd.Args().Present() {
		return "", r.argError(cmd.Args().First())
	}
	return r.targetOf(cmd)
}

func (r *runner) pushAction(_ context.Context, cmd *cliv3.Command) error {
	target, err := r.targetArgs(cmd)
	if err != nil {
		return err
	}
	if err := r.exec.Push(r.cwd, target); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	return nil
}

func (r *runner) pullAction(_ context.Context, cmd *cliv3.Command) error {
	target, err := r.targetArgs(cmd)
	if err != nil {
		return err
	}
	if err := r.exec.Pull(r.cwd, target); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	return nil
}

func (r *runner) diffAction(_ context.Context, cmd *cliv3.Command) error {
	target, err := r.targetArgs(cmd)
	if err != nil {
		return err
	}
	color, err := r.parseColor(cmd)
	if err != nil {
		return err
	}
	if code := r.exec.Diff(r.cwd, target, color, r.stdout, r.stderr); code != 0 {
		return &exitError{code: code}
	}
	return nil
}

func (r *runner) initAction(_ context.Context, cmd *cliv3.Command) error {
	if cmd.Args().Present() {
		return r.argError(cmd.Args().First())
	}
	if err := r.exec.Init(r.cwd); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	fmt.Fprintln(r.stdout, "created mdots.toml")
	return nil
}
