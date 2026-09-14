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

# mdots.toml を編集する（配置先をキーにする）
# "~/.vimrc" = { src = "vimrc" }
# "~/.gitconfig" = {
#   targets = [
#     { target = "win", src = "win/.gitconfig" },
#     { target = "wsl", src = "wsl/.gitconfig" },
#   ],
# }

# ファイルをコピー
mdots push
mdots pull

# Target付きで切り替え
mdots push --target win
mdots pull --target win

# 差分を確認
mdots push --dry-run
mdots pull --dry-run
```
