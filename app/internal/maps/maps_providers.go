package maps

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func coordPart(n float64) string {
	if n < 0 {
		return fmt.Sprintf("m%08.5f", -n)
	}
	return fmt.Sprintf("p%08.5f", n)
}
func normMapStyle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "hybrid", "aerial", "photo":
		return "hybrid"
	case "standard", "map", "road", "roadmap", "":
		return "standard"
	default:
		return "standard"
	}
}
func normMapZoom(s string) int {
	z, _ := strconv.Atoi(s)
	if z < 1 {
		z = mapDefaultZoom
	}
	if z > 18 {
		z = 18
	}
	return z
}

type mapTileLayerGo struct {
	Name  string
	Tiles []string
}

type mapProviderGo struct {
	Name   string
	Label  string
	Kind   string
	Styles []string
	Tiles  []string
	Layers []mapTileLayerGo
}

var mapProvidersGo = []mapProviderGo{
	{Name: "staticmap-de", Label: "StaticMap DE standard fallback", Kind: "static", Styles: []string{"standard"}},
	{Name: "osm-carto", Label: "OpenStreetMap standard", Kind: "tiles", Styles: []string{"standard"}, Tiles: []string{"https://tile.openstreetmap.org/{z}/{x}/{y}.png"}},
	{Name: "osm-hot", Label: "OpenStreetMap HOT standard", Kind: "tiles", Styles: []string{"standard"}, Tiles: []string{"https://s.tile.openstreetmap.fr/hot/{z}/{x}/{y}.png", "https://b.tile.openstreetmap.fr/hot/{z}/{x}/{y}.png", "https://c.tile.openstreetmap.fr/hot/{z}/{x}/{y}.png"}},
	{Name: "osm-de", Label: "OpenStreetMap DE standard", Kind: "tiles", Styles: []string{"standard"}, Tiles: []string{"https://tile.openstreetmap.de/{z}/{x}/{y}.png"}},
	{Name: "esri-export-hybrid", Label: "Esri hybrid export", Kind: "arcgis_export", Styles: []string{"hybrid"}},
	{Name: "esri-hybrid", Label: "Esri hybrid imagery", Kind: "layered_tiles", Styles: []string{"hybrid"}, Layers: []mapTileLayerGo{
		{Name: "imagery", Tiles: []string{"https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}", "https://services.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}"}},
		{Name: "labels", Tiles: []string{"https://services.arcgisonline.com/ArcGIS/rest/services/Reference/World_Boundaries_and_Places/MapServer/tile/{z}/{y}/{x}", "https://server.arcgisonline.com/ArcGIS/rest/services/Reference/World_Boundaries_and_Places/MapServer/tile/{z}/{y}/{x}"}},
	}},
}

func mapProviderByNameGo(name string) (mapProviderGo, bool) {
	for _, p := range mapProvidersGo {
		if p.Name == name {
			return p, true
		}
	}
	return mapProviderGo{}, false
}

func mapProviderSupportsStyle(p mapProviderGo, style string) bool {
	style = normMapStyle(style)
	for _, s := range p.Styles {
		if normMapStyle(s) == style {
			return true
		}
	}
	return false
}

func (s *Service) mapProvidersForStyle(style string) []mapProviderGo {
	style = normMapStyle(style)
	out := []mapProviderGo{}
	for _, p := range mapProvidersGo {
		if mapProviderSupportsStyle(p, style) {
			out = append(out, p)
		}
	}
	return out
}

