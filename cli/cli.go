// Package cli は mdots のコマンド骨格を定義する。
//
// CLIフレームワークの最終採用は urfave/cli v3（宣言ツリーで push/pull と
// --target/--dry-run を定義する）。pflagのみによる自前解析は段階移行の予備案
// として残すが、既定は v3 とする（spec #24 の Implementation Decisions による）。
//
// CLI表面（help/version時の文面・exit）は枠組み既定に寄せる。
// unknown時の文面・exitは凍結値のまま残し（#42で既定化）、
// 内部実行は Executor 委譲とし、エントリは薄く保つ。
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	cliv3 "github.com/urfave/cli/v3"
)

const GlobalHelp = `usage: mdots <push|pull|init> [options]

dotfilesをファイルコピー（非symlink）で管理するCLI。
Store（mdots.toml を含む管理リポジトリのルート）の直下で実行する。mdots.toml はカレント直下のみ参照する。

commands:
  push  Storeからdestへファイルをコピーする
  pull  destからStoreへファイルを回収する
  init  Storeにmdots.toml雛形を作る

global options:
  -h, --help     使い方を表示する
  --version      バージョンを表示する

run 'mdots <command> --help' for command-specific help.
`

const PushHelp = `usage: mdots push [--target <name>] [--dry-run]

Storeからdestへファイルをコピーする。
--target 未指定時はcommonのみ、指定時は common + 指定Target が対象。

options:
  --target <name>  対象Target (例: win, wsl)。--target=<name> 形式も可
  --dry-run        実際に書き込まず差分相当を出力する。差分ありは exit 1
  -h, --help       使い方を表示する
`

const PullHelp = `usage: mdots pull [--target <name>] [--dry-run]

destからStoreへファイルを回収する。
--target 未指定時はcommonのみ、指定時は common + 指定Target が対象。

options:
  --target <name>  対象Target (例: win, wsl)。--target=<name> 形式も可
  --dry-run        実際に書き込まず差分相当を出力する。差分ありは exit 1
  -h, --help       使い方を表示する
`

const InitHelp = `usage: mdots init

Storeにmdots.toml雛形を作る。
カレント直下に雛形を作り、見て使い方が分かるコメントとサンプルを含む。
既にあるときは mdots.toml already exists in <cwd> と表示し exit 1 になる。

options:
  -h, --help       使い方を表示する
`

// Executor は push/pull/init の内部実行系への委譲口である。
// 表面（文面・exit）は本パッケージが保ち、副作用のある処理だけを委譲する。
type Executor interface {
	Push(cwd, target string) error
	Pull(cwd, target string) error
	PushDryRun(cwd, target string, stdout, stderr io.Writer) int
	PullDryRun(cwd, target string, stdout, stderr io.Writer) int
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
	// cmdArgs はサブコマンド名より後ろの生トークンである。未知フラグの報告時に
	// フレームワークが正規化した名前から元の綴り（-V と --V の区別など）を復元する。
	cmdArgs []string
}

// Run は CLI表面の入口である。args は os.Args[1:] 相当、cwd は実行ディレクトリ。
// 戻り値はプロセスの exit code。
func Run(args []string, cwd, version string, ex Executor, stdout, stderr io.Writer) int {
	r := &runner{version: version, exec: ex, cwd: cwd, stdout: stdout, stderr: stderr}

	// 先頭トークンの事前振り分け。help（--help/-h・引数なし）は枠組み既定に任せるため
	// 事前振り分けしない。version は枠組み標準に任せるため事前振り分けしない。
	// push/pull の詳細なフラグ解釈は宣言ツリーに任せる。
	// unknown は凍結値のまま保つ（#42で既定化する）。
	if len(args) > 0 {
		switch args[0] {
		case "--help", "-h", "--version", "-v", "-V", "push", "pull", "init":
			// 宣言ツリーへ進む。--help/-h・引数なしは既定のhelp表示、
			// --version/-v は標準の version 表示、
			// -V は標準の未知フラグ扱いになる。
			// "version" は独自サブコマンドを廃止したため default の未知扱いに落とす。
		default:
			fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
			fmt.Fprint(stderr, GlobalHelp)
			return 1
		}
	}

	cmd := r.newCommand()
	if len(args) > 0 {
		r.cmdArgs = args[1:]
	}
	if err := cmd.Run(context.Background(), append([]string{"mdots"}, args...)); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			return ee.code
		}
		// 想定外のエラーは文面を出して exit 1（表面の想定経路はすべて exitError）。
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// targetFlag は --target の宣言である。--target <name> と --target=<name> の
// 両形式はフレームワークが解釈する。
func targetFlag() cliv3.Flag {
	return &cliv3.StringFlag{Name: "target", Usage: "対象Target (例: win, wsl)。--target=<name> 形式も可"}
}

// dryRunFlag は --dry-run の宣言である。
func dryRunFlag() cliv3.Flag {
	return &cliv3.BoolFlag{Name: "dry-run", Usage: "実際に書き込まず差分相当を出力する。差分ありは exit 1"}
}

