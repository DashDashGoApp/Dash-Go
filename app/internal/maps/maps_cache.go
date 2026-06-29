package maps

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const mapDefaultZoom = 15
const mapLookupVersion = 4
const mapStatusCacheTTL = 15 * time.Second

func (s *Service) mapImageDir() string     { return filepath.Join(s.cacheDir, "map-cache-img") }
func (s *Service) mapTileDir() string      { return filepath.Join(s.cacheDir, "map-cache-tiles") }
func (s *Service) mapCacheFile() string    { return filepath.Join(s.cacheDir, "map-cache.json") }
func (s *Service) mapProviderFile() string { return filepath.Join(s.configDir, "map-provider.json") }

type cacheFile struct {
	Path, Name string
	Size       int64
	Mtime      int64
}

func (s *Service) cacheFiles(dir string, exts map[string]bool) []cacheFile {
	_ = os.MkdirAll(dir, 0755)
	ents, _ := os.ReadDir(dir)
	out := []cacheFile{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".tmp") {
			_ = os.Remove(filepath.Join(dir, n))
			continue
		}
		if !exts[strings.ToLower(filepath.Ext(n))] {
			continue
		}
		st, err := e.Info()
		if err == nil {
			out = append(out, cacheFile{filepath.Join(dir, n), n, st.Size(), st.ModTime().Unix()})
		}
	}
	return out
}
func styleFromName(n string) string {
	s := reMapCacheExtension.ReplaceAllString(n, "")
	if strings.HasPrefix(s, "hybrid_z") {
		return "hybrid"
	}
	if strings.HasPrefix(s, "standard_z") || strings.HasPrefix(s, "z") {
		return "standard"
	}
	return "unknown"
}
func locFromName(n string) string {
	s := reMapCacheExtension.ReplaceAllString(n, "")
	re := reMapCacheCoordinate
	if m := re.FindStringSubmatch(s); len(m) == 3 {
		return m[1] + "_" + m[2]
	}
	return "file:" + s
}

func trimFilesWithKept(files []cacheFile, maxBytes int64, maxFiles int, maxAgeDays int) (int, int64, []cacheFile) {
	now := time.Now().Unix()
	kept := make([]cacheFile, 0, len(files))
	rem := 0
	rb := int64(0)
	for _, f := range files {
		if now-f.Mtime > int64(maxAgeDays*86400) {
			if os.Remove(f.Path) == nil {
				rem++
				rb += f.Size
				continue
			}
		}
		kept = append(kept, f)
	}
	slices.SortFunc(kept, func(left, right cacheFile) int { return compareInt64s(left.Mtime, right.Mtime) })
	total := int64(0)
	for _, f := range kept {
		total += f.Size
	}
	failed := []cacheFile{}
	for len(kept) > 0 && (len(kept) > maxFiles || total > maxBytes) {
		f := kept[0]
		kept = kept[1:]
		if os.Remove(f.Path) == nil {
			rem++
			rb += f.Size
			total -= f.Size
		} else {
			failed = append(failed, f)
		}
	}
	return rem, rb, append(failed, kept...)
}
func mapImageCacheSummaryFromFiles(files []cacheFile, removed int, removedBytes int64) map[string]any {
	bytes := int64(0)
	locs := map[string]bool{}
	styles := map[string]int{}
	for _, f := range files {
		bytes += f.Size
		locs[locFromName(f.Name)] = true
		styles[styleFromName(f.Name)]++
	}
	return map[string]any{"removed": removed, "removedBytes": removedBytes, "count": len(files), "bytes": bytes, "locationCount": len(locs), "styleCounts": styles, "maxBytes": 128 * 1024 * 1024, "maxFiles": 1440, "maxLocations": 160, "maxAgeDays": 180}
}
func mapTileCacheSummaryFromFiles(files []cacheFile, removed int, removedBytes int64) map[string]any {
	bytes := int64(0)
	for _, f := range files {
		bytes += f.Size
	}
	return map[string]any{"removed": removed, "removedBytes": removedBytes, "count": len(files), "bytes": bytes, "maxBytes": 64 * 1024 * 1024, "maxFiles": 1800, "maxAgeDays": 60}
}

