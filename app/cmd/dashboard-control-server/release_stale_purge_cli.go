package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Stale-managed-source cleanup is a separate transaction responsibility. It
// backs up only managed source files that disappeared from the new manifest.

func copyFileWithMode(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = out.Close()
			_ = os.Remove(dest)
		}
	}()
	if _, err := io.CopyBuffer(out, in, make([]byte, 128*1024)); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func (a *app) runPurgeStaleManagedCLI(args []string) int {
	fs := flag.NewFlagSet("purge-stale-managed", flag.ContinueOnError)
	root := fs.String("root", "", "installed dashboard root")
	manifestPath := fs.String("manifest", "", "installed manifest JSON")
	backup := fs.String("backup", "", "backup root")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *root == "" || *manifestPath == "" || *backup == "" {
		fmt.Fprintln(os.Stderr, "usage: --purge-stale-managed --root DIR --manifest FILE --backup DIR")
		return 64
	}
	manifest, err := readReleaseManifest(*manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	managed := map[string]bool{}
	for _, item := range manifest.Files {
		managed[item.Path] = true
	}
	type tree struct {
		dir      string
		suffixes map[string]bool
	}
	trees := []tree{
		{dir: "ui/css", suffixes: map[string]bool{".css": true}},
		{dir: "ui/js", suffixes: map[string]bool{".js": true}},
		{dir: "cmd/dashboard-control-server", suffixes: map[string]bool{".go": true}},
	}
	for _, tree := range trees {
		base := filepath.Join(*root, filepath.FromSlash(tree.dir))
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() || !tree.suffixes[strings.ToLower(filepath.Ext(d.Name()))] {
				return nil
			}
			rel, err := filepath.Rel(*root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if managed[rel] {
				return nil
			}
			if !safeReleasePath(rel) {
				return fmt.Errorf("unsafe stale path: %s", rel)
			}
			dest := filepath.Join(*backup, filepath.FromSlash(rel))
			if err := copyFileWithMode(path, dest); err != nil {
				return err
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			fmt.Println(rel)
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}
