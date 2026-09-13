# push/pull --dry-runを復活させdiffコマンドを廃止する

0008で差分表示をdest→Store固定の単一diffに戻したが、操作ごとのプレビュー利点を優先しpush/pull --dry-runに一本化すると決めた。方向は操作のコピー方向に合わせ（pushはdest→Store、pullはStore→dest）、--target/--colorとexit判定・欠落報告はdiff資産を引き継ぎ、diffは互換なしで削除する。
