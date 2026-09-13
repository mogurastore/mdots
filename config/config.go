package config

import (
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
type EntryValue struct {
	Src     *string        `toml:"src"`
	Targets *[]TargetEntry `toml:"targets"`
}

// TargetEntry は Target ごとの参照元である。Target は単数文字列。
type TargetEntry struct {
	Target *string `toml:"target"`
	Src    *string `toml:"src"`
}

// Entry は解決済みの1つの管理対象を表す src/dest ペア。
// src は Store 相対のファイルパス、dest は ~ 展開される配置先パス。
type Entry struct {
	Src  string `toml:"src"`
	Dest string `toml:"dest"`
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
func (c Config) Resolve(target string) []Entry {
	dests := c.sortedDests()
	var out []Entry
	for _, dest := range dests {
		v := c.Entries[dest]
		if v.Src != nil {
			out = append(out, Entry{Src: *v.Src, Dest: dest})
			continue
		}
		if v.Targets == nil {
			continue
		}
		for _, te := range *v.Targets {
			if te.Target != nil && *te.Target == target && te.Src != nil {
				out = append(out, Entry{Src: *te.Src, Dest: dest})
				break
			}
		}
	}
	return out
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
		for dest, item := range entries {
			m, ok := item.(map[string]interface{})
			if !ok {
				return fmt.Errorf("entries[%q]: table expected", dest)
			}
			if _, ok := m["target"]; ok {
				return fmt.Errorf("entries[%q]: old target format is no longer supported (use targets = [{ target = \"...\", src = \"...\" }])", dest)
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

// Validate は配置先キー・{src}/{targets} 排他・空・重複を検証する。
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
		seen := map[string]struct{}{}
		for i, te := range targets {
			if te.Target == nil || *te.Target == "" {
				return fmt.Errorf("entries[%q].targets[%d]: target must not be empty", dest, i)
			}
			if te.Src == nil || *te.Src == "" {
				return fmt.Errorf("entries[%q].targets[%d]: src is required", dest, i)
			}
			if _, dup := seen[*te.Target]; dup {
				return fmt.Errorf("entries[%q]: duplicate target %q", dest, *te.Target)
			}
			seen[*te.Target] = struct{}{}
		}
	}
	return nil
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
// 見て使い方を理解できるよう日本語コメントで push/pull と
// src/dest/target を説明し、サンプル Entry はすべてコメントアウト済み。
// 生成物は Load/Validate を通る（Entry ゼロ件）。
const Template = `# mdots.toml — Store（このファイルがあるディレクトリ）直下で mdots を実行する。
# Store はカレント直下の mdots.toml のみ参照する。サブディレクトリからは実行できない。
#
# 使い方:
#   mdots push              # common のみを Store から dest へコピー
#   mdots push --target win # common + win を対象にする
#   mdots pull --target win # dest から Store へ回収する
#   mdots push --dry-run    # 差分を dest→Store方向に出力する（差分なし: exit 0、差分あり: exit 1）
#   mdots pull --dry-run    # 差分を Store→dest方向に出力する
#
# Entry（1つの管理対象）:
#   src    = Store相対のファイルパス
#   dest   = 配置先パス（~ / ~/... はホームに展開）
#   target = 配列で指定（省略時 common）。単一も ["win"] のように書く。複数指定も可（例: ["win", "wsl"]）。--target 未指定時は common のみ、指定時は common + 指定Target が対象
#
# コメントを外して使う。まず common の1件から始めるのがおすすめ。
#
# [[entries]]
# src = "vimrc"
# dest = "~/.vimrc"
#
# [[entries]]
# src = "wezterm.lua"
# dest = "~/.config/wezterm/wezterm.lua"
# target = ["win"]
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
