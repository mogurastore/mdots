// resolve は sharable 群を共有先に寄せる自動修正操作である。
//
// 検出条件は doctor と同じく Store 上の別 src 間の内容一致のみとし、
// 既に同一 src のものは対象外、比較は Store 上の src 同士のみで
// HOME と override・モードの異同は無視し、部分一致は内容ハッシュで
// グループ化する。欠落は skip し、ディレクトリ・読み込み失敗はエラーとする。
//
// 新 src の命名はパスコンポーネントの最長共通サフィックスとし、
// 共有基底配下に置く。共通なしは basename に fallback し、
// 別グループと衝突時は -2, -3 を付ける。
//
// 適用は全グループ一括・非対話で、入力収集・事前検証・一括適用の順に行う。
// 事前検証で全グループの移動先を検査し、同内容の既存は採用、
// 異内容が1件でもあれば設定と Store ファイルのいずれも変更せず終了する。
// 書換は設定ファイルの完成形を一時書出して再読込検証してから置換する
// atomic 置換に従い、新位置の内容は同一のため先頭の内容を用い、
// 旧ファイルは削除し、空になった親ディレクトリは残す。HOME は見ない。
// dest キーと src 以外の指定は温存する。
//
// 出力と終了は、成功時は resolved 一覧で 0、候補なしは
// No sharable entries. で 0、検証失敗などは標準エラーに出して 1 とする。
// --dry-run は持たない。
package app

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mogurastore/mdots/internal/config"
)

// resolvePlan は1グループ分の寄せ計画である。
type resolvePlan struct {
	group  doctorGroup
	newSrc string
	data   []byte
	mode   os.FileMode
}

// Resolve は sharable 群を shared_dir 配下に寄せる。
// Target解決の Resolve とは別概念で、sharable解消の resolve である。
// 成功時は stdout に resolved 一覧を出して 0、候補なしは
// No sharable entries. を出して 0。Store不在・設定不正・
// 検証失敗などは stderr に出して 1 とし、検証失敗時は
// 設定と Store ファイルのいずれも変更しない。HOME は見ない。
func Resolve(cwd string, stdout, stderr io.Writer) int {
	store, err := config.FindStore(cwd)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cfg, err := config.Load(filepath.Join(store, "mdots.toml"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	sharedDir, err := cfg.ValidateSharedDir()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	groups, err := findSharable(store, cfg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(groups) == 0 {
		fmt.Fprintln(stdout, "No sharable entries.")
		return 0
	}
	plans, err := buildResolvePlans(store, sharedDir, groups)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := validateResolveTargets(store, plans); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := applyResolvePlans(store, filepath.Join(store, "mdots.toml"), &cfg, plans); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "resolved:")
	for _, p := range plans {
		for _, r := range p.group.refs {
			fmt.Fprintf(stdout, "  %s <- %s (target: %s, dest: %s)\n", p.newSrc, r.src, r.target, r.dest)
		}
	}
	return 0
}

// buildResolvePlans は各グループの新 src・内容・モードを収集する。
// 順序は groups の決定的順序を保ち、新 src の衝突時は -2, -3 を付ける。
func buildResolvePlans(store, sharedDir string, groups []doctorGroup) ([]resolvePlan, error) {
	used := map[string]struct{}{}
	plans := make([]resolvePlan, 0, len(groups))
	for _, g := range groups {
		suffix := commonSuffix(g.refs)
		// filepath.Join は OS 区切りになるため slash に戻す。
		newSrc := filepath.ToSlash(filepath.Join(sharedDir, suffix))
		base := newSrc
		n := 2
		for {
			if _, ok := used[newSrc]; !ok {
				break
			}
			newSrc = fmt.Sprintf("%s-%d", base, n)
			n++
		}
		used[newSrc] = struct{}{}
		firstSrc := g.refs[0].src
		data, err := os.ReadFile(filepath.Join(store, firstSrc))
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", firstSrc, err)
		}
		mode := os.FileMode(0o644)
		if fi, err := os.Stat(filepath.Join(store, firstSrc)); err == nil {
			mode = fi.Mode().Perm()
		}
		plans = append(plans, resolvePlan{group: g, newSrc: newSrc, data: data, mode: mode})
	}
	return plans, nil
}

// commonSuffix は参照群の src 群から最長共通サフィックスを返す。
// 共通なしは先頭 src の basename に fallback する。
func commonSuffix(refs []doctorRef) string {
	if len(refs) == 0 {
		return ""
	}
	split := make([][]string, 0, len(refs))
	for _, r := range refs {
		slash := filepath.ToSlash(r.src)
		split = append(split, strings.Split(slash, "/"))
	}
	// refs は src ソート済みのため先頭が決定的である。
	n := len(split[0])
	for _, parts := range split[1:] {
		if len(parts) < n {
			n = len(parts)
		}
	}
	common := []string{}
	for i := 1; i <= n; i++ {
		cand := split[0][len(split[0])-i]
		match := true
		for _, parts := range split[1:] {
			if parts[len(parts)-i] != cand {
				match = false
				break
			}
		}
		if !match {
			break
		}
		common = append([]string{cand}, common...)
	}
	if len(common) == 0 {
		last := split[0][len(split[0])-1]
		if last == "" || last == "." {
			last = split[0][0]
		}
		return last
	}
	return strings.Join(common, "/")
}

// validateResolveTargets は全グループの移動先を事前検査する。
// 同内容の既存は採用、異内容が1件でもあればエラーを返し、
// 呼び出し元は設定と Store ファイルのいずれも変更しない。
// ディレクトリ・読み込み失敗はエラーとする。
func validateResolveTargets(store string, plans []resolvePlan) error {
	// 決定的な報告のため newSrc 順に検査する。
	ordered := append([]resolvePlan(nil), plans...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].newSrc < ordered[j].newSrc })
	for _, p := range ordered {
		targetPath := filepath.Join(store, filepath.FromSlash(p.newSrc))
		info, err := os.Stat(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("resolve %s: %w", p.newSrc, err)
		}
		if info.IsDir() {
			return fmt.Errorf("resolve %s: src is a directory: %s", p.newSrc, targetPath)
		}
		got, err := os.ReadFile(targetPath)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", p.newSrc, err)
		}
		if !bytes.Equal(got, p.data) {
			return fmt.Errorf("resolve %s: already exists with different content: %s", p.newSrc, targetPath)
		}
	}
	return nil
}

