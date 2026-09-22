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
// Entry は targets.<名>.<dest> のみに存在し、素の src 形式と [common] は持たない。
// default_target は省略時の解決先を示す共有固定値である。
// shared_dir は resolve の移動先基底となる Store 相対ディレクトリで、
// resolve 実行時のみ必須とし、通常の doctor では無視する。
type Config struct {
	DefaultTarget string                            `toml:"default_target,omitempty"`
	SharedDir     string                            `toml:"shared_dir,omitempty"`
	TargetsMap    map[string]map[string]TargetValue `toml:"targets"`
}

// TargetValue は Target 内の1つの配置先に対する参照元である。
// Src は必須、Override は任意指定で省略時は true として解決される。
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

// sortedDests は指定 Target 内の配置先キーをソート順で返す。
func (c Config) sortedDests(target string) []string {
	dests := make([]string, 0)
	if c.TargetsMap == nil {
		return dests
	}
	for dest := range c.TargetsMap[target] {
		dests = append(dests, dest)
	}
	sort.Strings(dests)
	return dests
}

// resolveTargetName は要求名を解決する。空文字は省略として default_target へ解決する。
// 省略時に default_target が空・欠落ならエラー、解決先が未定義ならエラーにする。
// 明示名が未定義ならエラーにする。いずれも Target 名と default_target/targets への hint を含む。
func (c Config) resolveTargetName(requested string) (string, error) {
	defined := c.Targets()
	if requested != "" {
		for _, name := range defined {
			if name == requested {
				return requested, nil
			}
		}
		return "", fmt.Errorf("unknown target %q: defined targets are [%s] (hint: check --target or default_target/targets in mdots.toml)", requested, strings.Join(defined, ", "))
	}
	if c.DefaultTarget == "" {
		return "", fmt.Errorf("default_target is not set (hint: set default_target or use --target; check targets in mdots.toml)")
	}
	for _, name := range defined {
		if name == c.DefaultTarget {
			return c.DefaultTarget, nil
		}
	}
	return "", fmt.Errorf("default_target %q is unknown: defined targets are [%s] (hint: check default_target/targets in mdots.toml)", c.DefaultTarget, strings.Join(defined, ", "))
}

// Resolve は単一 Target に対する解決済み Entry 群を配置先ソート順で返す。
// 空文字は省略として default_target へ解決する。一度の解決で複数 Target は合成しない。
// 重ね適用は push＋push -t wsl の二度押しで表現する。
// 未定義名・欠落した default_target・未定義を指す default_target はエラーにする。
func (c Config) Resolve(target string) ([]Entry, error) {
	resolved, err := c.resolveTargetName(target)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, dest := range c.sortedDests(resolved) {
		tv := c.TargetsMap[resolved][dest]
		override := true
		if tv.Override != nil {
			override = *tv.Override
		}
		var src string
		if tv.Src != nil {
			src = *tv.Src
		}
		out = append(out, Entry{Src: src, Dest: dest, Override: override})
	}
	return out, nil
}

// Targets は定義済み Target 名を重複排除・ソートして返す。
// 未定義時は空スライスを返す。
func (c Config) Targets() []string {
	seen := map[string]struct{}{}
	for tname := range c.TargetsMap {
		seen[tname] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for tname := range seen {
		out = append(out, tname)
	}
	sort.Strings(out)
	return out
}

// TargetsMarked は定義済み Target 名をソートし、既定 Target に印を付けて返す。
// 印の書式は "<名> (default)" とし、テストで固定する。
func (c Config) TargetsMarked() []string {
	names := c.Targets()
	out := make([]string, 0, len(names))
	for _, name := range names {
		if name != "" && name == c.DefaultTarget {
			out = append(out, name+" (default)")
			continue
		}
		out = append(out, name)
	}
	return out
}

// Load は Store の mdots.toml を読み込み、validation して返す。
// 未知フィールドは一律 unknown field で拒否し、型不正は Decode エラーに任せる。
// default_target の欠落・空文字はここでは許容し、解決時にエラーにする。
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, err
	}
	if err := rejectUnexpectedFormat(md); err != nil {
		return Config{}, err
	}
	if cfg.TargetsMap == nil {
		cfg.TargetsMap = map[string]map[string]TargetValue{}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// rejectUnexpectedFormat は想定外フォーマットを一律拒否する。
// 未知フィールドは unknown field で拒否する。
// BurntSushi/toml は map への非テーブル代入を黙って空にするため、
// targets のテーブル型はここで明示的に拒否する。
// ドットヘッダで暗黙生成された親テーブルは Type が "" になるため許容する。
func rejectUnexpectedFormat(md toml.MetaData) error {
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return fmt.Errorf("unknown field %q", undecoded[0].String())
	}
	if md.IsDefined("targets") {
		if typ := md.Type("targets"); typ != "Hash" && typ != "" {
			return fmt.Errorf("targets: table expected")
		}
	}
	return nil
}