func (s *Service) mapProviderOrder(style string) []string {
	style = normMapStyle(style)
	providers := s.mapProvidersForStyle(style)
	names := []string{}
	nameOK := map[string]bool{}
	for _, p := range providers {
		names = append(names, p.Name)
		nameOK[p.Name] = true
	}
	state := jsonutil.Map(s.readJSONDefault(s.mapProviderFile(), map[string]any{}))
	primaryByStyle := jsonutil.Map(state["primaryByStyle"])
	primary := jsonutil.StringValue(primaryByStyle[style])
	if primary == "" || !nameOK[primary] {
		legacy := jsonutil.StringValue(state["primary"])
		if nameOK[legacy] {
			primary = legacy
		}
	}
	if style == "hybrid" {
		fast := []string{}
		for _, n := range names {
			if n != "esri-hybrid" {
				fast = append(fast, n)
			}
		}
		names = fast
		nameOK = map[string]bool{}
		for _, n := range names {
			nameOK[n] = true
		}
		if primary == "esri-hybrid" {
			primary = ""
		}
	}
	if primary == "" || !nameOK[primary] {
		return names
	}
	out := []string{primary}
	for _, n := range names {
		if n != primary {
			out = append(out, n)
		}
	}
	return out
}

func (s *Service) mapProviderStatus() map[string]any {
	state := jsonutil.Map(s.readJSONDefault(s.mapProviderFile(), map[string]any{}))
	primaryByStyle := jsonutil.Map(state["primaryByStyle"])
	failByName := map[string]map[string]any{}
	for _, raw := range jsonutil.List(state["failures"]) {
		m := jsonutil.Map(raw)
		if name := jsonutil.StringValue(m["provider"]); name != "" {
			failByName[name] = m
		}
	}
	providers := []map[string]any{}
	for _, p := range mapProvidersGo {
		styles := append([]string{}, p.Styles...)
		f := failByName[p.Name]
		primary := false
		for _, st := range styles {
			if jsonutil.StringValue(primaryByStyle[normMapStyle(st)]) == p.Name {
				primary = true
			}
		}
		if jsonutil.StringValue(state["primary"]) == p.Name {
			primary = true
		}
		providers = append(providers, map[string]any{"name": p.Name, "label": p.Label, "kind": p.Kind, "styles": styles, "primary": primary, "lastError": strOr(f["error"], ""), "lastFail": f["ts"]})
	}
	primary := jsonutil.StringValue(state["primary"])
	label := strOr(state["primaryLabel"], "")
	if label == "" && primary != "" {
		if p, ok := mapProviderByNameGo(primary); ok {
			label = p.Label
		}
	}
	if primary == "" {
		primary = "auto"
	}
	if label == "" {
		label = "auto"
	}
	return map[string]any{"primary": primary, "primaryLabel": label, "primaryByStyle": primaryByStyle, "lastOk": state["lastOk"], "lastFail": state["lastFail"], "lastError": strOr(state["lastError"], ""), "providers": providers, "styles": []map[string]any{{"name": "standard", "label": "Standard", "primary": defaultString(strOr(primaryByStyle["standard"], ""), "auto")}, {"name": "hybrid", "label": "Hybrid", "primary": defaultString(strOr(primaryByStyle["hybrid"], ""), "auto")}}}
}

func mapImageBase(lat, lon float64, z int, style string) string {
	style = normMapStyle(style)
	if z < 1 || z > 18 {
		z = mapDefaultZoom
	}
	return fmt.Sprintf("%s_z%d_%s_%s", style, z, coordPart(lat), coordPart(lon))
}

func legacyMapImageBase(lat, lon float64, z int) string {
	if z < 1 || z > 18 {
		z = mapDefaultZoom
	}
	return fmt.Sprintf("z%d_%s_%s", z, coordPart(lat), coordPart(lon))
}

func imageMimeFromExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "image/png"
	}
}

func imageExtFromMime(ctype string) (string, string) {
	c := strings.ToLower(ctype)
	switch {
	case strings.Contains(c, "jpeg") || strings.Contains(c, "jpg"):
		return ".jpg", "image/jpeg"
	case strings.Contains(c, "webp"):
		return ".webp", "image/webp"
	case strings.Contains(c, "svg"):
		return ".svg", "image/svg+xml"
	default:
		return ".png", "image/png"
	}
}

func isGoMapPlaceholder(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return false
	}
	if len(b) > 4096 {
		b = b[:4096]
	}
	s := string(b)
	return strings.Contains(s, "Go map preview") || strings.Contains(s, "Go map fallback") || strings.Contains(s, "Map unavailable")
}

