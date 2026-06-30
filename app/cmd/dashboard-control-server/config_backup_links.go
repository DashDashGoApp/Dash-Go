package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	maxConfigBackupCalendarLinks      = 512
	calendarBackupLinkRootHome        = "home"
	calendarBackupLinkRootSystem      = "system-calendars"
	calendarBackupSystemCalendarsRoot = "/Calendars"
)

// calendarBackupLink is a portable, root-relative representation of a direct
// calendar-directory symlink. Root is intentionally a small fixed enum; Target
// is never an arbitrary filesystem path in new backups. Legacy backups that
// carried a raw Target are normalized during validation before any restore work.
type calendarBackupLink struct {
	Path   string `json:"path"`
	Root   string `json:"root,omitempty"`
	Target string `json:"target"`
}

type calendarBackupLinkPolicy struct {
	HomeRoot            string
	SystemCalendarsRoot string
	CalendarDir         string
}

func (a *app) calendarBackupLinkPolicy() calendarBackupLinkPolicy {
	return calendarBackupLinkPolicy{
		HomeRoot:            filepath.Clean(a.home),
		SystemCalendarsRoot: calendarBackupSystemCalendarsRoot,
		CalendarDir:         filepath.Clean(a.calDir),
	}
}

// safeCalendarBackupLinkPath admits only direct calendar .ics link names. The
// link name is still intentionally narrow: backups never create nested or
// traversal-controlled destinations under the live calendar directory.
func safeCalendarBackupLinkPath(path string) bool {
	if path == "" || strings.TrimSpace(path) != path || strings.ContainsRune(path, 0) || strings.Contains(path, "/") || strings.Contains(path, "\\") {
		return false
	}
	if !strings.EqualFold(filepath.Ext(path), ".ics") {
		return false
	}
	return safeBackupEntry("calendars/" + path)
}

func safeCalendarBackupRelativeTarget(target string) bool {
	if target == "" || strings.TrimSpace(target) != target || strings.ContainsRune(target, 0) || filepath.IsAbs(target) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(target))
	if clean == "." || clean == "" || filepath.IsAbs(clean) || clean != filepath.FromSlash(target) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return false
	}
	return strings.EqualFold(filepath.Ext(clean), ".ics")
}

func calendarPathWithin(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

func (p calendarBackupLinkPolicy) trustedRoot(kind string) (string, bool) {
	var root string
	switch kind {
	case calendarBackupLinkRootHome:
		root = p.HomeRoot
	case calendarBackupLinkRootSystem:
		root = p.SystemCalendarsRoot
	default:
		return "", false
	}
	root = filepath.Clean(root)
	return root, root != "." && root != ""
}

// validateExistingCalendarTarget keeps an allowed lexical path from being
// redirected outside its allowed root through an existing symlinked ancestor.
// A missing target is valid: broken calendar links are a supported sync/mount
// state, provided their lexical path stays under one of the fixed roots.
func validateExistingCalendarTarget(root, target string) error {
	if !calendarPathWithin(root, target) {
		return errors.New("calendar link target escapes its trusted root")
	}
	resolvedRoot := filepath.Clean(root)
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		resolvedRoot = filepath.Clean(resolved)
	} else if !os.IsNotExist(err) {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	current := filepath.Clean(root)
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			// An intermediate broken symlink could redirect to an unknown tree.
			// Reject it; a directly missing calendar target is handled above.
			return fmt.Errorf("calendar link target has an unresolved symlink ancestor: %w", err)
		}
		if !calendarPathWithin(resolvedRoot, resolved) && filepath.Clean(resolved) != resolvedRoot {
			return errors.New("calendar link target resolves outside its trusted root")
		}
	}
	// An existing endpoint must be an ordinary calendar file. A direct source
	// link may legitimately be broken when a sync mount is offline, but an
	// existing directory, device, FIFO, or other special entry is never an
	// acceptable calendar target.
	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("calendar link target is not a regular file")
	}
	return nil
}

func (p calendarBackupLinkPolicy) classifyAbsoluteTarget(target string) (string, string, error) {
	if target == "" || strings.ContainsRune(target, 0) || !filepath.IsAbs(target) || !strings.EqualFold(filepath.Ext(target), ".ics") {
		return "", "", errors.New("calendar link target is invalid")
	}
	target = filepath.Clean(target)
	for _, kind := range []string{calendarBackupLinkRootHome, calendarBackupLinkRootSystem} {
		root, ok := p.trustedRoot(kind)
		if !ok || !calendarPathWithin(root, target) {
			continue
		}
		if err := validateExistingCalendarTarget(root, target); err != nil {
			return "", "", err
		}
		rel, err := filepath.Rel(root, target)
		if err != nil || !safeCalendarBackupRelativeTarget(filepath.ToSlash(rel)) {
			return "", "", errors.New("calendar link target is invalid")
		}
		return kind, filepath.ToSlash(rel), nil
	}
	return "", "", errors.New("calendar link target is outside trusted calendar roots")
}

