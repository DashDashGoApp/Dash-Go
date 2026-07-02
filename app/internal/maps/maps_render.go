package maps

import (
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func slippyXY(lat, lon float64, zoom int) (float64, float64) {
	if lat < -85.05112878 {
		lat = -85.05112878
	}
	if lat > 85.05112878 {
		lat = 85.05112878
	}
	n := math.Pow(2, float64(zoom))
	latRad := lat * math.Pi / 180
	y := (1.0 - math.Asinh(math.Tan(latRad))/math.Pi) / 2.0 * n
	return (lon + 180.0) / 360.0 * n * 256.0, y * 256.0
}

func tileBounds(lat, lon float64, zoom, width, height int) (int, float64, float64, int, int, int, int, int) {
	if zoom < 1 {
		zoom = 1
	}
	if zoom > 18 {
		zoom = 18
	}
	cx, cy := slippyXY(lat, lon, zoom)
	left, top := cx-float64(width)/2, cy-float64(height)/2
	n := 1 << zoom
	x0 := int(math.Floor(left / 256.0))
	x1 := int(math.Floor((left + float64(width)) / 256.0))
	y0 := int(math.Floor(top / 256.0))
	y1 := int(math.Floor((top + float64(height)) / 256.0))
	return zoom, left, top, n, x0, x1, y0, y1
}

func markerSVG(width, height int) string {
	mx, my := float64(width)/2.0, float64(height)/2.0
	pinPath := fmt.Sprintf("M%.1f %.1f C %.1f %.1f %.1f %.1f %.1f %.1f C %.1f %.1f %.1f %.1f %.1f %.1f Z", mx, my+16, mx-18, my-8, mx-10, my-30, mx, my-30, mx+10, my-30, mx+18, my-8, mx, my+16)
	return fmt.Sprintf(`<g aria-hidden="true"><ellipse cx="%.1f" cy="%.1f" rx="16" ry="5" fill="#000000" opacity=".28"/><path d="%s" fill="#d94b4b" stroke="#000000" stroke-opacity=".42" stroke-width="5" stroke-linejoin="round" opacity=".42"/><path d="%s" fill="#d94b4b" stroke="#ffffff" stroke-width="2.8" stroke-linejoin="round"/><circle cx="%.1f" cy="%.1f" r="5.8" fill="#ffffff" stroke="#000000" stroke-opacity=".22" stroke-width="1"/></g>`, mx, my+18, pinPath, pinPath, mx, my-12)
}

func tileURLTemplate(tpl string, z, x, y int) string {
	out := strings.ReplaceAll(tpl, "{z}", strconv.Itoa(z))
	out = strings.ReplaceAll(out, "{x}", strconv.Itoa(x))
	out = strings.ReplaceAll(out, "{y}", strconv.Itoa(y))
	return out
}

func dataImageTag(x, y float64, mime string, b []byte) string {
	return fmt.Sprintf(`<image x="%.1f" y="%.1f" width="256" height="256" href="data:%s;base64,%s"/>`, x, y, mime, base64.StdEncoding.EncodeToString(b))
}

func pixelToLatLon(px, py float64, zoom int) (float64, float64) {
	n := math.Pow(2, float64(zoom)) * 256.0
	lon := px/n*360.0 - 180.0
	y := py / n
	lat := math.Atan(math.Sinh(math.Pi*(1.0-2.0*y))) * 180.0 / math.Pi
	return lat, lon
}

func (s *Service) renderArcGISExportSVG(lat, lon float64, zoom int, width, height int) ([]byte, string, error) {
	zoom, left, top, _, _, _, _, _ := tileBounds(lat, lon, zoom, width, height)
	north, west := pixelToLatLon(left, top, zoom)
	south, east := pixelToLatLon(left+float64(width), top+float64(height), zoom)
	if west > east {
		west, east = east, west
	}
	if south > north {
		south, north = north, south
	}
	bbox := fmt.Sprintf("%.6f,%.6f,%.6f,%.6f", west, south, east, north)
	base := "bbox=" + url.QueryEscape(bbox) + "&bboxSR=4326&imageSR=4326&size=" + url.QueryEscape(fmt.Sprintf("%d,%d", width, height)) + "&format=png32&transparent=false&f=image"
	imgURL := "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/export?" + base
	img, mime, err := fetchMapURL(imgURL, 5*time.Second, 2*1024*1024)
	if err != nil {
		return nil, "", err
	}
	pieces := []string{fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height), `<rect width="100%" height="100%" fill="#1c2428"/>`, fmt.Sprintf(`<image x="0" y="0" width="%d" height="%d" href="data:%s;base64,%s"/>`, width, height, mime, base64.StdEncoding.EncodeToString(img))}
	labelURL := "https://server.arcgisonline.com/ArcGIS/rest/services/Reference/World_Boundaries_and_Places/MapServer/export?" + strings.Replace(base, "transparent=false", "transparent=true", 1)
	if labels, labelMime, e := fetchMapURL(labelURL, 3*time.Second, 1024*1024); e == nil {
		pieces = append(pieces, fmt.Sprintf(`<image x="0" y="0" width="%d" height="%d" href="data:%s;base64,%s"/>`, width, height, labelMime, base64.StdEncoding.EncodeToString(labels)))
	}
	pieces = append(pieces, markerSVG(width, height), `</svg>`)
	return []byte(strings.Join(pieces, "\n")), "image/svg+xml", nil
}

