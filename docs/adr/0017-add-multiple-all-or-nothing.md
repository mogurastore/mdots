# add複数件はall-or-nothingで登録する

`mdots add [--target <name>] <dest>...` は複数destを一括登録し、事前に全件検証して1件でもNGならmdots.tomlを変更せずNG全件をstderrに報告する。単一targetを全件に適用し、成功時は入力順に `added ...` を出す。半端な登録残りを避けADR-0013の「検証失敗時は変更しない」を複数件に拡張するためで、best-effortに対する安全側の選択である。
