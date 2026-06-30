package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func safeBackupEntry(name string) bool {
	name = filepath.ToSlash(strings.TrimSpace(name))
	if name == "" || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") {
		return false
	}
	for p := range strings.SplitSeq(name, "/") {
		if p == "" || p == "." || p == ".." {
			return false
		}
	}
	return true
}

func addZipFile(z *zip.Writer, source, name string, mode os.FileMode) error {
	if !safeBackupEntry(name) {
		return fmt.Errorf("unsafe backup path: %s", name)
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("backup refuses non-regular file: %s", source)
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.Method = zip.Deflate
	header.SetMode(mode)
	w, err := z.CreateHeader(header)
	if err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	_, copyErr := io.CopyBuffer(w, in, make([]byte, 128*1024))
	closeErr := in.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// addZipTree archives regular files only. Passing calendarLinks permits one
// narrow exception: direct .ics symlinks in the calendar root are represented
// as metadata instead of being followed or copied into the ZIP.
func (a *app) addZipTree(z *zip.Writer, root, prefix string, calendarLinks *[]calendarBackupLink) (int, error) {
	count := 0
	st, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !st.IsDir() {
		return 0, fmt.Errorf("backup refuses non-directory tree: %s", root)
	}
	policy := a.calendarBackupLinkPolicy()
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if calendarLinks == nil {
				return fmt.Errorf("backup refuses non-regular file: %s", p)
			}
			link, err := policy.linkFromFilesystem(root, p)
			if err != nil {
				return err
			}
			*calendarLinks = append(*calendarLinks, link)
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup refuses non-regular file: %s", p)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		name := strings.TrimRight(prefix, "/") + "/" + filepath.ToSlash(rel)
		if !safeBackupEntry(name) {
			return fmt.Errorf("unsafe backup path: %s", name)
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate
		w, err := z.CreateHeader(header)
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyBuffer(w, in, make([]byte, 128*1024))
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		count++
		return nil
	})
	return count, err
}

func validateCalendarBackupLinksAgainstArchive(links []calendarBackupLink, seen map[string]bool) error {
	for _, link := range links {
		name := "calendars/" + link.Path
		if seen[name] {
			return fmt.Errorf("calendar link metadata conflicts with archived file: %s", link.Path)
		}
	}
	return nil
}

func (a *app) validateConfigBackupArchive(path string) (int, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return 0, err
	}
	defer zr.Close()
	if len(zr.File) == 0 || len(zr.File) > 20000 {
		return 0, errors.New("backup archive entry count is invalid")
	}
	seen := map[string]bool{}
	metaFound := false
	for _, f := range zr.File {
		if !safeBackupEntry(f.Name) || seen[f.Name] {
			return 0, fmt.Errorf("unsafe or duplicate backup entry: %s", f.Name)
		}
		seen[f.Name] = true
		if f.Name == "backup-meta.json" {
			metaFound = true
		}
		rc, err := f.Open()
		if err != nil {
			return 0, err
		}
		if _, err := io.CopyBuffer(io.Discard, rc, make([]byte, 128*1024)); err != nil {
			_ = rc.Close()
			return 0, err
		}
		if err := rc.Close(); err != nil {
			return 0, err
		}
	}
	if !metaFound {
		return 0, errors.New("backup archive metadata is missing")
	}
	links, err := a.configBackupCalendarLinks(zr)
	if err != nil {
		return 0, err
	}
	if err := validateCalendarBackupLinksAgainstArchive(links, seen); err != nil {
		return 0, err
	}
	return len(zr.File), nil
}
