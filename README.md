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

Storeから環境へ展開します。

```sh
# 差分を確認してから展開する（Store -> dest）
mdots push --dry-run
mdots push
```

### 複数環境で使う場合

同じ配置先を Target 別に分けて管理できます。

```sh
# Targetごとに登録・回収する
mdots add --target win ~/.gitconfig
mdots pull --target win

mdots add --target wsl ~/.gitconfig
mdots pull --target wsl

# 定義済みTargetの一覧
mdots targets

# 環境に合わせて展開する
mdots push --target wsl
```

## その他の機能

- 共有候補の検出: `doctor` で Store 上で内容が一致する src を検出
- 設定の整理: `sort` で mdots.toml を正規形にソート

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
