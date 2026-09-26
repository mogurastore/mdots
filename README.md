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

## 始め方

現在の環境からStoreを作成する。

```sh
# 1. mdots.tomlを作る
mdots init

# 2. 管理したいファイルを登録する（登録だけでコピーはしない）
mdots add ~/.vimrc

# 3. 差分を確認してからStoreに回収する（dest -> Store）
mdots pull --dry-run
mdots pull
```

別マシンではStoreをcloneして展開します。

```sh
# 差分を確認してから展開する（Store -> dest）
mdots push --dry-run
mdots push
```

日常の運用はこの繰り返しです。

```sh
# 編集したらStoreに取り込む
mdots pull

# Storeを更新したら各マシンに配る
mdots push
```

## 設定例（mdots.toml）

```toml
default_target = "base"

[targets.base."~/.vimrc"]
src = "dotfiles/base/.vimrc"

[targets.base."~/.bashrc"]
src = "dotfiles/base/.bashrc"
override = false

[targets.wsl."~/.gitconfig"]
src = "dotfiles/wsl/.gitconfig"
```