func goodMapImage(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.Size() <= 1000 || time.Since(st.ModTime()) > 180*24*time.Hour {
		return false
	}
	if strings.EqualFold(filepath.Ext(path), ".svg") && isGoMapPlaceholder(path) {
		return false
	}
	_ = os.Chtimes(path, time.Now(), time.Now())
	return true
}

func (s *Service) cachedMapImagePath(lat, lon float64, z int, style string) (string, string) {
	bases := []string{mapImageBase(lat, lon, z, style)}
	if normMapStyle(style) == "standard" {
		bases = append(bases, legacyMapImageBase(lat, lon, z))
	}
	for _, base := range bases {
		for _, ext := range []string{".svg", ".png", ".jpg", ".jpeg", ".webp"} {
			p := filepath.Join(s.mapImageDir(), base+ext)
			if goodMapImage(p) {
				return p, imageMimeFromExt(p)
			}
		}
	}
	return "", ""
}

func tileCacheBase(providerName string, z, x, y int, layer string) string {
	safe := mapProviderNameSafe.ReplaceAllString(strings.ToLower(providerName), "_")
	if len(safe) > 48 {
		safe = safe[:48]
	}
	layer = mapProviderNameSafe.ReplaceAllString(strings.ToLower(layer), "_")
	if len(layer) > 24 {
		layer = layer[:24]
	}
	prefix := safe
	if layer != "" {
		prefix += "_" + layer
	}
	if z < 0 || z > 22 || x < 0 || y < 0 || prefix == "" {
		return ""
	}
	return fmt.Sprintf("%s_z%d_x%d_y%d", prefix, z, x, y)
}

func (s *Service) cachedTile(providerName string, z, x, y int, layer string) ([]byte, string, bool) {
	base := tileCacheBase(providerName, z, x, y, layer)
	if base == "" {
		return nil, "", false
	}
	for _, em := range []struct{ ext, mime string }{{".png", "image/png"}, {".jpg", "image/jpeg"}, {".jpeg", "image/jpeg"}, {".webp", "image/webp"}} {
		p := filepath.Join(s.mapTileDir(), base+em.ext)
		st, err := os.Stat(p)
		if err == nil && st.Size() > 50 && time.Since(st.ModTime()) < 60*24*time.Hour {
			b, err := os.ReadFile(p)
			if err == nil {
				_ = os.Chtimes(p, time.Now(), time.Now())
				return b, em.mime, true
			}
		}
	}
	return nil, "", false
}

func fetchMapURL(rawURL string, timeout time.Duration, maxBytes int64) ([]byte, string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Dash-Go/1.5.2-beta.2 local-kiosk map preview (+local cache)")
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ctype := strings.ToLower(resp.Header.Get("Content-Type"))
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(b)) > maxBytes {
		return nil, "", fmt.Errorf("response too large")
	}
	if !strings.Contains(ctype, "image") && !strings.Contains(ctype, "octet-stream") {
		return nil, "", fmt.Errorf("not an image")
	}
	if len(b) < 50 {
		return nil, "", fmt.Errorf("image response too small")
	}
	if ctype == "" {
		ctype = "image/png"
	}
	return b, ctype, nil
}

func (s *Service) fetchTile(p mapProviderGo, rawURL string, z, x, y int, layer string) ([]byte, string, error) {
	if b, mime, ok := s.cachedTile(p.Name, z, x, y, layer); ok {
		return b, mime, nil
	}
	b, ctype, err := fetchMapURL(rawURL, 2*time.Second, 2*1024*1024)
	if err != nil {
		return nil, "", err
	}
	ext, mime := imageExtFromMime(ctype)
	base := tileCacheBase(p.Name, z, x, y, layer)
	if base != "" {
		_ = os.MkdirAll(s.mapTileDir(), 0755)
		final := filepath.Join(s.mapTileDir(), base+ext)
		tmp := final + ".tmp"
		if os.WriteFile(tmp, b, 0644) == nil {
			_ = os.Rename(tmp, final)
		} else {
			_ = os.Remove(tmp)
		}
	}
	return b, mime, nil
}
