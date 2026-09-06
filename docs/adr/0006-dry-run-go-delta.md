# dry-run差分表示にgo-deltaを採用する

自前LCSはO(n*m)で巨大ファイルに弱く単一ハンクしか出せないため、差分生成と着色をgo-delta一本に寄せると決めた。inline統一・前後3行・word強調・auto既定（`--color=auto|always|never`で上書き）でpush/pullのdry-runに使う。
