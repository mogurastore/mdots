# dry-runに--statのファイル別集計を追加する

push/pull --dry-run の差分が大きいと全体像を掴みにくいため、--stat を追加する。--dry-run と併用したときだけ有効で、patch の代わりにファイル別の変更行数バーと集計行（files changed / insertions / deletions）を出す。方向は dry-run の差分方向に合わせ、ラベルは new 側（push は Store の src、pull は dest）とする。--stat 単独は --color と同様に --dry-run 必須エラーとする。