func (s *Service) renderTileSVG(p mapProviderGo, lat, lon float64, zoom int, width, height int) ([]byte, string, error) {
	zoom, left, top, n, x0, x1, y0, y1 := tileBounds(lat, lon, zoom, width, height)
	if len(p.Tiles) == 0 {
		return nil, "", fmt.Errorf("no tile URLs")
	}
	pieces := []string{fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height), `<rect width="100%" height="100%" fill="#d9d4c8"/>`}
	count := 0
	for ty := y0; ty <= y1; ty++ {
		if ty < 0 || ty >= n {
			continue
		}
		for tx := x0; tx <= x1; tx++ {
			ux := tx % n
			if ux < 0 {
				ux += n
			}
			var b []byte
			var mime string
			var err error
			for offset := 0; offset < len(p.Tiles); offset++ {
				tpl := p.Tiles[(ux+ty+offset)%len(p.Tiles)]
				b, mime, err = s.fetchTile(p, tileURLTemplate(tpl, zoom, ux, ty), zoom, ux, ty, "")
				if err == nil {
					break
				}
			}
			if err != nil || b == nil {
				return nil, "", fmt.Errorf("tile fetch failed: %v", err)
			}
			pieces = append(pieces, dataImageTag(float64(tx)*256.0-left, float64(ty)*256.0-top, mime, b))
			count++
		}
	}
	if count < 1 {
		return nil, "", fmt.Errorf("no tiles fetched")
	}
	pieces = append(pieces, markerSVG(width, height), `</svg>`)
	return []byte(strings.Join(pieces, "\n")), "image/svg+xml", nil
}

func (s *Service) renderLayeredTileSVG(p mapProviderGo, lat, lon float64, zoom int, width, height int) ([]byte, string, error) {
	zoom, left, top, n, x0, x1, y0, y1 := tileBounds(lat, lon, zoom, width, height)
	if len(p.Layers) == 0 {
		return nil, "", fmt.Errorf("no tile layers")
	}
	pieces := []string{fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height), `<rect width="100%" height="100%" fill="#1c2428"/>`}
	imagery := 0
	lastErr := ""
	for _, layer := range p.Layers {
		if len(layer.Tiles) == 0 {
			continue
		}
		layerCount := 0
		for ty := y0; ty <= y1; ty++ {
			if ty < 0 || ty >= n {
				continue
			}
			for tx := x0; tx <= x1; tx++ {
				ux := tx % n
				if ux < 0 {
					ux += n
				}
				var b []byte
				var mime string
				var err error
				for offset := 0; offset < len(layer.Tiles); offset++ {
					tpl := layer.Tiles[(ux+ty+offset)%len(layer.Tiles)]
					b, mime, err = s.fetchTile(p, tileURLTemplate(tpl, zoom, ux, ty), zoom, ux, ty, layer.Name)
					if err == nil {
						break
					}
				}
				if err != nil || b == nil {
					lastErr = fmt.Sprintf("%s z%d/%d/%d: %v", layer.Name, zoom, ux, ty, err)
					continue
				}
				pieces = append(pieces, dataImageTag(float64(tx)*256.0-left, float64(ty)*256.0-top, mime, b))
				layerCount++
			}
		}
		if layer.Name == "imagery" {
			imagery += layerCount
		}
	}
	if imagery < 1 {
		if lastErr == "" {
			lastErr = "no imagery tiles fetched"
		}
		return nil, "", fmt.Errorf("no imagery tiles fetched; %s", lastErr)
	}
	pieces = append(pieces, markerSVG(width, height), `</svg>`)
	return []byte(strings.Join(pieces, "\n")), "image/svg+xml", nil
}

