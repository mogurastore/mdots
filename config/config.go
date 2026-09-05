package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config は Store 直下の mdots.toml 全体を表す。
type Config struct {
	Entries []Entry `toml:"entries"`
}

// Entry は1つの管理対象を表す src/dest ペア。
// src は Store 相対のファイルパス、dest は ~ 展開される配置先パス。
// Target は適用先を識別する自由文字列。省略時は common として扱う。
type Entry struct {
	Src    string     `toml:"src"`
	Dest   string     `toml:"dest"`
	Target TargetList `toml:"target,omitempty"`
}

// TargetList は string | string[] を受け付ける Target の集合。
type TargetList []string

// UnmarshalTOML は string | string[] の両方を受け付ける。
func (t *TargetList) UnmarshalTOML(value interface{}) error {
	switch v := value.(type) {
	case nil:
		*t = nil
		return nil
	case string:
		*t = TargetList{v}
		return nil
	case []interface{}:
		out := make(TargetList, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("target must be string or string[]")
			}
			out = append(out, s)
		}
		*t = out
		return nil
	case []string:
		*t = TargetList(v)
		return nil
	default:
		return fmt.Errorf("target must be string or string[]")
	}
}

// IsCommon は Entry が common (Target 未指定または common) かを返す。
func (e Entry) IsCommon() bool {
	if len(e.Target) == 0 {
		return true
	}
	for _, t := range e.Target {
		if t == "common" {
			return true
		}
	}
	return false
}

// MatchesTarget は Entry が指定 Target の対象かを返す。
// Target 未指定時は common のみ、指定時は common + 指定 Target が対象。
// Target が配列のときはいずれかが一致すれば対象。
func (e Entry) MatchesTarget(target string) bool {
	if target == "" {
		return e.IsCommon()
	}
	if e.IsCommon() {
		return true
	}
	for _, t := range e.Target {
		if t == target {
			return true
		}
	}
	return false
}

// FilterByTarget は Entry 群から指定 Target の対象のみを返す。
func FilterByTarget(entries []Entry, target string) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.MatchesTarget(target) {
			out = append(out, e)
		}
	}
	return out
}

// Load は Store の mdots.toml を読み込み、validation して返す。
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate は Entry の src/dest 必須を検証する。
func (c Config) Validate() error {
	for i, e := range c.Entries {
		if e.Src == "" {
			return fmt.Errorf("entries[%d]: src is required", i)
		}
		if e.Dest == "" {
			return fmt.Errorf("entries[%d]: dest is required", i)
		}
		for _, t := range e.Target {
			if t == "" {
				return fmt.Errorf("entries[%d]: target must not be empty", i)
			}
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
