# urfave/cli v3の既定に従いCLI表面の互換維持コードを削減する

自前解析から移行した際に凍結したhelp/version/unknown文面・exit維持コードが肥大化したため、空値・余剰引数拒否（targetOf/targetArgs）のみ残し、helpテンプレ・事前振り分け・usageError・version宣言を既定に戻して削減すると決めた。調査は docs/research/urfave-cli-v3-defaults.md による。

## Considered Options

- 凍結維持: 既存文面・exit 1を保つが、約100行の上乗せが残るため却下。
- 既定に全寄せ: help（USAGE:形式）、version（mdots version <v>、-vのみ）、unknown（No help topic、exit 3）、引数なし（stdout、exit 0）、Incorrect Usage:受容を採用。

## Consequences

- TestCliGlobalHelp/NoArgs/Version/UnknownCommand/DiffIsRemoved等の文面・exit期待値を更新する必要がある。
- HideVersion/HideHelpCommand/ExitErrHandler no-opはテスト死防止のため残す。
