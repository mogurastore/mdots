# urfave/cli v3.11.0 デフォルト挙動の一次情報調査

- 調査日: 2026-09-06
- 対象バージョン: `github.com/urfave/cli/v3 v3.11.0`（`go.mod:7` で確認。以下「v3.11.0」）
- 一次情報の範囲: 公式ドキュメント（https://cli.urfave.org/）、v3.11.0 のソースコード
  （`https://github.com/urfave/cli/blob/v3.11.0/...`。手元のモジュールキャッシュ
  `/home/lisp719/go/pkg/mod/github.com/urfave/cli/v3@v3.11.0/` の実物と行番号で対照）、
  公式サンプル（上記ドキュメント内の Examples およびリポジトリ内 `examples_test.go`）。
  二次記事は不使用。
- 検証方法: ソース読解に加え、デフォルト構成の `cli.Command`（compat コードなし）を
  v3.11.0 の実ソースに対して実行するプローブで挙動を実測した。プローブはプロセス死を
  避けるため `cli.OsExiter` のみ記録式に差し替え、`ExitErrHandler` は既定（nil）のまま
  であるため、既定のエラーハンドリング経路はそのまま通っている。

## 0. 前提確認

- `go.mod:7` は `github.com/urfave/cli/v3 v3.11.0`。当該バージョンのソースに当たった。
- `cli/cli.go` の互換維持コードとは、`Run` 101–138 行の先頭トークン事前振り分け、
  `CustomRootCommandHelpTemplate`（165 行）・`CustomHelpTemplate`（173, 183, 195, 204 行）、
  `usageError`（231–246 行）・`rawFlagToken`（250–261 行）・`targetOf`（270–279 行）・
  `targetArgs`（282–287 行）・`argError`（264–268 行）、`rootAction` ガード（217–225 行）、
  `version` サブコマンド宣言（199–210 行）を指す。

## 1. v3.11.0 のデフォルト挙動

### 1.1 help テンプレートと help 手段

- 既定の help フラグは `cli.HelpFlag`（`-h`/`--help` の `BoolFlag`）であり、内部で検査
  されて生成済み help を表示して実行を打ち切る。
  出典: 公式ドキュメント「Generated Help Text」章（https://cli.urfave.org/v3/examples/help/generated-help-text/）冒頭、
  ソース `flag.go:39-45`（https://github.com/urfave/cli/blob/v3.11.0/flag.go）。
- help テンプレート変数は `RootCommandHelpTemplate`・`CommandHelpTemplate`・
  `SubcommandHelpTemplate` の3つで、再代入・拡張・`cli.HelpPrinter` 差し替えで
  カスタマイズする設計。
  出典: 同ドキュメント「Customization」章、ソース `template.go:44`（Root）、
  `template.go` 内 `CommandHelpTemplate`・`template.go:94`（Subcommand）
  （https://github.com/urfave/cli/blob/v3.11.0/template.go）。
- 既定の root help の見出しは `NAME:`/`USAGE:`/`COMMANDS:`/`GLOBAL OPTIONS:` の
  大文字形式（例: `USAGE:\n   mdots [global options] [command [command options]]`）。
  現行凍結文面（`usage:` 小文字始まりの `GlobalHelp`）とは形式が異なる。
  出典: ソース `template.go:44-`、およびプローブ実測（`no-args` ケースで上記形式を stdout に出力）。
- `HideHelp` 未設定の既定では、help サブコマンド（`help`/`h`）と `HelpFlag` が自動付与
  される。すなわち既定で `mdots help` および `mdots help push` が使える。
  出典: ソース `command_setup.go:222-270`（`ensureHelp`。help コマンド追加は 244–252 行、
  HelpFlag 追加は 254–268 行）（https://github.com/urfave/cli/blob/v3.11.0/command_setup.go）。
  プローブ実測でも `help` / `help push` が help を表示し err は nil。
- 引数なし実行の既定は「root の既定 Action（help 表示）」であり、help を stdout に
  出して err は nil（呼び出し元が `os.Exit` しない限りプロセス exit 0 相当）。
  出典: ソース `command_setup.go:44-47`（`Action == nil` なら `helpCommandAction` を設定）、
  プローブ実測（`no-args` ケース: stdout に root help、err nil、exit 系未発火）。

### 1.2 HideVersion / HideHelpCommand / Suggest の既定値

- 3つとも構造体フィールドのゼロ値は `false` であり、`setupDefaults` で無条件に
  `true` へ変える処理はない。
  出典: ソース `command.go:53`（HideHelpCommand）、`command.go:55`（HideVersion）、
  `command.go:114`（Suggest）（https://github.com/urfave/cli/blob/v3.11.0/command.go）。
