// doctor は sharable 検出のみを行う。
//
// 検出条件は同一 dest × 別 src × Store 上の内容一致とする。
// dest 違いの内容一致と既に同一 src のものは対象外、比較は Store 上の
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
	src    string
}

// doctorGroup は同一 dest・同一内容で共有化候補となる一群である。
type doctorGroup struct {
	dest string
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
		fmt.Fprintf(stdout, "sharable: %s\n", g.dest)
		for _, r := range g.refs {
			fmt.Fprintf(stdout, "  %s (target: %s)\n", r.src, r.target)
		}
	}
	return 1
}

// findSharable は同一 dest ごとに別 src 間の内容一致を集める。
// dest・グループ・参照の順序はソートして決定的にする。
func findSharable(store string, cfg config.Config) ([]doctorGroup, error) {
	byDest := map[string][]doctorRef{}
	for target, dests := range cfg.TargetsMap {
		for dest, tv := range dests {
			var src string
			if tv.Src != nil {
				src = *tv.Src
			}
			byDest[dest] = append(byDest[dest], doctorRef{target: target, src: src})
		}
	}
	dests := make([]string, 0, len(byDest))
	for dest := range byDest {
		dests = append(dests, dest)
	}
	sort.Strings(dests)

	var out []doctorGroup
	for _, dest := range dests {
		groups, err := sharableForDest(store, dest, byDest[dest])
		if err != nil {
			return nil, err
		}
		out = append(out, groups...)
	}
	return out, nil
}

// sharableForDest は1つの dest に対する候補群を返す。
// 同一 src は1つに束ねて対象外とし、欠落は skip する。
func sharableForDest(store, dest string, refs []doctorRef) ([]doctorGroup, error) {
	srcToTarget := map[string]string{}
	for _, r := range refs {
		if prev, ok := srcToTarget[r.src]; ok {
			if r.target < prev {
				srcToTarget[r.src] = r.target
			}
			continue
		}
		srcToTarget[r.src] = r.target
	}
	if len(srcToTarget) < 2 {
		return nil, nil
	}
	srcs := make([]string, 0, len(srcToTarget))
	for src := range srcToTarget {
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
		byHash[hash] = append(byHash[hash], doctorRef{target: srcToTarget[src], src: src})
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
			return refs[i].target < refs[j].target
		})
		groups = append(groups, doctorGroup{dest: dest, refs: refs})
	}
	// ハッシュ順は内容依存で非決定的に見えるため、出力順は先頭srcで整える。
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].refs[0].src < groups[j].refs[0].src
	})
	return groups, nil
}
