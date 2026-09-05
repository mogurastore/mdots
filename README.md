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
`src` は Store 相対のファイルパス、`dest` は `~` 展開される配置先パス、
`target` は省略時 common 扱いの自由文字列（例: `win`, `wsl`）。

```toml
# <Store>/mdots.toml
[[entries]]
src = "vimrc"
dest = "~/.vimrc"

[[entries]]
src = "wezterm.lua"
dest = "~/.config/wezterm/wezterm.lua"
target = "win"

[[entries]]
src = "shared.conf"
dest = "~/.config/shared.conf"
target = ["win", "wsl"]
```

```sh
mdots push              # common のみを Store から dest へコピー
mdots push --target win # common + win を対象にする
mdots pull --target win # dest から Store へ回収する
mdots push --dry-run    # 差分を diff -u 風に確認する（差分なし: exit 0、差分あり: exit 1）
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
| `push [--target <name>] [--dry-run]` | Store から dest へコピーする |
| `pull [--target <name>] [--dry-run]` | dest から Store へ回収する |
| `init` | カレント直下に `mdots.toml` 雛形を作る |

- `--target` 未指定時は common のみ、指定時は common + 指定 Target が対象になる。
- `--dry-run` は実際に書き込まず差分相当を出力する。差分ありは exit 1、差分なしは exit 0。
- `mdots.toml` が見つからないときは `mdots.toml not found in <cwd>` と表示し exit 1 になる。
- `init` は雛形を作る。既にあるときは `mdots.toml already exists in <cwd>` と表示し exit 1 になる。
- Store はカレント直下の `mdots.toml` のみ参照する。Store直下で実行し、サブディレクトリからは実行できない。