- 唯一の例外は `Version == ""` の場合に `HideVersion = true` が自動設定されること
  （バージョン未定義なら version 表示手段を隠す）。
  出典: ソース `command_setup.go:39-42`
  （https://github.com/urfave/cli/blob/v3.11.0/command_setup.go）。
- `Suggest` はオプトイン機能であり、ドキュメントも「`Command.Suggest = true` に
  設定して有効化する」と説明する。既定 `false` では不明フラグ・不明コマンド時の
  サジェスト追記は行われない（`command_run.go:194-198` の `if cmd.Suggest` が偽に
  なる。`help.go:318-327` の `if cmd.Suggest` も同様）。
  出典: 公式ドキュメント「Suggestions」章（https://cli.urfave.org/v3/examples/help/suggestions/）、
  ソース `command.go:114`、公式サンプル `examples_test.go:547`（`Suggest: true` を
  明示する例）（https://github.com/urfave/cli/blob/v3.11.0/examples_test.go）。

### 1.3 ExitErrHandler の既定

- フィールドのドキュメントは「提供されなければ `HandleExitCoder` を既定動作として
  使用する」と明記している。
  出典: ソース `command.go:91-94`
  （https://github.com/urfave/cli/blob/v3.11.0/command.go）。
- 呼び出し側は root まで再帰して `ExitErrHandler` を探し、nil なら `HandleExitCoder(err)`
  を呼ぶ。
  出典: ソース `command.go:324-336`（`handleExitCoder`）
  （https://github.com/urfave/cli/blob/v3.11.0/command.go）。
- `HandleExitCoder` は `ExitCoder` 実装エラー時にメッセージを出力して
  `OsExiter(exitCode)` を呼び、`MultiError` 時は内包エラーを同様に処理する。
  **通常のエラー（`ExitCoder` でも `MultiError` でもない、例: フラグ解釈失敗の生エラー）
  に対しては何もしない**（`os.Exit` しない。`Command.Run` はそのエラーを呼び出し元に
  返すだけ）。
  出典: ソース `errors.go`（`HandleExitCoder`、`Exit`、`handleMultiError`）
  （https://github.com/urfave/cli/blob/v3.11.0/errors.go）。
  公式ドキュメント「Exit Codes」章も「`Command.Run` の呼び出しは自動的に `os.Exit`
  を呼ばない。exit code を付けたい場合は `cli.ExitCoder` を満たすエラーを返せ」と
  説明する（https://cli.urfave.org/v3/examples/exit-codes/）。
- 注意点（実測で判明）: `HandleExitCoder` の出力先はパッケージグローバルの
  `ErrWriter`（既定 `os.Stderr`）であり、**`Command.ErrWriter` に注入した writer には
  出ない**。プローブでは `No help topic for 'version'` が注入バッファではなく
  プロセス標準エラーに出た。
  出典: ソース `errors.go`（`var ErrWriter io.Writer = os.Stderr` と `HandleExitCoder`
  内の `fmt.Fprintln(ErrWriter, ...)`）、プローブ実測。

### 1.4 OnUsageError の既定

- 既定は nil（ゼロ値）。フラグ解釈失敗・必須フラグ不足・引数解釈失敗の各経路は
  `if cmd.OnUsageError != nil` で分岐し、nil の場合は既定文面を出す。
  出典: ソース `command_run.go:189-213`（フラグ解釈失敗経路）、`command_run.go:261-272`
  および `command_run.go:348-358`（その他の usage error 経路）
  （https://github.com/urfave/cli/blob/v3.11.0/command_run.go）。
  型定義は `funcs.go:30`
  （https://github.com/urfave/cli/blob/v3.11.0/funcs.go）。
- 既定文面は `Incorrect Usage: <生エラー>\n\n` を `cmd.Root().ErrWriter`（注入先が
  生きる）へ出し、続けて root なら root help、サブコマンドなら subcommand help を
  `Writer`（stdout 側）へ出す。戻り値は生の解釈エラーそのもの（exit code は付与
  されない。前節のとおり `HandleExitCoder` も通常エラーには無反応）。
  出典: ソース `command_run.go:194-213`、プローブ実測（`push-unknown-flag` ケース:
  stderr=`Incorrect Usage: flag provided but not defined: -unknown\n\n`、
  stdout=subcommand help、err=同文面の通常エラー、exit 系未発火）。
- なお解釈失敗時に `checkHelp()` が真（help フラグが既に解釈済み）の場合は、
  `OnUsageError` より先に help 表示して err nil で復帰する（§3 の優先順位を参照）。
  出典: ソース `command_run.go:178-193`。

