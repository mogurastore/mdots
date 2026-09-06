# diffコマンドを復活させpush/pull --dry-runを廃止する

0004でdiffを廃止しpush/pull --dry-runに一本化したが、0007でpullのみdest→Storeに反転したため方向で---/+++が入れ替わり混乱するため、差分表示をdest→Store固定の単一diffコマンドに戻すと決めた。destをold・Storeをnewとするためpushで追加される行が+になり、欠落は両側とも新規予定として報告する。--target/--colorとexit判定はdry-run資産を引き継ぎ、push/pullの--dry-run/--colorは互換なしで削除する。