func (s *Service) renderStaticProvider(p mapProviderGo, lat, lon float64, zoom int) ([]byte, string, error) {
	u := fmt.Sprintf("https://staticmap.openstreetmap.de/staticmap.php?center=%.6f,%.6f&zoom=%d&size=520x220&maptype=mapnik&markers=%.6f,%.6f,red-pushpin", lat, lon, zoom, lat, lon)
	b, ctype, err := fetchMapURL(u, 10*time.Second, 1024*1024)
	if err != nil {
		return nil, "", err
	}
	if strings.Contains(strings.ToLower(ctype), "image") {
		return b, "image/png", nil
	}
	return b, ctype, nil
}

func (s *Service) renderMapWithProvider(name string, lat, lon float64, zoom int, style string) ([]byte, string, error) {
	p, ok := mapProviderByNameGo(name)
	if !ok {
		return nil, "", fmt.Errorf("unknown provider")
	}
	if !mapProviderSupportsStyle(p, style) {
		return nil, "", fmt.Errorf("provider does not support %s", normMapStyle(style))
	}
	switch p.Kind {
	case "tiles":
		return s.renderTileSVG(p, lat, lon, zoom, 520, 220)
	case "layered_tiles":
		return s.renderLayeredTileSVG(p, lat, lon, zoom, 520, 220)
	case "arcgis_export":
		return s.renderArcGISExportSVG(lat, lon, zoom, 520, 220)
	case "static":
		return s.renderStaticProvider(p, lat, lon, zoom)
	default:
		return nil, "", fmt.Errorf("bad provider kind")
	}
}