### 1.5 SkipFlagParsing の既定

- 既定は `false`（ゼロ値）。
  出典: ソース `command.go:119`
  （https://github.com/urfave/cli/blob/v3.11.0/command.go）。
- `true` の場合、そのコマンドのフラグ解釈を丸ごと飛ばし、引数をそのまま位置引数
  として `Action` に渡す。
  出典: ソース `command_run.go:157-163`
  （https://github.com/urfave/cli/blob/v3.11.0/command_run.go）。

### 1.6 version 表示の既定（--version / -v / version サブコマンド）

- 既定の version フラグは `cli.VersionFlag`（`--version`、`-v`（小文字）の `BoolFlag`）。
  `-V`（大文字）は含まれない。
  出典: 公式ドキュメント「Basics » Version Flags」章の「A default version flag
  (`-v/--version`) is provided as `cli.VersionFlag`」
  （https://cli.urfave.org/v3/examples/flags/basics/）、
  ソース `flag.go:28-34`
  （https://github.com/urfave/cli/blob/v3.11.0/flag.go）。
- version フラグは root（`isRoot`）にのみ自動付与される。サブコマンド側
  （例: `mdots push --version`）では未知フラグ扱いになる。
  出典: ソース `command_setup.go:96-118`（`!cmd.HideVersion && isRoot` 条件）、
  ソース `flag.go:33`（`Local: true`）、プローブ実測（`push-version-flag` ケース:
  `flag provided but not defined: -version`）。
- 表示形式の既定は `VersionPrinter`（=`DefaultPrintVersion`）による
  `"<Name> version <Version>\n"`（例: `mdots version v0.0.0-test`）。現行凍結文面
  `mdots <version>` とは異なる。
  出典: ソース `help.go:350-358`（`ShowVersion`、`DefaultPrintVersion`）
  （https://github.com/urfave/cli/blob/v3.11.0/help.go）、プローブ実測。
- version 判定は root のみ（`cmd.parent == nil && !cmd.HideVersion && checkVersion(cmd)`）
  で、後続引数を無視して version を表示して err nil で復帰する。
  出典: ソース `command_run.go:222-224`、ソース `help.go:470-472`（`checkVersion`）。
- **`version` サブコマンドは既定では存在しない。** `mdots version` は
  unknown command 扱い（§1.7）になる。
  出典: ソース `command_setup.go:222-252`（自動付与されるのは help コマンドのみ）、
  プローブ実測（`version-subcmd` ケース: `No help topic for 'version'`、exit 3）。

### 1.7 unknown command 時の文面と exit コード

- 未知サブコマンドは最終的に root の Action（既定は `helpCommandAction`）に渡り、
  第一位置引数を help トピック名として解決しようとして失敗する。
  `CommandNotFound` 未設定（既定 nil）の場合は文面 `No help topic for '<name>'` を
  作り `Exit(errMsg, 3)` を返す。
  出典: ソース `help.go:318-328`
  （https://github.com/urfave/cli/blob/v3.11.0/help.go）。
- `Exit(_, 3)` は `ExitCoder` なので、`ExitErrHandler` 既定（`HandleExitCoder`）に
  よりメッセージがパッケージグローバル `ErrWriter`（= プロセスの stderr）に出て
  `OsExiter(3)` が呼ばれる。すなわち**既定の unknown command は exit 3** であり、
  現行凍結（`unknown command: <name>` + GlobalHelp を注入 stderr へ、exit 1）とは
  文面・出力先・exit コードのすべてが異なる。
  出典: ソース `errors.go`（`Exit`、`HandleExitCoder`）、プローブ実測
  （`unknown-cmd` ケース: err=`No help topic for 'frobnicate'`、OsExiter(3) 発火、
  注入バッファには何も出ない）。
- `CommandNotFound` を設定すればこの経路は自前化できる（設定時は nil 早期復帰せず
  自作関数が呼ばれる）。
  出典: ソース `help.go:318-334`、型定義 `funcs.go:21`。

### 1.8 unknown flag 時の文面と exit コード

- 生エラー文面は `flag provided but not defined: -<name>`（単ダッシュ＋正規化名）。
  `-V` と `--V` の区別はこの時点では失われる。
  出典: ソース `command_parse.go:10`（`providedButNotDefinedErrMsg` 定数）
  （https://github.com/urfave/cli/blob/v3.11.0/command_parse.go）。
