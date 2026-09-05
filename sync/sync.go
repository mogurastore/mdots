package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mogurastore/mdots/config"
)

// Push は Store の src を dest へファイルコピーする。
// Entry の src は Store 相対、dest は ~ 展開される配置先パス。
// 親ディレクトリは mkdir -p、パーミッションは元ファイルに追従、上書きは無確認。
// src/dest がディレクトリの場合はエラーになる。src 不在もエラーで中断する。
func Push(storeRoot string, entries []config.Entry) error {
	for _, e := range entries {
		srcPath := filepath.Join(storeRoot, e.Src)
		destPath, err := config.ExpandDest(e.Dest)
		if err != nil {
			return fmt.Errorf("push %s: dest expand: %w", e.Src, err)
		}
		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("push %s: %w", e.Src, err)
		}
	}
	return nil
}

// Pull は dest を Store の src へファイルコピーする。
// Entry の src は Store 相対、dest は ~ 展開される配置先パス。
// Push と同じく親ディレクトリは mkdir -p、パーミッションは元ファイルに追従、上書きは無確認。
// dest が不在・ディレクトリの場合はエラーで中断する。Store 側がディレクトリの場合もエラーになる。
func Pull(storeRoot string, entries []config.Entry) error {
	for _, e := range entries {
		destPath, err := config.ExpandDest(e.Dest)
		if err != nil {
			return fmt.Errorf("pull %s: dest expand: %w", e.Src, err)
		}
		srcPath := filepath.Join(storeRoot, e.Src)
		if err := copyFile(destPath, srcPath); err != nil {
			return fmt.Errorf("pull %s: %w", e.Src, err)
		}
	}
	return nil
}

func copyFile(srcPath, destPath string) error {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("src is a directory: %s", srcPath)
	}
	if destInfo, err := os.Stat(destPath); err == nil && destInfo.IsDir() {
		return fmt.Errorf("dest is a directory: %s", destPath)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if err := os.Chmod(destPath, srcInfo.Mode().Perm()); err != nil {
		return err
	}
	return nil
}
