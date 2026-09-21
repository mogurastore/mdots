# targets.<名>.<dest>統一＋default_target単一解決にする

mdots.tomlの Entry 記法を `targets.<名>.<dest>` のみに統一し、無指定Entryと `[common]` を持たないことにした。共有固定の `default_target` を持ち、省略時は既定へ解決し、明示・既定とも未定義名はエラーにする。解決は単一 Target のみとし、重ねは `push`＋`push -t wsl` の二度押しで表現する。手動 Target 選択（ADR-0002）の方針と整合させ、ゼロベース・リリース前のため旧 `entries` 形式からの移行は提供しない破壊的変更とした。
