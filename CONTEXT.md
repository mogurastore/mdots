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
Entryの適用先を識別する単数文字列（例: win, wsl, linux）。解決は単一 Target のみで、重ねは二度押しで表現する。
_Avoid_: os, platform, env, common

**default_target**:
省略時に解決する共有固定の既定 Target 名。
_Avoid_: fallback

**push**:
Storeからdestへファイルをコピーする操作。
_Avoid_: deploy, apply

**pull**:
destからStoreへファイルを回収する操作。
_Avoid_: capture, import

**init**:
Storeにmdots.toml雛形を作る操作。
_Avoid_: create, new, setup

**add**:
未登録の既存ファイルを新規Entryとして登録する操作。取り込みはpullが行う。
_Avoid_: import, capture

**targets**:
定義済みTarget名の一覧を表示する操作。
_Avoid_: list

**dry-run**:
push/pullで書き込まず差分を出力する操作。方向は操作のコピー方向に合わせる。
_Avoid_: diff, preview

**override**:
Entryのコピー先に既存ファイルがある場合の上書き可否。falseは既存を保護し新規作成のみ行う。
_Avoid_: overwrite, force

**dotfiles**:
Store直下でaddが生成する既定のsrc配置 (例: dotfiles/.vimrc)。必須ではなく推奨で、他のsrcも読み書きできる。

**sharable**:
別src間でStore上の内容が一致し共有化候補となる状態。destの異同は問わない。
_Avoid_: duplicate
