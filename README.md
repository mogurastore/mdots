# mdots

dotfilesをファイルコピー（非symlink）で管理するCLI。
Store（`mdots.toml` を含む管理リポジトリのルート）とホーム側を `push` / `pull` で同期する。

## 導入

いずれかの方法で導入できる。

```sh
# go install
go install github.com/mogurastore/mdots@latest

# mise (github バックエンド)
mise use github:mogurastore/mdots@latest

# GitHub Releases からバイナリを取得
# https://github.com/mogurastore/mdots/releases から OS/Arch に合う
# mdots-<version>-<os>-<arch>.tar.gz (Windows は .zip) を展開する
```

## 最小サンプル

Store（管理リポジトリ）の直下に `mdots.toml` を置く。
`mdots init` でコメント付き雛形を作れる。
`[entries]` は配置先キー形式で書く。配置先（`~` / `~/...` はホームに展開）をキーにし、
値は `{ src = ... }` か `{ targets = [...] }` のどちらか一方だけを書く（併記不可）。
`src` は Store 相対のファイルパス、`target` は単数文字列（例: `target = "win"`）。
指定なしEntryはTargetの有無・値に関わらず常に適用され、
`targets` は一致したTargetの参照元だけが適用される（不一致・無指定時はスキップ）。
`"common"` は普通のTargetとして扱う。実行順は配置先ソートで固定される。

```toml
# <Store>/mdots.toml
[entries]
"~/.vimrc" = { src = "vimrc" }
"~/.config/wezterm/wezterm.lua" = { src = "wezterm.lua" }
"~/.gitconfig" = { targets = [
  { target = "win", src = "win/.gitconfig" },
  { target = "wsl", src = "wsl/.gitconfig" },
] }
```

```sh
mdots push              # 指定なしEntryのみを Store から dest へコピー
mdots push --target win # 指定なしEntry＋winに一致したEntryを対象にする
mdots pull --target win # dest から Store へ回収する
mdots push --dry-run    # 差分を dest→Store方向に出力する（差分なし: exit 0、差分あり: exit 1。+はpushで追加される行）
mdots pull --dry-run    # 差分を Store→dest方向に出力する（+はpullで取り込まれる行）
```

## 使い方

```sh
mdots --help            # 使い方を表示する
mdots push --help       # コマンド別の使い方を表示する
mdots --version         # バージョンを表示する（ldflags -X main.version で埋め込み）
```

> 補足: `go install @latest` で導入した場合はバージョンが `mdots dev` と表示される。
> タグ付きバージョンは GitHub Releases のバイナリでのみ埋め込まれる。

| コマンド | 意味 |
| --- | --- |
| `push [--target <name>] [--dry-run] [--color auto\|always\|never]` | Store から dest へコピーする |
| `pull [--target <name>] [--dry-run] [--color auto\|always\|never]` | dest から Store へ回収する |
| `init` | カレント直下に `mdots.toml` 雛形を作る |

- `--target` 未指定時は指定なしEntryのみ、指定時は指定なしEntry＋一致したTargetのEntryが対象になる。一致なしはスキップされる。
- `--dry-run` は実際に書き込まず差分を出力する。差分ありは exit 1、差分なしは exit 0。方向は操作のコピー方向に合わせる。`--color` 単独はエラーになる。
- `mdots.toml` が見つからないときは `mdots.toml not found in <cwd>` と表示し exit 1 になる。
- `init` は雛形を作る。既にあるときは `mdots.toml already exists in <cwd>` と表示し exit 1 になる。
- Store はカレント直下の `mdots.toml` のみ参照する。Store直下で実行し、サブディレクトリからは実行できない。