func (p calendarBackupLinkPolicy) normalizeLink(link calendarBackupLink) (calendarBackupLink, error) {
	if !safeCalendarBackupLinkPath(link.Path) {
		return calendarBackupLink{}, fmt.Errorf("backup contains an unsafe calendar link path: %s", link.Path)
	}
	if link.Target == "" || strings.ContainsRune(link.Target, 0) {
		return calendarBackupLink{}, fmt.Errorf("backup contains an invalid calendar link target: %s", link.Path)
	}
	if link.Root == "" {
		// Legacy backups carried a raw link text. Resolve it only lexically from
		// the known direct calendar directory, then convert it to the new fixed
		// root + relative target representation before it reaches os.Symlink.
		legacy := filepath.FromSlash(link.Target)
		if !filepath.IsAbs(legacy) {
			legacy = filepath.Join(p.CalendarDir, legacy)
		}
		root, target, err := p.classifyAbsoluteTarget(filepath.Clean(legacy))
		if err != nil {
			return calendarBackupLink{}, fmt.Errorf("backup contains an unsupported legacy calendar link target: %s", link.Path)
		}
		return calendarBackupLink{Path: link.Path, Root: root, Target: target}, nil
	}
	root, ok := p.trustedRoot(link.Root)
	if !ok || !safeCalendarBackupRelativeTarget(link.Target) {
		return calendarBackupLink{}, fmt.Errorf("backup contains an invalid calendar link target: %s", link.Path)
	}
	absolute := filepath.Join(root, filepath.FromSlash(link.Target))
	if !calendarPathWithin(root, absolute) {
		return calendarBackupLink{}, fmt.Errorf("backup contains an unsafe calendar link target: %s", link.Path)
	}
	if err := validateExistingCalendarTarget(root, absolute); err != nil {
		return calendarBackupLink{}, fmt.Errorf("backup contains an unsafe calendar link target: %s: %w", link.Path, err)
	}
	return calendarBackupLink{Path: link.Path, Root: link.Root, Target: filepath.ToSlash(filepath.Clean(filepath.FromSlash(link.Target)))}, nil
}

func (p calendarBackupLinkPolicy) restoreTarget(link calendarBackupLink) (string, error) {
	canonical, err := p.normalizeLink(link)
	if err != nil {
		return "", err
	}
	root, ok := p.trustedRoot(canonical.Root)
	if !ok {
		return "", errors.New("calendar link root is unavailable")
	}
	target := filepath.Join(root, filepath.FromSlash(canonical.Target))
	if !calendarPathWithin(root, target) {
		return "", errors.New("calendar link target escapes its trusted root")
	}
	if err := validateExistingCalendarTarget(root, target); err != nil {
		return "", err
	}
	return target, nil
}

func normalizeCalendarBackupLinks(links []calendarBackupLink, policy calendarBackupLinkPolicy) ([]calendarBackupLink, error) {
	if len(links) > maxConfigBackupCalendarLinks {
		return nil, errors.New("backup contains too many calendar links")
	}
	seen := make(map[string]bool, len(links))
	out := make([]calendarBackupLink, 0, len(links))
	for _, link := range links {
		canonical, err := policy.normalizeLink(link)
		if err != nil {
			return nil, err
		}
		if seen[canonical.Path] {
			return nil, fmt.Errorf("backup contains duplicate calendar link metadata: %s", canonical.Path)
		}
		seen[canonical.Path] = true
		out = append(out, canonical)
	}
	slices.SortFunc(out, func(left, right calendarBackupLink) int { return compareText(left.Path, right.Path) })
	return out, nil
}

func (p calendarBackupLinkPolicy) linkFromFilesystem(root, path string) (calendarBackupLink, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return calendarBackupLink{}, err
	}
	name := filepath.ToSlash(rel)
	if !safeCalendarBackupLinkPath(name) {
		return calendarBackupLink{}, fmt.Errorf("backup refuses unsupported calendar symlink: %s", path)
	}
	rawTarget, err := os.Readlink(path)
	if err != nil {
		return calendarBackupLink{}, err
	}
	absolute := filepath.FromSlash(rawTarget)
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(filepath.Dir(path), absolute)
	}
	kind, target, err := p.classifyAbsoluteTarget(filepath.Clean(absolute))
	if err != nil {
		return calendarBackupLink{}, fmt.Errorf("backup refuses calendar symlink outside trusted roots: %s", path)
	}
	return calendarBackupLink{Path: name, Root: kind, Target: target}, nil
}