- 既定の利用者向け出力は `Incorrect Usage: flag provided but not defined: -<name>`
  （stderr 注入先）＋ subcommand help（stdout 注入先）。`Command.Run` の戻り値は
  通常エラーであり exit code は付与されない（`os.Exit` もしない）。
  出典: ソース `command_run.go:194-213`、プローブ実測（`push-unknown-flag`、
  `version-V-upper` の各ケース）。
- 現行凍結（`unknown argument: <元綴り>`＋コマンド別 help を stderr へ集約、exit 1）
  とは文面・出力先・exit 付与のいずれも異なる。

## 2. 現行 cli/cli.go（99–335 行）との差分と削減見込み

### 2.1 先頭トークン事前振り分け（Run 101–138 行）

既定に従うと各入力の扱いは次のように変わる（いずれもプローブ実測済み）：

| 入力 | 現行凍結 | 既定 | 差 |
|---|---|---|---|
| 引数なし | GlobalHelp を stderr へ、exit 1 | root help を stdout へ、err nil（exit 0 相当） | 出力先・文面・exit が変わる |
| `--help`/`-h` | GlobalHelp を stdout へ、exit 0 | root help テンプレートを stdout へ、err nil | exit は同じ、文面が変わる |
| `--version` | `mdots <v>` を stdout へ、exit 0 | `mdots version <v>` を stdout へ、err nil | 文面が変わる |
| `-V` | version 表示、exit 0 | `flag provided but not defined: -V` の usage error（help 付き） | 壊れる（後述の凍結テストが落ちる） |
| `version` | version 表示、exit 0（事前振り分けが先取り） | `No help topic for 'version'`、exit 3 | 壊れる |
| 未知コマンド | `unknown command: <name>`＋GlobalHelp を stderr へ、exit 1 | `No help topic for '<name>'` をグローバル stderr へ、exit 3 | 文面・出力先・exit が変わる |

削減量の目安（概算）: 事前振り分け本体は 104–124 行の約 21 行。ただし削除すると
上表のとおり凍結テスト（`TestCliNoArgsPrintsGlobalHelpToStderr`、
`TestCliVersion`、`TestCliUnknownCommand`、`TestCliDiffIsRemoved`）がすべて落ちる
ため、テスト側の凍結を変えない限り削除できない。分類は「要判断」（§5 参照）。

### 2.2 CustomRootCommandHelpTemplate / CustomHelpTemplate（153–213 行）

- 既定テンプレート（§1.1）に寄せると `GlobalHelp`・`PushHelp`・`PullHelp`・
  `InitHelp` の各定数（22–69 行、約 48 行）と、設定行（165、173、183、195 行の
  計 4 行）が不要になる。
- ただし既定 help は `USAGE:` 大文字・`COMMANDS:`/`GLOBAL OPTIONS:` 構成・
  `VERSION:` 節付きであり、凍結文面と完全一致しない。`TestCliGlobalHelp` は
  `usage:`/`push`/`pull`/`init` の部分一致だが `USAGE:` は小文字 `usage:` を含まない
  ため、そのままでは落ちる。`TestCliCommandHelp`（`--target`/`--dry-run` の含有
  検査）および `TestCliInitHelp`（`init`/`mdots.toml` の含有検査）は既定テンプレート
  でも含有条件を満たす可能性がある（いずれも Usage・フラグ一覧に出る文字列の
  ため）が、日本語 Usage での実測は未実施（§5 参照）。
  よって分類は「要判断」（テストの凍結緩和が前提。ただし `TestCliCommandHelp`・
  `TestCliInitHelp` は既定テンプレートでも含有条件を満たす可能性があるため、
  移行時に再実測のこと。§4.1 参照）。

### 2.3 usageError / rawFlagToken / targetOf / targetArgs（231–287 行）

- 既定（`OnUsageError` nil）では §1.4 のとおり `Incorrect Usage:`＋help 表示＋
  生エラー返却になる。凍結文面（`missing value for --target` /
  `unknown argument: <元綴り>` を stderr へ集約、exit 1）を保つ限り、
  `usageError` 本体（231–246 行、約 16 行）と元綴り復元の `rawFlagToken`
  （250–261 行、約 12 行）は必要。分類は「要判断」（文面凍結を緩めれば削除可）。
- 技術的根拠: 生エラー `flag needs an argument: --target` への contains 判定
  （239 行 `strings.Contains(msg, "needs an argument")`）は v3.11.0 の定数
  `argumentNotProvidedErrMsg = "flag needs an argument: "`（`command_parse.go:11`）
  と整合している。`flag provided but not defined: -` 前方一致（241 行）も同ファイル
  10 行の定数と整合している。すなわち現行コードは v3.11.0 の文面に依存しており、
  将来の文面変更には弱い。
