package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config は Store 直下の mdots.toml 全体を表す。
// [entries] は配置先キー化され、値は {src} か {targets} の排他いずれかである。
type Config struct {
	Entries map[string]EntryValue `toml:"entries"`
}

// EntryValue は1つの配置先に対する設定値である。
// Src か Targets のいずれか一方のみを持つ（排他）。
// Override は任意指定で、省略時は true として解決される。
// Targets 形式では Target 値の override が優先され、なければ配置先直下が使われる。
type EntryValue struct {
	Src      *string                 `toml:"src"`
	Override *bool                   `toml:"override"`
	Targets  *map[string]TargetValue `toml:"targets"`
}

// TargetValue は Target キーごとの参照元である。Target はマップキー（単数文字列）。
// Override は任意指定で、省略時は配置先直下・既定値 true に従う。
type TargetValue struct {
	Src      *string `toml:"src"`
	Override *bool   `toml:"override"`
}

// Entry は解決済みの1つの管理対象を表す src/dest ペア。
// src は Store 相対のファイルパス、dest は ~ 展開される配置先パス。
// Override は既存保護の解決結果で、false は既存を保護し新規作成のみ行う。
type Entry struct {
	Src      string `toml:"src"`
	Dest     string `toml:"dest"`
	Override bool   `toml:"override"`
}

// sortedDests は配置先キーをソート順で返す。解決・検証の順序を固定する。
func (c Config) sortedDests() []string {
	dests := make([]string, 0, len(c.Entries))
	for dest := range c.Entries {
		dests = append(dests, dest)
	}
	sort.Strings(dests)
	return dests
}

// Resolve は指定 Target に対する解決済み Entry 群を配置先ソート順で返す.
// 指定なし Entry は Target の有無・値に関わらず常に適用される.
// {targets} は完全一致のみ適用され、不一致・無指定時はスキップされる.
// "common" は普通のTargetとしてのみ一致する.
// override は解決結果に引き継ぐのみで、選択自体には影響しない。省略時は true。
func (c Config) Resolve(target string) []Entry {
	dests := c.sortedDests()
	var out []Entry
	for _, dest := range dests {
		v := c.Entries[dest]
		if v.Src != nil {
			out = append(out, Entry{Src: *v.Src, Dest: dest, Override: resolveOverride(v.Override, nil)})
			continue
		}
		if v.Targets == nil {
			continue
		}
		if tv, ok := (*v.Targets)[target]; ok && tv.Src != nil {
			out = append(out, Entry{Src: *tv.Src, Dest: dest, Override: resolveOverride(v.Override, tv.Override)})
		}
	}
	return out
}