// newCommand はコマンド宣言ツリーを組み立てる。help表示は枠組み既定の
// テンプレートに任せ、--target/--dry-run の定義だけを宣言する。
func (r *runner) newCommand() *cliv3.Command {
	return &cliv3.Command{
		Name:    "mdots",
		Usage:   "dotfilesをファイルコピー（非symlink）で管理するCLI。",
		Version: r.version,
		// 組み込みの help サブコマンドは表面にないため抑止する。
		// version 表示・help表示は枠組み標準に任せる。
		// unknown 時の扱いは Run の事前振り分けが凍結値で行う（#42で既定化する）。
		HideHelpCommand: true,
		Writer:          r.stdout,
		ErrWriter:       r.stderr,
		// 想定外の出力（Incorrect Usage 等）を抑え、文面は自前の凍結値のみにする。
		ExitErrHandler: func(context.Context, *cliv3.Command, error) {},
		Commands: []*cliv3.Command{
			{
				Name:  "push",
				Usage: "Storeからdestへファイルをコピーする",
				Flags: []cliv3.Flag{
					targetFlag(),
					dryRunFlag(),
				},
				OnUsageError: r.usageError(PushHelp),
				Action:       r.pushAction,
			},
			{
				Name:  "pull",
				Usage: "destからStoreへファイルを回収する",
				Flags: []cliv3.Flag{
					targetFlag(),
					dryRunFlag(),
				},
				OnUsageError: r.usageError(PullHelp),
				Action:       r.pullAction,
			},
			{
				Name:         "init",
				Usage:        "Storeにmdots.toml雛形を作る",
				OnUsageError: r.usageError(InitHelp),
				Action:       r.initAction,
			},
		},
	}
}

// usageError はフラグ解釈失敗時（未知フラグ・--target の値不足）の表面を凍結値で出す。
//
// なお従来の自前解析との優先順位の違いは残る。--help と不正トークンの併用時は help が不正より前にある場合に限り help 優先となり（逆順は usageError）、
// bare `--` はフラグ区切りとして扱う。いずれも exit 1 である点は従来通り。
func (r *runner) usageError(commandHelp string) cliv3.OnUsageErrorFunc {
	return func(_ context.Context, _ *cliv3.Command, err error, _ bool) error {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "needs an argument"):
			// --target の値がない場合。対象フラグは target のみ。
			msg = "missing value for --target"
		case strings.HasPrefix(msg, "flag provided but not defined: -"):
			name := strings.TrimLeft(strings.TrimPrefix(msg, "flag provided but not defined: -"), "-")
			msg = "unknown argument: " + r.rawFlagToken(name)
		}
		fmt.Fprintln(r.stderr, msg)
		fmt.Fprint(r.stderr, commandHelp)
		return &exitError{code: 1}
	}
}

// rawFlagToken は報告されたフラグ名に対応する元の綴りを生トークンから探す。
// 見つからなければ --<name> 形式にフォールバックする。
func (r *runner) rawFlagToken(name string) string {
	for _, tok := range r.cmdArgs {
		base := strings.TrimLeft(tok, "-")
		if i := strings.Index(base, "="); i >= 0 {
			base = base[:i]
		}
		if base == name {
			return tok
		}
	}
	return "--" + name
}

// argError は余分な位置引数の表面を凍結値で出す。
func (r *runner) argError(commandHelp, arg string) error {
	fmt.Fprintf(r.stderr, "unknown argument: %s\n", arg)
	fmt.Fprint(r.stderr, commandHelp)
	return &exitError{code: 1}
}

func (r *runner) targetOf(cmd *cliv3.Command, commandHelp string) (string, error) {
	target := cmd.String("target")
	if cmd.IsSet("target") && target == "" {
		// --target= のように空値で指定された場合。
		fmt.Fprintln(r.stderr, "missing value for --target")
		fmt.Fprint(r.stderr, commandHelp)
		return "", &exitError{code: 1}
	}
	return target, nil
}

// targetArgs は余分な位置引数の検査と --target の解釈をまとめて行う。
func (r *runner) targetArgs(cmd *cliv3.Command, commandHelp string) (string, error) {
	if cmd.Args().Present() {
		return "", r.argError(commandHelp, cmd.Args().First())
	}
	return r.targetOf(cmd, commandHelp)
}

func (r *runner) pushAction(_ context.Context, cmd *cliv3.Command) error {
	target, err := r.targetArgs(cmd, PushHelp)
	if err != nil {
		return err
	}
	if cmd.Bool("dry-run") {
		if code := r.exec.PushDryRun(r.cwd, target, r.stdout, r.stderr); code != 0 {
			return &exitError{code: code}
		}
		return nil
	}
	if err := r.exec.Push(r.cwd, target); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	return nil
}

func (r *runner) pullAction(_ context.Context, cmd *cliv3.Command) error {
	target, err := r.targetArgs(cmd, PullHelp)
	if err != nil {
		return err
	}
	if cmd.Bool("dry-run") {
		if code := r.exec.PullDryRun(r.cwd, target, r.stdout, r.stderr); code != 0 {
			return &exitError{code: code}
		}
		return nil
	}
	if err := r.exec.Pull(r.cwd, target); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	return nil
}

func (r *runner) initAction(_ context.Context, cmd *cliv3.Command) error {
	if cmd.Args().Present() {
		return r.argError(InitHelp, cmd.Args().First())
	}
	if err := r.exec.Init(r.cwd); err != nil {
		fmt.Fprintln(r.stderr, err)
		return &exitError{code: 1}
	}
	fmt.Fprintln(r.stdout, "created mdots.toml")
	return nil
}
