# add単発回帰とinteractive廃止で仕様を簡素化する

ADR-0017の複数add（all-or-nothing）とpush/pullの--interactive専用モードを廃止し、`mdots add [--target <name>] <dest>`単発と非対話のpush/pullに戻す。2件目以降はunknown argumentで拒否し、-iは空ける。仕様とコードの見通しを優先し、半端な登録残り防止は単発の不変性（失敗時不変）で満たすためで、ADR-0017をsupersedeする。
