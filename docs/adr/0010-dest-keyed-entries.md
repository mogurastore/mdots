# Entryをdestキー化しcommonの特別扱いを廃止する

mdots.tomlの `[entries]` をdestキー化し、値は `{src}` と `{targets=[{target, src}]}` の排他、targetは単数string、指定なしEntryは常に適用、`"common"` は普通のTargetとして扱うことにした。[[entries]]反復の冗長さを消し、同dest内の競合を構造で不可能にし、二重表現を許さないため。旧形式は読めなくなる破壊的変更とした。