// Validate は Target 名・配置先キー・src 必須・空を検証する。
// 同一 Target 内の同一 dest 重複は TOML キー重複として構造上不可能なため検証しない。
// 同一 dest の跨 Target 定義は許可する。default_target の欠落・空は解決時に扱いここでは許容する。
// shared_dir は resolve 実行時のみ必須とするため、ここでは検証せず無視する。
func (c Config) Validate() error {
	for _, tname := range sortedKeys(c.TargetsMap) {
		if tname == "" {
			return fmt.Errorf("targets[%q]: target must not be empty", tname)
		}
		dests := c.TargetsMap[tname]
		if dests == nil {
			continue
		}
		for _, dest := range sortedKeys(dests) {
			tv := dests[dest]
			if dest == "" {
				return fmt.Errorf("targets[%q][%q]: dest must not be empty", tname, dest)
			}
			if tv.Src == nil || *tv.Src == "" {
				return fmt.Errorf("targets[%q][%q]: src is required", tname, dest)
			}
		}
	}
	return nil
}

// ValidateSharedDir は resolve 用の共有基底を検証し、正規化済みの値を返す。
// 空・欠落は必須エラー、絶対パス・Store 外への脱出は相対エラーにする。
// doctor では呼ばず無視する。
func (c Config) ValidateSharedDir() (string, error) {
	v := strings.TrimSpace(c.SharedDir)
	if v == "" {
		return "", fmt.Errorf("shared_dir is not set (hint: set shared_dir in mdots.toml)")
	}
	slash := filepath.ToSlash(v)
	if filepath.IsAbs(v) || strings.HasPrefix(slash, "/") {
		return "", fmt.Errorf("shared_dir must be a relative path: %q (hint: set Store-relative shared_dir in mdots.toml)", c.SharedDir)
	}
	if slash == "~" || strings.HasPrefix(slash, "~/") {
		return "", fmt.Errorf("shared_dir must be a relative path: %q (hint: set Store-relative shared_dir in mdots.toml)", c.SharedDir)
	}
	parts := []string{}
	for _, part := range strings.Split(slash, "/") {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(parts) == 0 {
				return "", fmt.Errorf("shared_dir must not escape Store: %q (hint: set Store-relative shared_dir in mdots.toml)", c.SharedDir)
			}
			parts = parts[:len(parts)-1]
			continue
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("shared_dir must not be empty (hint: set shared_dir in mdots.toml)")
	}
	return strings.Join(parts, "/"), nil
}

