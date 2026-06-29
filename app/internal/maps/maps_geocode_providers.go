package maps

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const mapGeocoderUserAgent = "Dash-Go/1.5.0-beta.39 local-kiosk map preview"

func (s *Service) nominatimSearch(q string) ([]map[string]any, error) {
	u := "https://nominatim.openstreetmap.org/search?format=jsonv2&limit=3&q=" + url.QueryEscape(q)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", mapGeocoderUserAgent)
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var raw []map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Service) censusSearch(q string) ([]map[string]any, error) {
	q = reUSCountrySuffix.ReplaceAllString(cleanLocationPiece(q), "")
	if !(looksLikeUSAddress(q) || looksLikeStreetAddress(q)) {
		return nil, nil
	}
	u := "https://geocoding.geo.census.gov/geocoder/locations/onelineaddress?benchmark=Public_AR_Current&format=json&address=" + url.QueryEscape(q)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", mapGeocoderUserAgent)
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&raw); err != nil {
		return nil, err
	}
	out := []map[string]any{}
	result := jsonutil.Map(raw["result"])
	for _, it := range jsonutil.List(result["addressMatches"]) {
		m := jsonutil.Map(it)
		coords := jsonutil.Map(m["coordinates"])
		lat, lon := anyFloat(coords["y"]), anyFloat(coords["x"])
		if lat == 0 && lon == 0 {
			continue
		}
		out = append(out, map[string]any{"lat": lat, "lon": lon, "display_name": defaultString(strOr(m["matchedAddress"], ""), q), "source": "us-census"})
	}
	return out, nil
}

func (s *Service) geocodeEventLocationVariants(variants []string) ([]map[string]any, string, string, []string) {
	errors := []string{}
	for _, cand := range variants {
		if !(looksLikeUSAddress(cand) || looksLikeStreetAddress(cand)) {
			continue
		}
		rows, err := s.censusSearch(cand)
		if err != nil {
			errors = append(errors, fmt.Sprintf("us-census %q: %s", cand, err.Error()))
			continue
		}
		if len(rows) > 0 {
			return rows, cand, "us-census", errors
		}
	}
	for _, cand := range variants {
		rows, err := s.nominatimSearch(cand)
		if err != nil {
			errors = append(errors, fmt.Sprintf("nominatim %q: %s", cand, err.Error()))
			continue
		}
		if len(rows) > 0 {
			return rows, cand, "nominatim", errors
		}
	}
	return nil, "", "", errors
}

func (s *Service) geocode(q string) map[string]any {
	q = strings.TrimSpace(strings.Split(q, ",")[0])
	if len(q) > 60 {
		q = q[:60]
	}
	if q == "" {
		return map[string]any{"results": []any{}, "error": "missing q"}
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Get("https://geocoding-api.open-meteo.com/v1/search?name=" + url.QueryEscape(q) + "&count=5&language=en&format=json")
	if err != nil {
		return map[string]any{"results": []any{}, "error": "lookup failed: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return map[string]any{"results": []any{}, "error": fmt.Sprintf("lookup failed: HTTP %d", resp.StatusCode)}
	}
	var raw map[string]any
	if json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&raw) != nil {
		return map[string]any{"results": []any{}, "error": "lookup failed: invalid response"}
	}
	out := []any{}
	for _, it := range jsonutil.List(raw["results"]) {
		m := jsonutil.Map(it)
		parts := []string{}
		for _, k := range []string{"name", "admin1", "country_code"} {
			if s := strOr(m[k], ""); s != "" {
				parts = append(parts, s)
			}
		}
		out = append(out, map[string]any{"label": strings.Join(parts, ", "), "city": strOr(m["name"], ""), "lat": anyFloat(m["latitude"]), "lon": anyFloat(m["longitude"])})
	}
	return map[string]any{"results": out}
}
