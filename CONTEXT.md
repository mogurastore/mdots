# mdots

dotfilesをファイルコピー（非symlink）で管理するCLI。Store（管理リポジトリ）とホーム側を push/pull で同期する。

## Language

**Store**:
mdots.toml を含む管理リポジトリのルート。カレント直下のmdots.tomlのみ参照する。
_Avoid_: repo, dotfiles repo

**Entry**:
1つの管理対象を表す src/dest ペア。srcはStore相対のファイルパス、destは~展開される配置先パス。
_Avoid_: mapping, rule

**Target**:
Entryの適用先を識別する単数文字列（例: win, wsl, linux）。指定なしEntryはTargetに関わらず適用される。
_Avoid_: os, platform, env, common

**push**:
Storeからdestへファイルをコピーする操作。
_Avoid_: deploy, apply

**pull**:
destからStoreへファイルを回収する操作。
_Avoid_: capture, import

**init**:
Storeにmdots.toml雛形を作る操作。
_Avoid_: create, new, setup

**dry-run**:
push/pullで書き込まず差分を出力する操作。方向は操作のコピー方向に合わせる。
_Avoid_: diff, preview

**override**:
Entryのコピー先に既存ファイルがある場合の上書き可否。falseは既存を保護し新規作成のみ行う。
_Avoid_: overwrite, force