- `targetOf` の空値検査（272–277 行）・`targetArgs`/`argError` の余剰位置引数検査
  （264–268、282–287 行）は、既定が「余剰位置引数を黙殺」「`--target=` 空値を許容」
  であることへの上乗せである（§3 で実測）。削除すると `TestCliTargetFlagErrors`
  の `extra-positional`・`--target=` ケースが exit 0 になって落ちる。
  分類は「凍結維持」。

### 2.4 rootAction ガード（217–225 行、約 9 行）

- 事前振り分けを残す構成では到達不能に近いが、削除すると未知コマンド到達時の
  挙動が既定（`No help topic`、exit 3）に変わる。事前振り分けごと削除する計画と
  一体で判断すべきであり、単独分類は「要判断」。

### 2.5 version サブコマンド宣言（199–210 行、約 12 行）

- 既定に `version` サブコマンドは存在しない（§1.6）。宣言を消すと `mdots version`
  は exit 3 の `No help topic` になり、`TestCliVersion` の `{"version"}` ケースが
  落ちる。事前振り分け（115 行の `"version"` 分岐）が残る限り宣言側は実質
  デッドコードだが、ツリー完全性のための残置とコメントにあるとおり、削ると
  凍結テストが壊れる。分類は「凍結維持」。
- なお `SkipFlagParsing: true`（205 行）はこの宣言専用の設定であり、宣言ごと残す
  以上は維持。

### 2.6 HideVersion / HideHelpCommand / Suggest（160–162 行）

- `HideVersion: true`・`HideHelpCommand: true` は load-bearing である。
  `Version` 非空のため、既定では `--version`/`-v` フラグと `help` サブコマンドが
  自動付与される（§1.1、§1.6）。外すと version 文面が `mdots version <v>` に
  変わり、`help` サブコマンドが出現する。分類は「凍結維持」。
- `Suggest: false`（162 行）のみ冗長である。既定が `false`（§1.2）のため削除して
  も挙動は同一。分類は「削減可」（1 行）。

### 2.7 ExitErrHandler（167 行）

- no-op ハンドラはテスト同一プロセス性を守る load-bearing コードである。
  既定（`HandleExitCoder`）に戻すと、`ExitCoder`（例: unknown command 時の
  `Exit(_, 3)`）で `OsExiter`（既定 `os.Exit`）が呼ばれ、テストプロセスが死ぬ。
  分類は「凍結維持」。

## 3. フラグ詳細

### 3.1 --target の space 形式 / equals 形式

- 両形式とも対応する。space 形式は次トークンを値として消費し、equals 形式は
  `=` 右辺を値とする。
  出典: 公式ドキュメント「Basics」章の「Flag values can be provided with a space
  after the flag name or using the `=` sign」
  （https://cli.urfave.org/v3/examples/flags/basics/）、
  ソース `command_parse.go`（`=` 分割処理と次トークン消費処理）
  （https://github.com/urfave/cli/blob/v3.11.0/command_parse.go）。
  プローブ実測: `--target win`・`--target=win` ともに `target="win"` で Action 実行。
- 現行 `targetFlag`（142–144 行）のコメント記述と一致する。削減余地なし。

### 3.2 --target の値なし（末尾）

- 生エラーは `flag needs an argument: --target`（注: ダブルダッシュ付きの
  トークン全体が付く）。
  出典: ソース `command_parse.go:11`（定数）、`command_run.go:194` 経路、
  プローブ実測（`push-target-missing` ケース）。
- 既定の利用者向け出力は `Incorrect Usage: flag needs an argument: --target`＋
  help。現行は `missing value for --target` に正規化して stderr 集約・exit 1。
  出典: `cli/cli.go:234-237`、プローブ実測。

### 3.3 --target=（空値 equals）

- **エラーにならない。** `=` ありのため値なし判定を素通りし、空文字で `Set` され、
  `IsSet("target") == true`・`String("target") == ""` の状態で Action が実行される。
  出典: ソース `command_parse.go`（`valFromEqual` フラグ。
  `flagVal == "" && !valFromEqual` のときのみ次トークン要求／不足エラーのため、
  `=` 付き空値はそのまま `cmd.set` される）、ソース `flag_string.go`
  （`stringValue.Set` は空値を拒否しない）、プローブ実測
  （`push-target-empty-eq` ケース: err nil で `ACTION push target=""` 実行）。
- 現行 `targetOf`（270–279 行）がこの穴を塞いで `missing value for --target`・
  exit 1 にしている。既定移行で当該検査を外すと、空 target が黙って実行系に
  流れる。分類は「凍結維持」。

### 3.4 --dry-run（bool）

