# targetsをTargetキー化し値は { src } とする

mdots.tomlの `{targets}` をTargetキー化したマップとし、値は `{ src }` 形式に決めた。destキー化（ADR-0010）と同様に重複を構造上不可能にし、値の短縮形と拡張余地を天秤にかけて対称性を優先したため。旧 `targets = [{ target, src }]` 配列は読めない破壊的変更とした。
