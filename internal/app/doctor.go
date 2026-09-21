// doctor は sharable 検出のみを行う。
//
// 検出条件は Store 上の別 src 間の内容一致のみとする。
// dest の異同は問わず、既に同一 src のものは対象外、比較は Store 上の
// src 同士のみで HOME と override・モードの異同は無視する。
// 部分一致は内容ハッシュでグループ化して報告する。
// 欠落ファイルは skip し、ディレクトリ・読み込み失敗はエラーとする。
// 自動修正はしない。出力は候補あり→一覧して exit 1、なし→
// No sharable entries. で exit 0 とし、dry-run の差分あり→1 と整合させる。
package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/mogurastore/mdots/internal/config"
)

// doctorRef は sharable 候補を構成する1つの参照元である。
type doctorRef struct {
	target string
	dest   string
	src    string
}

// doctorGroup は同一内容で共有化候補となる一群である。
type doctorGroup struct {
	refs []doctorRef
}

// Doctor は Store 全体の sharable を検出して報告する。
// 候補ありは stdout に一覧して 1、なしは No sharable entries. を出して 0。
// Store不在・toml不正・ディレクトリ・読み込み失敗は stderr に出して 1。
// mdots.toml と Store 内容は変更しない。HOME は見ない。
func Doctor(cwd string, stdout, stderr io.Writer) int {
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
	groups, err := findSharable(store, cfg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(groups) == 0 {
		fmt.Fprintln(stdout, "No sharable entries.")
		return 0
	}
	for _, g := range groups {
		fmt.Fprintln(stdout, "sharable:")
		for _, r := range g.refs {
			fmt.Fprintf(stdout, "  %s (target: %s, dest: %s)\n", r.src, r.target, r.dest)
		}
	}
	return 1
}

// findSharable は Store 全体で別 src 間の内容一致を集める。
// グループ・参照の順序はソートして決定的にする。
func findSharable(store string, cfg config.Config) ([]doctorGroup, error) {
	var refs []doctorRef
	for target, dests := range cfg.TargetsMap {
		for dest, tv := range dests {
			var src string
			if tv.Src != nil {
				src = *tv.Src
			}
			refs = append(refs, doctorRef{target: target, dest: dest, src: src})
		}
	}
	return sharableForRefs(store, refs)
}

// sharableForRefs は参照群に対する候補群を返す。
// 同一 src は1つに束ねて対象外とし、欠落は skip する。
func sharableForRefs(store string, refs []doctorRef) ([]doctorGroup, error) {
	srcToRef := map[string]doctorRef{}
	for _, r := range refs {
		if prev, ok := srcToRef[r.src]; ok {
			if r.target < prev.target || (r.target == prev.target && r.dest < prev.dest) {
				srcToRef[r.src] = r
			}
			continue
		}
		srcToRef[r.src] = r
	}
	if len(srcToRef) < 2 {
		return nil, nil
	}
	srcs := make([]string, 0, len(srcToRef))
	for src := range srcToRef {
		srcs = append(srcs, src)
	}
	sort.Strings(srcs)

	byHash := map[string][]doctorRef{}
	for _, src := range srcs {
		srcPath := filepath.Join(store, src)
		info, err := os.Stat(srcPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("doctor %s: %w", src, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("doctor %s: src is a directory: %s", src, srcPath)
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("doctor %s: %w", src, err)
		}
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		byHash[hash] = append(byHash[hash], srcToRef[src])
	}

	hashes := make([]string, 0, len(byHash))
	for h := range byHash {
		hashes = append(hashes, h)
	}
	sort.Strings(hashes)

	var groups []doctorGroup
	for _, h := range hashes {
		refs := byHash[h]
		if len(refs) < 2 {
			continue
		}
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].src != refs[j].src {
				return refs[i].src < refs[j].src
			}
			if refs[i].target != refs[j].target {
				return refs[i].target < refs[j].target
			}
			return refs[i].dest < refs[j].dest
		})
		groups = append(groups, doctorGroup{refs: refs})
	}
	// ハッシュ順は内容依存で非決定的に見えるため、出力順は先頭srcで整える。
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].refs[0].src < groups[j].refs[0].src
	})
	return groups, nil
}