- 値なし指定で `true` になる。`--dry-run=false` のように `=` 形式で明示値も取れる。
  値の解釈は `strconv.ParseBool`。
  出典: 公式ドキュメント「Basics」章の「Boolean flags are the exception: passing
  a boolean flag without a value sets it to `true`. Use the `=` form, such as
  `--flag=false`, to provide an explicit boolean value.」
  （https://cli.urfave.org/v3/examples/flags/basics/）、
  ソース `command_parse.go`（`boolFlag` 分岐で `flagVal` 空時に `"true"` を補う）、
  ソース `flag_bool.go:60-76`（`boolValue.Set` の `ParseBool` 処理）
  （https://github.com/urfave/cli/blob/v3.11.0/flag_bool.go）。
  プローブ実測: `--dry-run` → `dry=true`、`--dry-run=false` → `dry=false`。
- 現行 `dryRunFlag`（147–149 行）はこの既定のままで凍結テストと整合する。
  削減余地なし。

### 3.5 --help/-h 併用時と不正トークン併用時の優先順位

- **順序依存であり、無条件の help 優先ではない。** help フラグが不正トークンより
  先に解釈されれば help 表示（err nil）、不正トークンが先なら usage error。
  - `push --help --unknown` → help 表示、err nil（help 勝ち）。
  - `push --unknown --help` → `flag provided but not defined: -unknown` の
    usage error（error 勝ち。後続の `--help` は解釈されない）。
  出典: ソース `command_run.go:178-193`（解釈失敗時に `checkHelp()` を先に見る。
  `checkHelp` は `command.go:185-189` のとおり解釈済み help フラグの有無のみで
  決まるため、help がエラー箇所より前で解釈済みかどうかが勝敗を決める）、
  プローブ実測（`help-then-bad` / `bad-then-help` ケース）。
- 現行 `cli/cli.go:229-230` のコメント「`--help` と不正トークンの併用時は
  フレームワークの help 優先」は imprecise であり、「help トークンが不正トークン
  より前にある場合に限り help 優先」が正確。移行時のコメント修正が必要。
- 凍結テストに help＋不正トークンの併用ケースは含まれていない（`TestCliCommandHelp`
  は単独 help のみ）。よってこの点はテスト境界外。分類は「未確認」ではなく実測済み
  だが、テストで凍結されていないため移行時の振る舞い選択は自由。

### 3.6 bare `--` の扱い

- `--` 以降はフラグ解釈を打ち切り、残りすべてを位置引数として `Action` に渡す。
  出典: ソース `command_parse.go:87-95`（`// stop parsing once we see a "--"`）、
  公式ドキュメントのシェル補完章における「`--` の後は位置引数のみ受け付ける」
  言及（`help.go:490-491` コメント、https://github.com/urfave/cli/blob/v3.11.0/help.go）、
  プローブ実測（`bare-dd` ケース: `push -- --target win` で
  `args=["--target" "win"]` として Action 実行、target は空）。
- 現行は `targetArgs`（282–287 行）が余剰位置引数を `unknown argument`・exit 1 に
  するため、`push -- --target win` は exit 1 になる（コメント 230 行の
  「bare `--` はフラグ区切りとして扱う。いずれも exit 1」の記述と一致）。
  既定移行で `targetArgs` を外すと exit 0 通過になる。分類は「凍結維持」
  （`TestCliTargetFlagErrors` の精神を守るため）。

## 4. テストで凍結されている外部挙動とデフォルト移行の境界

凡例: ○=既定移行でも壊れない、×=壊れる、△=条件付き。

### 4.1 cli/cli_test.go