// Targets は全Entryのtargetsキーを集約し重複排除・ソートして返す。
// 指定なしEntryは無視し、targetsマップのキーのみを対象とする。
// 未定義時は空スライスを返す。
func (c Config) Targets() []string {
	seen := map[string]struct{}{}
	for _, v := range c.Entries {
		if v.Targets == nil {
			continue
		}
		for tname := range *v.Targets {
			seen[tname] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for tname := range seen {
		out = append(out, tname)
	}
	sort.Strings(out)
	return out
}

// resolveOverride は override の解決規則である。Target 値が優先され、
// なければ配置先直下、どちらも省略時は true（上書きする＝現状維持）。
func resolveOverride(top, inner *bool) bool {
	if inner != nil {
		return *inner
	}
	if top != nil {
		return *top
	}
	return true
}

// Load は Store の mdots.toml を読み込み、validation して返す。
// 旧 [[entries]] 配列形式・旧 target 記法は明確に失敗させる。
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := rejectOldFormat(data); err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Entries == nil {
		cfg.Entries = map[string]EntryValue{}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// rejectOldFormat は旧形式を汎用マップで先読みし、明確な失敗文言で拒否する。
// 旧 [[entries]] 配列・旧 target 記法・旧 targets 配列を拒否し、
// 配置先直下と targets マップ値の未知フィールドもここで厳密に拒否する。
// override は bool のみ受け付け、不正値は明確に失敗させる。
func rejectOldFormat(data []byte) error {
	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return err
	}
	v, ok := raw["entries"]
	if !ok || v == nil {
		return nil
	}
	switch entries := v.(type) {
	case map[string]interface{}:
		for _, dest := range sortedKeys(entries) {
			item := entries[dest]
			m, ok := item.(map[string]interface{})
			if !ok {
				return fmt.Errorf("entries[%q]: table expected", dest)
			}
			if _, ok := m["target"]; ok {
				return fmt.Errorf("entries[%q]: old target format is no longer supported (use targets = { win = { src = \"...\" } })", dest)
			}
			for field := range m {
				if field != "src" && field != "targets" && field != "override" {
					return fmt.Errorf("entries[%q]: unknown field %q", dest, field)
				}
			}
			if ov, ok := m["override"]; ok && ov != nil {
				if _, ok := ov.(bool); !ok {
					return fmt.Errorf("entries[%q]: override must be a boolean", dest)
				}
			}
			tv, ok := m["targets"]
			if !ok || tv == nil {
				continue
			}
			switch targets := tv.(type) {
			case map[string]interface{}:
				for _, tname := range sortedKeys(targets) {
					titem := targets[tname]
					tm, ok := titem.(map[string]interface{})
					if !ok {
						return fmt.Errorf("entries[%q].targets[%q]: table expected", dest, tname)
					}
					for field := range tm {
						if field != "src" && field != "override" {
							return fmt.Errorf("entries[%q].targets[%q]: unknown field %q", dest, tname, field)
						}
					}
					if ov, ok := tm["override"]; ok && ov != nil {
						if _, ok := ov.(bool); !ok {
							return fmt.Errorf("entries[%q].targets[%q]: override must be a boolean", dest, tname)
						}
					}
				}
			case []interface{}:
				return oldTargetsArrayError(dest)
			case []map[string]interface{}:
				return oldTargetsArrayError(dest)
			default:
				msg := fmt.Sprintf("%T", tv)
				if strings.HasPrefix(msg, "[]") {
					return oldTargetsArrayError(dest)
				}
				return fmt.Errorf("entries[%q]: targets must be a table", dest)
			}
		}
		return nil
	case []interface{}:
		return fmt.Errorf("entries: old [[entries]] format is no longer supported (use [entries] with dest keys)")
	case []map[string]interface{}:
		return fmt.Errorf("entries: old [[entries]] format is no longer supported (use [entries] with dest keys)")
	default:
		// 配列デコードの別表現や想定外の型も旧形式または型不正として扱う。
		msg := fmt.Sprintf("%T", v)
		if strings.HasPrefix(msg, "[]") {
			return fmt.Errorf("entries: old [[entries]] format is no longer supported (use [entries] with dest keys)")
		}
		return fmt.Errorf("entries: table expected")
	}
}

// oldTargetsArrayError は旧 targets 配列形式に対する明確な失敗文言を返す。
func oldTargetsArrayError(dest string) error {
	return fmt.Errorf("entries[%q]: old targets array format is no longer supported (use targets = { win = { src = \"...\" } })", dest)
}

// Validate は配置先キー・{src}/{targets} 排他・空を検証する。
// 重複TargetはTOMLキー重複として構造上不可能なため検証しない。
func (c Config) Validate() error {
	dests := c.sortedDests()
	for _, dest := range dests {
		v := c.Entries[dest]
		if dest == "" {
			return fmt.Errorf("entries[%q]: dest must not be empty", dest)
		}
		hasSrc := v.Src != nil
		hasTargets := v.Targets != nil
		if hasSrc && hasTargets {
			return fmt.Errorf("entries[%q]: src and targets are mutually exclusive", dest)
		}
		if !hasSrc && !hasTargets {
			return fmt.Errorf("entries[%q]: either src or targets is required", dest)
		}
		if hasSrc {
			if *v.Src == "" {
				return fmt.Errorf("entries[%q]: src is required", dest)
			}
			continue
		}
		targets := *v.Targets
		if len(targets) == 0 {
			return fmt.Errorf("entries[%q]: targets must not be empty", dest)
		}
		for _, tname := range sortedKeys(targets) {
			tv := targets[tname]
			if tname == "" {
				return fmt.Errorf("entries[%q]: target must not be empty", dest)
			}
			if tv.Src == nil || *tv.Src == "" {
				return fmt.Errorf("entries[%q].targets[%q]: src is required", dest, tname)
			}
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ExpandDest は Entry の dest の ~ / ~/ を os.UserHomeDir() で展開する。
func ExpandDest(dest string) (string, error) {
	if dest != "~" && !strings.HasPrefix(dest, "~/") {
		return dest, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if dest == "~" {
		return home, nil
	}
	return filepath.Join(home, dest[2:]), nil
}

// ExpandedDest は Entry の dest を展開した配置先パスを返す。
func (e Entry) ExpandedDest() (string, error) {
	return ExpandDest(e.Dest)
}

// Template は init が作る mdots.toml 雛形である。
// サンプルはすべてコメントアウト済み。
// 生成物は Load/Validate を通る（Entry ゼロ件）。
const Template = `[entries]
# "~/.vimrc" = { src = "vimrc" }
#
# "~/.bashrc" = { src = "bashrc", override = false }
#
# "~/.gitconfig" = {
#   targets = {
#     win = { src = "win/.gitconfig" },
#     wsl = { src = "wsl/.gitconfig", override = false },
#   },
# }
`

// Init は指定ディレクトリ直下に mdots.toml 雛形を作り、作ったパスを返す。
// 既にあるときは mdots.toml already exists in <dir> 形式のエラーを返す。
func Init(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(abs, "mdots.toml")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("mdots.toml already exists in %s", abs)
		}
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(Template); err != nil {
		return "", err
	}
	return path, nil
}

// FindStore はカレント直下の mdots.toml のみを参照し、
// 見つかった Store ルートを返す。見つからないときは in <cwd> 形式のエラーを返す。
func FindStore(startDir string) (string, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(abs, "mdots.toml")); err == nil {
		return abs, nil
	}
	return "", fmt.Errorf("mdots.toml not found in %s", abs)
}

// NormalizeDest は add の入力 dest を Entry の配置先キー形（~/...）に正規化する。
// ~/... は残りを Clean して ~/... に整え、HOME 配下の絶対パスは ~/... に変換する。
// HOME 自体・~/ 直下は ~ に整える。それ以外はエラーを返す。
func NormalizeDest(raw string) (string, error) {
	if raw == "~" {
		return "~", nil
	}
	if strings.HasPrefix(raw, "~/") {
		rest := raw[2:]
		cleaned := filepath.Clean(rest)
		if cleaned == "." {
			return "~", nil
		}
		return "~/" + filepath.ToSlash(cleaned), nil
	}
	if filepath.IsAbs(raw) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		absClean := filepath.Clean(raw)
		homeClean := filepath.Clean(home)
		if absClean == homeClean {
			return "~", nil
		}
		if strings.HasPrefix(absClean, homeClean+string(os.PathSeparator)) {
			rel := strings.TrimPrefix(absClean, homeClean+string(os.PathSeparator))
			return "~/" + filepath.ToSlash(rel), nil
		}
	}
	return "", fmt.Errorf("dest must be ~/... or under HOME: %q", raw)
}

// SrcForDest は正規化済み dest から Store 相対の src を算出する。
// ~/ を除去し dotfiles/ を付ける。~ 自体は dotfiles に写像する。
func SrcForDest(normalizedDest string) (string, error) {
	if normalizedDest == "~" {
		return "dotfiles", nil
	}
	if strings.HasPrefix(normalizedDest, "~/") {
		rest := normalizedDest[2:]
		if rest == "" {
			return "dotfiles", nil
		}
		return "dotfiles/" + rest, nil
	}
	return "", fmt.Errorf("dest must be ~/...: %q", normalizedDest)
}

// SrcForDestWithTarget は正規化済み dest と Target から Store 相対の src を算出する。
// target 空時は SrcForDest と同じ。target 指定時は dotfiles/<target>/... に写像する。
// ~ 自体は dotfiles/<target> に写像する。
func SrcForDestWithTarget(normalizedDest, target string) (string, error) {
	if target == "" {
		return SrcForDest(normalizedDest)
	}
	if normalizedDest == "~" {
		return "dotfiles/" + target, nil
	}
	if strings.HasPrefix(normalizedDest, "~/") {
		rest := normalizedDest[2:]
		if rest == "" {
			return "dotfiles/" + target, nil
		}
		return "dotfiles/" + target + "/" + rest, nil
	}
	return "", fmt.Errorf("dest must be ~/...: %q", normalizedDest)
}

// Add は未登録の配置先を素の src 形式で新規 Entry として登録する。
// 既存 dest はエラーを返し、override は書かない（省略時 true として解決される）。
func (c *Config) Add(dest, src string) error {
	if dest == "" {
		return fmt.Errorf("entries[%q]: dest must not be empty", dest)
	}
	if src == "" {
		return fmt.Errorf("entries[%q]: src is required", dest)
	}
	if c.Entries == nil {
		c.Entries = map[string]EntryValue{}
	}
	if _, ok := c.Entries[dest]; ok {
		return fmt.Errorf("entries[%q]: already registered", dest)
	}
	s := src
	c.Entries[dest] = EntryValue{Src: &s}
	return nil
}

// AddTarget は配置先を targets 形式で登録する。
// dest 未登録なら新規 Entry を作り、targets 形式で登録済みかつ指定 Target が
// 未登録なら追記マージする。素の src 形式で登録済み、または同一 Target が
// 登録済みならエラーを返す。
// override は書かない（省略時 true として解決される）。
func (c *Config) AddTarget(dest, target, src string) error {
	if dest == "" {
		return fmt.Errorf("entries[%q]: dest must not be empty", dest)
	}
	if target == "" {
		return fmt.Errorf("entries[%q]: target must not be empty", dest)
	}
	if src == "" {
		return fmt.Errorf("entries[%q].targets[%q]: src is required", dest, target)
	}
	if c.Entries == nil {
		c.Entries = map[string]EntryValue{}
	}
	s := src
	if existing, ok := c.Entries[dest]; ok {
		if existing.Src != nil || existing.Targets == nil {
			return fmt.Errorf("entries[%q]: already registered", dest)
		}
		if _, ok := (*existing.Targets)[target]; ok {
			return fmt.Errorf("entries[%q]: already registered", dest)
		}
		merged := make(map[string]TargetValue, len(*existing.Targets)+1)
		for name, tv := range *existing.Targets {
			merged[name] = tv
		}
		merged[target] = TargetValue{Src: &s}
		c.Entries[dest] = EntryValue{Targets: &merged}
		return nil
	}
	c.Entries[dest] = EntryValue{Targets: &map[string]TargetValue{target: {Src: &s}}}
	return nil
}

// Save は Config 全体をテーブル形式で保存する。
// インデントは付けない。値を持たない空のテーブルヘッダ
// （[entries] を含む中間ヘッダ：直後が次のヘッダまたは末尾）は出さない。
// TOMLでは [a.b.c] だけで親が暗黙定義されるため、生成物は Load で読みに戻せる
// （ヘッダあり・なし・インライン・targets・override 混在）。
func (c Config) Save(path string) error {
	if err := c.Validate(); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = ""
	if err := enc.Encode(c); err != nil {
		return err
	}
	out := stripEmptyTableHeaders(buf.String())
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// stripEmptyTableHeaders は値行を持たないテーブルヘッダ行を取り除く。
// ヘッダ行の直後が次のヘッダ行または末尾の場合、そのヘッダは空とみなす。
// キー行が1つでも続くヘッダは残す（例: override を持つ親テーブル）。
func stripEmptyTableHeaders(out string) string {
	lines := strings.Split(out, "\n")
	var kept []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !isTableHeader(trimmed) {
			kept = append(kept, line)
			continue
		}
		nextIsHeaderOrEnd := true
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				continue
			}
			nextIsHeaderOrEnd = isTableHeader(strings.TrimSpace(next))
			break
		}
		if nextIsHeaderOrEnd {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// isTableHeader は TOML のテーブルヘッダ行かを報告する。
func isTableHeader(line string) bool {
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")
}