// applyResolvePlans は新ファイル作成・設定置換・旧ファイル削除を一括で行う。
// dest キーと src 以外の指定は温存し、空になった親ディレクトリは残す。HOME は見ない。
// Target解決の Resolve とは別概念で、sharable解消の resolve である。
func applyResolvePlans(store, tomlPath string, cfg *config.Config, plans []resolvePlan) error {
	created := []string{}
	cleanup := func() {
		for _, p := range created {
			_ = os.Remove(p)
		}
	}
	for _, p := range plans {
		targetPath := filepath.Join(store, filepath.FromSlash(p.newSrc))
		if _, err := os.Stat(targetPath); err == nil {
			// 同内容の既存は採用済みのため書かない。異内容は事前検証で弾いている。
			continue
		} else if !os.IsNotExist(err) {
			cleanup()
			return fmt.Errorf("resolve %s: %w", p.newSrc, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			cleanup()
			return fmt.Errorf("resolve %s: %w", p.newSrc, err)
		}
		if err := os.WriteFile(targetPath, p.data, p.mode); err != nil {
			cleanup()
			return fmt.Errorf("resolve %s: %w", p.newSrc, err)
		}
		if err := os.Chmod(targetPath, p.mode); err != nil {
			cleanup()
			return fmt.Errorf("resolve %s: %w", p.newSrc, err)
		}
		created = append(created, targetPath)
	}
	next := *cfg
	if next.TargetsMap == nil {
		next.TargetsMap = map[string]map[string]config.TargetValue{}
	}
	for _, p := range plans {
		for _, r := range p.group.refs {
			dests, ok := next.TargetsMap[r.target]
			if !ok {
				continue
			}
			tv, ok := dests[r.dest]
			if !ok {
				continue
			}
			s := p.newSrc
			tv.Src = &s
			dests[r.dest] = tv
		}
	}
	if err := config.SaveAtomic(tomlPath, next); err != nil {
		cleanup()
		return err
	}
	*cfg = next
	for _, p := range plans {
		for _, r := range p.group.refs {
			if r.src == p.newSrc {
				continue
			}
			oldPath := filepath.Join(store, filepath.FromSlash(r.src))
			if err := os.Remove(oldPath); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("resolve %s: %w", r.src, err)
			}
		}
	}
	return nil
}