| テスト | 凍結内容 | 既定移行時の判定 | 根拠 |
|---|---|---|---|
| TestCliGlobalHelp（73–86 行） | `--help`/`-h` で exit 0 かつ stdout に `usage:`/`push`/`pull`/`init` を含む | ×（`usage:` で落ちる） | 既定は `USAGE:` 大文字（§1.1）。小文字 `usage:` を含まない |
| TestCliNoArgsPrintsGlobalHelpToStderr（88–97 行） | 引数なしで exit 1 かつ stderr に `usage:` | ×（3 点とも変わる） | 既定は stdout＋err nil（§1.1、§2.1） |
| TestCliVersion（99–110 行） | `--version`/`-V`/`version` で exit 0 かつ stdout に version 含有 | △（`--version` のみ文面違いで落ち、`-V`/`version` は経路ごと消滅） | 既定は `mdots version <v>` 形式、`-V` 未定義、`version` サブコマンドなし（§1.6） |
| TestCliUnknownCommand（112–123 行） | 未知コマンドで exit 1 かつ stderr に `unknown command: frobnicate`＋`usage:` | × | 既定は `No help topic`＋exit 3＋グローバル stderr（§1.7） |
| TestCliDiffIsRemoved（125–134 行） | `diff` で非 zero かつ `unknown command: diff` | ×（exit は非 zero だが文面が変わる） | §1.7 と同経路 |
| TestCliCommandHelp（136–157 行） | `push --help` 等で exit 0 かつ `push`/`--target`/`--dry-run` 含有 | ○（文面は変わるが含有物は残る可能性が高い） | 既定 subcommand help に `mdots push`・`--target`・`--dry-run`・`--help, -h` が出ることをプローブで確認（`push-help` ケース）。ただし厳密には stringify 形式依存のため移行時に再実測が必要 |
| TestCliPushDispatchesTarget（159–184 行） | 無指定/space/equals で target 伝達・exit 0 | ○ | いずれも既定の素直な解釈（§3.1）。プローブ実測済み |
| TestCliTargetFlagErrors（186–201 行） | 7 ケース全て非 zero | △（`--target` 末尾・`--unknown` は既定でも非 zero だが、`--target=` と `extra-positional` は exit 0 通過になる） | §3.3、§3.6 の実測。`pull` 系も同様 |
| TestCliPushDryRunDelegatesExitCode（203–220 行） | dry-run 委譲と exit 伝播 | ○ | フラグ解釈自体は既定通り（§3.4）。Action 本体は維持前提 |
| TestCliPullDryRunDelegatesExitCode（222–231 行） | 同上 pull 版 | ○ | 同上 |
| TestCliExecutorErrorExitsNonZero（233–242 行） | 実行エラーの stderr・非 zero | ○ | Action 本体のエラーハンドリングは移行対象外。ただし既定 `ExitErrHandler` に戻すと通常エラーは `os.Exit` しないため、呼び出し側の int 化は別途必要（§1.3） |
| TestCliInitDispatches（246–260 行） | `init` で exit 0、`created`/`mdots.toml` 含有 | ○ | Action 本体維持前提 |
| TestCliInitHelp（262–278 行） | `init --help`/`-h` で exit 0、`init`/`mdots.toml` 含有、Init 未実行 | △（要実測。含有物は残る可能性があるが未検証） | 既定 subcommand help は Usage（`Storeにmdots.toml雛形を作る`）を出すため `init`・`mdots.toml` が含有される可能性がある。ただし日本語 Usage での実測はしていないため、移行時に再実測が必要 |
| TestCliInitRejectsExtraArgs（280–288 行） | `init extra` で非 zero、Init 未実行 | ×（exit 0 通過になる） | 既定は余剰位置引数を黙殺（プローブ `extra-positional` ケース） |
| TestCliInitExecutorError（290–299 行） | 実行エラーの stderr・非 zero | ○ | 同 ExecutorError |

### 4.2 cli_test.go（root）

| テスト | 凍結内容 | 判定 | 根拠 |
|---|---|---|---|
| TestDryRunIntegration（16–67 行） | dry-run の差分出力・非書き込み・exit | ○ | フラグ解釈は既定通り。実行系は移行対象外 |
| TestDiffIsRemoved（69–78 行） | `diff` で非 zero＋`unknown command: diff` | ×（文面） | §1.7 |
| TestStoreNotFoundFriendlyError（80–89 行） | Store なし push の friendly 文言 | ○ | 実行系の文言であり CLI 層に依存しない |

### 4.3 main_test.go

| テスト | 凍結内容 | 判定 | 根拠 |
|---|---|---|---|
| TestPushEndToEnd / TestPullEndToEnd / TestPushWithTargetRepresentative / TestPullMissingDestIsError / TestCommandsWithoutStoreFail / TestPushFromSubdirFails | いずれも正常系・異常系の exit と副作用 | ○ | すべて素直なフラグ解釈（無指定・space 形式）のみを使用し、help・unknown・余剰引数に触れないため |

### 4.4 境界のまとめ

- 壊れる境界は「CLI 表面の文面・exit・出力先」に集中する（help、version、`-V`、
  unknown command/flag、`--target=`、`extra-positional`、`--`）。
- 壊れない境界は「素直なフラグ解釈＋実行委譲」（space/equals の target、dry-run
  の委譲・exit 伝播、push/pull/init の正常系、実行エラーの非 zero 伝播）である。
- すなわち、デフォルト移行の可否はテスト凍結の緩和可否と同値であり、コード削減
  だけを先行させると §4.1 の×項目が落ちる。

## 5. 未確認事項

