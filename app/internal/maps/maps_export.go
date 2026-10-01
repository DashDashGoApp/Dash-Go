package maps

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func (s *Service) renderArcGISExportSVG(lat, lon float64, zoom int, width, height int) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mapRenderDeadline)
	defer cancel()
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
	labelURL := "https://server.arcgisonline.com/ArcGIS/rest/services/Reference/World_Boundaries_and_Places/MapServer/export?" + strings.Replace(base, "transparent=false", "transparent=true", 1)
	img, labels := fetchArcGISExports(ctx, imgURL, labelURL)
	if img.err != nil {
		return nil, "", img.err
	}
	pieces := []string{fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height), `<rect width="100%" height="100%" fill="#1c2428"/>`, fmt.Sprintf(`<image x="0" y="0" width="%d" height="%d" href="data:%s;base64,%s"/>`, width, height, img.mime, base64.StdEncoding.EncodeToString(img.b))}
	if labels.err == nil {
		pieces = append(pieces, fmt.Sprintf(`<image x="0" y="0" width="%d" height="%d" href="data:%s;base64,%s"/>`, width, height, labels.mime, base64.StdEncoding.EncodeToString(labels.b)))
	}
	pieces = append(pieces, markerSVG(width, height), `</svg>`)
	return []byte(strings.Join(pieces, "\n")), "image/svg+xml", nil
}

// arcGISExport carries one export fetch outcome. A nil byte slice with a nil
// error never occurs: every path sets exactly one of the two.
type arcGISExport struct {
	b    []byte
	mime string
	err  error
}

// fetchArcGISExports fetches the imagery and label exports concurrently over
// the shared transport. Both requests are independent, so the wall time is the
// slower of the two rather than their sum. Label failure stays the caller's
// optional case; imagery failure is the render failure.
func fetchArcGISExports(ctx context.Context, imgURL, labelURL string) (arcGISExport, arcGISExport) {
	imgCh := make(chan arcGISExport, 1)
	labelCh := make(chan arcGISExport, 1)
	go func() {
		b, mime, err := fetchMapURLContext(ctx, imgURL, 5*time.Second, 2*1024*1024)
		imgCh <- arcGISExport{b: b, mime: mime, err: err}
	}()
	go func() {
		b, mime, err := fetchMapURLContext(ctx, labelURL, 3*time.Second, 1024*1024)
		labelCh <- arcGISExport{b: b, mime: mime, err: err}
	}()
	return <-imgCh, <-labelCh
}