func mapFallbackSVG(lat, lon float64, z int, style string, reason string) string {
	bg, road, water := "#dfe8d8", "#ffffff", "#b9d9ef"
	if normMapStyle(style) == "hybrid" {
		bg, road, water = "#263238", "#5f6f76", "#1d4d63"
	}
	label := fmt.Sprintf("%.5f, %.5f · z%d", lat, lon, z)
	if reason != "" {
		// Truncate before escaping so an entity like &amp; is never split, and
		// on rune boundaries so the SVG stays valid UTF-8.
		if runes := []rune(reason); len(runes) > 80 {
			reason = string(runes[:80])
		}
		reason = strings.ReplaceAll(reason, "&", "&amp;")
		reason = strings.ReplaceAll(reason, "<", "&lt;")
		reason = strings.ReplaceAll(reason, ">", "&gt;")
		label += " · " + reason
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="520" height="220" viewBox="0 0 520 220"><rect width="520" height="220" fill="%s"/><path d="M-20 60 C70 35 115 90 190 60 S335 25 540 60" fill="none" stroke="%s" stroke-width="26" opacity=".70"/><path d="M45 235 C95 160 160 145 210 118 S345 65 485 -20" fill="none" stroke="%s" stroke-width="14" opacity=".82"/><path d="M-25 165 C75 150 120 175 205 158 S390 126 545 168" fill="none" stroke="%s" stroke-width="9" opacity=".65"/>%s<rect x="14" y="174" width="492" height="36" rx="12" fill="#000" opacity=".42"/><text x="260" y="198" text-anchor="middle" font-family="system-ui,Segoe UI,sans-serif" font-size="15" font-weight="800" fill="#ffffff" stroke="#000000" stroke-width="2.4" stroke-opacity=".34" paint-order="stroke">Map unavailable · %s</text></svg>`, bg, water, road, road, markerSVG(520, 220), label)
}

func (s *Service) fetchMapImage(lat, lon float64, zoom int, style string, force bool) (string, string) {
	style = normMapStyle(style)
	if zoom < 1 || zoom > 18 {
		zoom = mapDefaultZoom
	}
	if !force {
		if p, mime := s.cachedMapImagePath(lat, lon, zoom, style); p != "" {
			return p, mime
		}
	}
	base := mapImageBase(lat, lon, zoom, style)
	_ = os.MkdirAll(s.mapImageDir(), 0755)
	if strings.HasSuffix(os.Args[0], ".test") && os.Getenv("DASHBOARD_MAP_NETWORK_TEST") != "1" {
		path := filepath.Join(s.mapImageDir(), base+".svg")
		_ = os.WriteFile(path, []byte(mapFallbackSVG(lat, lon, zoom, style, "test-mode map network disabled")), 0644)
		return path, "image/svg+xml"
	}
	state := jsonutil.Map(s.readJSONDefault(s.mapProviderFile(), map[string]any{}))
	failures := []map[string]any{}
	for _, name := range s.mapProviderOrder(style) {
		b, mime, err := s.renderMapWithProvider(name, lat, lon, zoom, style)
		if err != nil {
			failures = append(failures, map[string]any{"provider": name, "style": style, "ts": time.Now().Unix(), "error": err.Error()})
			continue
		}
		ext, _ := imageExtFromMime(mime)
		if strings.Contains(strings.ToLower(mime), "svg") {
			ext = ".svg"
		}
		path := filepath.Join(s.mapImageDir(), base+ext)
		tmp := path + ".tmp"
		if os.WriteFile(tmp, b, 0644) != nil || os.Rename(tmp, path) != nil {
			_ = os.Remove(tmp)
			failures = append(failures, map[string]any{"provider": name, "style": style, "ts": time.Now().Unix(), "error": "cache write failed"})
			continue
		}
		primaryByStyle := jsonutil.Map(state["primaryByStyle"])
		primaryByStyle[style] = name
		label := name
		if p, ok := mapProviderByNameGo(name); ok {
			label = p.Label
		}
		state["primary"] = name
		state["primaryLabel"] = label
		state["primaryByStyle"] = primaryByStyle
		state["lastOk"] = time.Now().Unix()
		state["lastError"] = ""
		fs := []any{}
		for _, f := range failures {
			fs = append(fs, f)
		}
		state["failures"] = fs
		_ = fileio.WriteJSON(s.mapProviderFile(), state)
		_ = s.cleanMapImageCache()
		_ = s.cleanMapTileCache()
		return path, mime
	}
	lastErr := "tile provider fetch failed"
	if len(failures) > 0 {
		lastErr = fmt.Sprint(failures[len(failures)-1]["error"])
	}
	fs := []any{}
	parts := []string{}
	for _, f := range failures {
		fs = append(fs, f)
		parts = append(parts, fmt.Sprintf("%s/%s: %s", f["provider"], f["style"], f["error"]))
	}
	state["failures"] = fs
	state["lastFail"] = time.Now().Unix()
	state["lastError"] = strings.Join(parts, "; ")
	_ = fileio.WriteJSON(s.mapProviderFile(), state)
	path := filepath.Join(s.mapImageDir(), base+".svg")
	_ = os.WriteFile(path, []byte(mapFallbackSVG(lat, lon, zoom, style, lastErr)), 0644)
	return path, "image/svg+xml"
}

func (s *Service) handleMapImage(w http.ResponseWriter, r *http.Request) {
	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, _ := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	z := normMapZoom(r.URL.Query().Get("z"))
	st := normMapStyle(r.URL.Query().Get("style"))
	force := r.URL.Query().Get("force") == "1" || strings.EqualFold(r.URL.Query().Get("refresh"), "true")
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 || (lat == 0 && lon == 0) {
		s.err(w, "bad coordinates", 400)
		return
	}
	p, mime := s.fetchMapImage(lat, lon, z, st, force)
	if p == "" {
		s.err(w, "map render failed", 502)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeFile(w, r, p)
}
