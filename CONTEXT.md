# mdots

dotfilesをファイルコピー（非symlink）で管理するCLI。Store（管理リポジトリ）とホーム側を push/pull/diff で同期する。

## Language

**Store**:
mdots.yaml を含む管理リポジトリのルート。カレント直下のmdots.yamlのみ参照する。
_Avoid_: repo, dotfiles repo

**Entry**:
1つの管理対象を表す src/dest ペア。srcはStore相対のファイルパス、destは~展開される配置先パス。
_Avoid_: mapping, rule

**Target**:
Entryの適用先を識別する自由文字列（例: win, wsl, linux）。省略時はcommon。
_Avoid_: os, platform, env

**common**:
Target未指定のEntry。--target 未指定時にのみ適用され、指定時は common + 指定Target が適用される。
_Avoid_: base, default

**push**:
Storeからdestへファイルをコピーする操作。
_Avoid_: deploy, apply

**pull**:
destからStoreへファイルを回収する操作。
_Avoid_: capture, import

**diff**:
Storeとdestの差分を表示する操作。
_Avoid_: status, check
