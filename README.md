# mdots

dotfilesを管理するCLI。

## 特徴

- 双方向にコピー（push/pull）
- target指定で複数環境に対応
- symlinkではなくファイルをコピーする方式
- override=falseで既存を保護し新規作成のみ行う

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

# ファイルをコピー
mdots push
mdots pull

# 未登録の既存ファイルを新規Entryとして登録する（取り込みはpullが行う）
mdots add ~/.vimrc

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

配置先をテーブルヘッダにする（add の保存と同じテーブル形式）。

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