// SaveAtomic は設定全体の完成形を一時書出して再読込検証してから置換する。
// mode は既存ファイルの権限を継承し、失敗時は元を不変に保つ。
func SaveAtomic(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = ""
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	out := stripEmptyTableHeaders(buf.String())
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mdots.toml.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(out); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, err := Load(tmpName); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	renamed = true
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
// default_target = "base" と shared_dir = "dotfiles/shared" を持ち、コメント・例示は含まない。
// base は予約語ではなく初期値例である。
// 生成物は Load/Validate を通る（Entry ゼロ件）。
const Template = `default_target = "base"
shared_dir = "dotfiles/shared"
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

// SrcForDestWithTarget は正規化済み dest と Target から Store 相対の src を算出する。
// dotfiles/<target>/... に写像する。~ 自体は dotfiles/<target> に写像する。
// target 空時はエラーを返す（無指定 plain 登録は廃止）。
func SrcForDestWithTarget(normalizedDest, target string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("targets[%q]: target must not be empty", target)
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

// AddTarget は配置先を targets 形式で登録する。
// 指定 Target 内の同一 dest が登録済みならエラーを返す。
// 同一 dest の跨 Target 定義は許可する。
// override は書かない（省略時 true として解決される）。
func (c *Config) AddTarget(dest, target, src string) error {
	if dest == "" {
		return fmt.Errorf("targets[%q][%q]: dest must not be empty", target, dest)
	}
	if target == "" {
		return fmt.Errorf("targets[%q]: target must not be empty", target)
	}
	if src == "" {
		return fmt.Errorf("targets[%q][%q]: src is required", target, dest)
	}
	if c.TargetsMap == nil {
		c.TargetsMap = map[string]map[string]TargetValue{}
	}
	s := src
	if dests, ok := c.TargetsMap[target]; ok {
		if _, ok := dests[dest]; ok {
			return fmt.Errorf("targets[%q][%q]: already registered", target, dest)
		}
		merged := make(map[string]TargetValue, len(dests)+1)
		for d, tv := range dests {
			merged[d] = tv
		}
		merged[dest] = TargetValue{Src: &s}
		c.TargetsMap[target] = merged
		return nil
	}
	c.TargetsMap[target] = map[string]TargetValue{dest: {Src: &s}}
	return nil
}

// AppendTargetEntry はtargets形式の新規1 Target分fragmentを生成し、
// 元ファイルに空行1行で区切って追記した完成形を再Loadして問題なければatomicに置換する。
// 新規dest・既存destへのTarget追記のいずれも末尾追記で統一する。
// 同一 Target 内の同一 dest 重複は再Loadで拒否し、跨 Target の同一 dest は許可する。
// 失敗時は元ファイルを不変に保つ。
func AppendTargetEntry(path, dest, target, src string) error {
	if dest == "" {
		return fmt.Errorf("targets[%q][%q]: dest must not be empty", target, dest)
	}
	if target == "" {
		return fmt.Errorf("targets[%q]: target must not be empty", target)
	}
	if src == "" {
		return fmt.Errorf("targets[%q][%q]: src is required", target, dest)
	}
	srcCopy := src
	entry := map[string]map[string]TargetValue{target: {dest: {Src: &srcCopy}}}
	return appendSingleEntry(path, entry)
}

// appendSingleEntry は1 Entry分のfragment化と追記＋検証＋置換を一本化する。
func appendSingleEntry(path string, entry map[string]map[string]TargetValue) error {
	fragment, err := encodeSingleEntry(entry)
	if err != nil {
		return err
	}
	return appendFragmentAtomic(path, fragment)
}

// encodeSingleEntry は1 Target分だけのConfigを Save と同じテーブル形式でfragment化する。
// default_target は含めない（追記先の既存値を温存する）。
func encodeSingleEntry(entry map[string]map[string]TargetValue) (string, error) {
	singleConfig := Config{TargetsMap: entry}
	if err := singleConfig.Validate(); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = ""
	if err := enc.Encode(singleConfig); err != nil {
		return "", err
	}
	out := stripEmptyTableHeaders(buf.String())
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}

// appendFragmentAtomic は元ファイルrawにfragmentを空行1行で区切って追記した完成形をtempに書き、
// 再Loadで検証してからrenameでatomicに置換する。元ファイルのmodeを継承し
// （取得不可・新規時は0644）、失敗時はtempを削除して元を不変に保つ。
// 空ファイル時はfragmentのみ、既に空行で終わる時は追加しない。末尾改行なしは補う。
func appendFragmentAtomic(path, fragment string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	combined := raw
	if len(combined) > 0 {
		if combined[len(combined)-1] != '\n' {
			combined = append(combined, '\n')
		}
		if len(combined) < 2 || combined[len(combined)-2] != '\n' {
			combined = append(combined, '\n')
		}
	}
	combined = append(combined, []byte(fragment)...)
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mdots.toml.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(combined); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, err := Load(tmpName); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	renamed = true
	return nil
}

// stripEmptyTableHeaders は値行を持たないテーブルヘッダ行を取り除く。
// ヘッダ行の直後が次のヘッダ行または末尾の場合、そのヘッダは空とみなす。
// キー行が1つでも続くヘッダは残す（例: override を持つ親テーブル）。
// default_target 行は値行として扱い、直前の空ヘッダ判定に影響させない。
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