- `UseShortOptionHandling` 有効時の挙動は未確認（現行コードは未使用のため調査対象外）。
  既定は無効であることのみ確認（`command.go` のゼロ値 `false`。ドキュメント
  「Short Options」章 https://cli.urfave.org/v3/examples/flags/short-options/）。
- `mutuallyExclusiveFlags`・`PrefixMatchCommands`・環境変数由来フラグ等の高度機能は
  未確認（現行コードは未使用）。
- v3.11.0 より新しいバージョンでの文面変更の有無は未確認（本調査は v3.11.0 に固定）。
- `TestCliCommandHelp` が既定テンプレートで通るかの厳密な確認は未確認
  （プローブは英語 Usage のため。日本語 Usage での再実測が必要）。

## 6. 削減候補の一覧

分類定義: 削減可=既定に戻しても凍結テストが壊れない / 要判断=テスト凍結の緩和と
セットで削減可 / 凍結維持=削ると凍結テストが壊れる。

### 削減可

- `cli/cli.go:162`（`Suggest: false`）: 既定が `false` のため冗長。1 行削減。
  出典: ソース `command.go:114`、ドキュメント「Suggestions」章
  （https://cli.urfave.org/v3/examples/help/suggestions/）。

### 要判断

- `cli/cli.go:107-110`（引数なし分岐）: 既定は stdout＋exit 0。`TestCliNoArgsPrintsGlobalHelpToStderr`
 （`cli/cli_test.go:88-97`）の緩和が前提。約 4 行。
- `cli/cli.go:111-114`（`--help`/`-h` 分岐）: 既定でも exit 0 だが文面が変わる。
  `TestCliGlobalHelp`（`cli/cli_test.go:73-86`）の `usage:` 含有条件の緩和が前提。約 4 行。
- `cli/cli.go:115-117`（`--version`/`-V`/`version` 分岐）: 既定は `mdots version <v>`・
  `-V` 未定義・`version` サブコマンドなし。`TestCliVersion`
  （`cli/cli_test.go:99-110`）の緩和が前提。約 3 行。
- `cli/cli.go:120-124`（unknown 分岐）＋`cli/cli.go:217-225`（`rootAction` ガード）:
  既定は `No help topic`・exit 3。`TestCliUnknownCommand`・`TestCliDiffIsRemoved`
  （`cli/cli_test.go:112-134`、`cli_test.go:69-78`）の緩和が前提。一体で約 14 行。
- `cli/cli.go:22-69`（Help 定数群）＋`cli/cli.go:165,173,183,195`（テンプレート指定）:
  既定テンプレートへの寄せ。`TestCliGlobalHelp`・`TestCliInitHelp` 等の文面条件の
  緩和が前提。約 52 行。
- `cli/cli.go:231-246`（`usageError`）＋`cli/cli.go:250-261`（`rawFlagToken`）:
  既定の `Incorrect Usage:`＋help 表示への寄せ。`TestCliTargetFlagErrors`
  （`cli/cli_test.go:186-201`）の文面検査はない（exit のみ）ため exit 面では維持
  可能だが、文面凍結（`cli_test.go` の `unknown command` 系や将来の文面テスト）を
  捨てる判断が前提。約 28 行。
- `cli/cli.go:229-230`（`usageError` の優先順位コメント）: 「help 優先」は
  順序条件付きが正確（§3.5）。削減ではなく訂正。要 1–2 行修正。
  出典: ソース `command_run.go:178-193`。

### 凍結維持

- `cli/cli.go:160`（`HideVersion: true`）: 外すと version フラグ自動付与で文面変化。
  `TestCliVersion` が壊れる。出典: ソース `command_setup.go:96-118`。
- `cli/cli.go:161`（`HideHelpCommand: true`）: 外すと `help` サブコマンドが出現。
  出典: ソース `command_setup.go:244-252`。
- `cli/cli.go:167`（`ExitErrHandler` no-op）: 外すと `ExitCoder` 時に `os.Exit` が
  呼ばれテストプロセスが死ぬ。出典: ソース `command.go:324-336`、`errors.go`。
- `cli/cli.go:199-210`（`version` サブコマンド宣言）: 消すと `mdots version` が
  exit 3 になる。`TestCliVersion` が壊れる。出典: §1.6、プローブ実測。
- `cli/cli.go:264-287`（`argError`・`targetOf`・`targetArgs`）: 外すと
  `extra-positional`・`--target=` が exit 0 通過になる。
  `TestCliTargetFlagErrors`・`TestCliInitRejectsExtraArgs` が壊れる。
  出典: §3.3、§3.6、プローブ実測。