func (s *Service) cleanMapImageCache() map[string]any {
	summary, _ := s.cleanMapImageCacheWithFiles()
	s.invalidateMapStatusCache()
	return summary
}
func (s *Service) cleanMapImageCacheWithFiles() (map[string]any, []cacheFile) {
	ex := map[string]bool{".svg": true, ".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
	removed := 0
	rb := int64(0)
	files := s.cacheFiles(s.mapImageDir(), ex)
	remaining := make([]cacheFile, 0, len(files))
	for _, f := range files {
		st := styleFromName(f.Name)
		if st != "standard" && st != "hybrid" {
			if os.Remove(f.Path) == nil {
				removed++
				rb += f.Size
				continue
			}
		}
		remaining = append(remaining, f)
	}
	r, b, remaining := trimFilesWithKept(remaining, 128*1024*1024, 1440, 180)
	removed += r
	rb += b
	return mapImageCacheSummaryFromFiles(remaining, removed, rb), remaining
}
func (s *Service) cleanMapTileCache() map[string]any {
	summary, _ := s.cleanMapTileCacheWithFiles()
	s.invalidateMapStatusCache()
	return summary
}
func (s *Service) cleanMapTileCacheWithFiles() (map[string]any, []cacheFile) {
	ex := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
	files := s.cacheFiles(s.mapTileDir(), ex)
	r, b, remaining := trimFilesWithKept(files, 64*1024*1024, 1800, 60)
	return mapTileCacheSummaryFromFiles(remaining, r, b), remaining
}
func oldestNewest(files []cacheFile) (any, any) {
	if len(files) == 0 {
		return nil, nil
	}
	o, n := files[0].Mtime, files[0].Mtime
	for _, f := range files {
		o = min(o, f.Mtime)
		n = max(n, f.Mtime)
	}
	return o, n
}

// mapCacheStatus is shown only in lazy Dashboard Control cards. Directory
// walks can be expensive on a busy SD card, so retain a tiny snapshot between
// nearby status requests. Mutating cache operations explicitly invalidate it.
func (s *Service) mapCacheStatus() map[string]any {
	now := time.Now()
	s.mapStatusMu.Lock()
	defer s.mapStatusMu.Unlock()
	if s.mapStatusCache != nil && now.Sub(s.mapStatusAt) < mapStatusCacheTTL {
		return copyStatusMap(s.mapStatusCache)
	}
	result := s.mapCacheStatusWithCleanup(false)
	s.mapStatusCache = copyStatusMap(result)
	s.mapStatusAt = now
	return result
}

func (s *Service) invalidateMapStatusCache() {
	s.mapStatusMu.Lock()
	s.mapStatusCache = nil
	s.mapStatusAt = time.Time{}
	s.mapStatusMu.Unlock()
	s.invalidateSystemStatus()
}
func (s *Service) mapCacheStatusWithCleanup(clean bool) map[string]any {
	imageExts := map[string]bool{".svg": true, ".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
	tileExts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
	var img, tile map[string]any
	var imageFiles, tileFiles []cacheFile
	if clean {
		img, imageFiles = s.cleanMapImageCacheWithFiles()
		tile, tileFiles = s.cleanMapTileCacheWithFiles()
	} else {
		imageFiles = s.cacheFiles(s.mapImageDir(), imageExts)
		tileFiles = s.cacheFiles(s.mapTileDir(), tileExts)
		img = mapImageCacheSummaryFromFiles(imageFiles, 0, 0)
		tile = mapTileCacheSummaryFromFiles(tileFiles, 0, 0)
	}
	of, nf := oldestNewest(imageFiles)
	ot, nt := oldestNewest(tileFiles)
	gc := 0
	if m, ok := s.readJSONDefault(s.mapCacheFile(), map[string]any{}).(map[string]any); ok {
		gc = len(m)
	}
	result := map[string]any{"ok": true, "imageCount": img["count"], "imageBytes": img["bytes"], "imageLocationCount": img["locationCount"], "imageStyleCounts": img["styleCounts"], "imageMaxBytes": 128 * 1024 * 1024, "imageMaxFiles": 1440, "imageMaxLocations": 160, "imageMaxAgeDays": 180, "oldestImage": of, "newestImage": nf, "tileCount": tile["count"], "tileBytes": tile["bytes"], "tileMaxBytes": 64 * 1024 * 1024, "tileMaxFiles": 1800, "tileMaxAgeDays": 60, "oldestTile": ot, "newestTile": nt, "geocodeCount": gc, "provider": s.mapProviderStatus(), "mapStyles": []map[string]string{{"name": "standard", "label": "Standard"}, {"name": "hybrid", "label": "Hybrid"}}, "prewarmStyles": []string{"standard", "hybrid"}, "zoomLevels": []int{13, 15, 17}, "defaultZoom": 15, "defaultStyle": "standard", "prewarm": s.readJSONDefault(filepath.Join(s.cacheDir, "map-prewarm-state.json"), map[string]any{"running": false}), "lastCleanup": img, "lastTileCleanup": tile, "dir": s.mapImageDir(), "tileDir": s.mapTileDir()}
	if clean {
		s.invalidateMapStatusCache()
	}
	return result
}
func (s *Service) clearMapCache(clearGeocodes, clearProvider, clearTiles bool) map[string]any {
	ex := map[string]bool{".svg": true, ".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
	removed := 0
	rb := int64(0)
	for _, f := range s.cacheFiles(s.mapImageDir(), ex) {
		if os.Remove(f.Path) == nil {
			removed++
			rb += f.Size
		}
	}
	tr := 0
	tb := int64(0)
	if clearTiles {
		for _, f := range s.cacheFiles(s.mapTileDir(), map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}) {
			if os.Remove(f.Path) == nil {
				tr++
				tb += f.Size
			}
		}
	}
	gc := 0
	if clearGeocodes {
		if m, ok := s.readJSONDefault(s.mapCacheFile(), map[string]any{}).(map[string]any); ok {
			gc = len(m)
		}
		_ = os.Remove(s.mapCacheFile())
	}
	if clearProvider {
		_ = os.Remove(s.mapProviderFile())
	}
	s.invalidateMapStatusCache()
	return map[string]any{"ok": true, "removed": removed, "removedBytes": rb, "tilesRemoved": tr, "tileBytesRemoved": tb, "geocodesCleared": gc, "status": s.mapCacheStatus()}
}
