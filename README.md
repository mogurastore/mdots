# mdots

dotfilesを管理するCLI。

## 特徴

- 双方向にコピー（push/pull）
- target指定で複数環境に対応
- symlinkではなくファイルをコピーする方式

## インストール

```sh
go install github.com/mogurastore/mdots@latest

mise use github:mogurastore/mdots@latest

# または GitHub Releases から OS/Arch に合うバイナリを取得
```

## 使い方

```sh
# 設定ファイルを作る
mdots init

# 既存ファイルを新規Entryとして設定ファイルに登録する
mdots add ~/.vimrc

# ファイルをコピー
mdots push
mdots pull

# Target付きで切り替え
mdots push --target win
mdots pull --target win

# 定義済みTarget名の一覧を表示
mdots targets

# 差分を確認
mdots push --dry-run
mdots pull --dry-run
```

## 設定例（mdots.toml）

```toml
[entries."~/.vimrc"]
src = "dotfiles/.vimrc"

[entries."~/.bashrc"]
src = "dotfiles/.bashrc"
override = false

[entries."~/.gitconfig".targets.win]
src = "dotfiles/win/.gitconfig"

[entries."~/.gitconfig".targets.wsl]
src = "dotfiles/wsl/.gitconfig"
override = false
```
