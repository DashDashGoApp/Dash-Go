package weather

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const radarProxyRequestLimit = 90

func (s *Service) radarAllowRequest(provider string) bool {
	s.radarMu.Lock()
	defer s.radarMu.Unlock()
	if s.radarRequestTimes == nil {
		s.radarRequestTimes = map[string][]time.Time{}
	}
	now := time.Now()
	old := s.radarRequestTimes[provider]
	kept := make([]time.Time, 0, len(old))
	for _, ts := range old {
		if now.Sub(ts) < time.Minute {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= radarProxyRequestLimit {
		s.radarRequestTimes[provider] = kept
		return false
	}
	s.radarRequestTimes[provider] = append(kept, now)
	return true
}

func radarTileCoordinates(r *http.Request) (int, int, int, error) {
	q := r.URL.Query()
	z, err := strconv.Atoi(q.Get("z"))
	if err != nil || z < 0 || z > 12 {
		return 0, 0, 0, fmt.Errorf("invalid zoom")
	}
	x, err := strconv.Atoi(q.Get("x"))
	if err != nil || x < 0 || x >= 1<<z {
		return 0, 0, 0, fmt.Errorf("invalid tile x")
	}
	y, err := strconv.Atoi(q.Get("y"))
	if err != nil || y < 0 || y >= 1<<z {
		return 0, 0, 0, fmt.Errorf("invalid tile y")
	}
	return z, x, y, nil
}

func (s *Service) radarTileURL(provider string, z, x, y int) (string, error) {
	provider = radarNormalizeProviderID(provider)
	switch provider {
	case "tomorrow":
		key := s.radarKey(provider, "DASH_RADAR_TOMORROW_KEY")
		if key == "" {
			return "", fmt.Errorf("radar provider needs an API key")
		}
		return "https://api.tomorrow.io/v4/map/tile/" + strconv.Itoa(z) + "/" + strconv.Itoa(x) + "/" + strconv.Itoa(y) + "/precipitationIntensity/now.png?apikey=" + url.QueryEscape(key), nil
	case "weatherbit":
		key := s.radarKey(provider, "DASH_RADAR_WEATHERBIT_KEY")
		if key == "" {
			return "", fmt.Errorf("radar provider needs an API key")
		}
		return "https://maps.weatherbit.io/v2.0/singleband/catprecipdbz/latest/" + strconv.Itoa(z) + "/" + strconv.Itoa(x) + "/" + strconv.Itoa(y) + ".png?key=" + url.QueryEscape(key), nil
	case "xweather":
		id := s.radarKey(provider, "DASH_RADAR_XWEATHER_ID")
		secret := s.radarKey(provider, "DASH_RADAR_XWEATHER_SECRET")
		if id == "" || secret == "" {
			return "", fmt.Errorf("radar provider needs an API key")
		}
		return "https://maps.api.xweather.com/" + url.PathEscape(id) + "_" + url.PathEscape(secret) + "/radar/" + strconv.Itoa(z) + "/" + strconv.Itoa(x) + "/" + strconv.Itoa(y) + "/current.png", nil
	default:
		return "", fmt.Errorf("provider is not available through the keyed proxy")
	}
}

func (s *Service) handleRadarTile(w http.ResponseWriter, r *http.Request) {
	provider := radarNormalizeProviderID(r.URL.Query().Get("provider"))
	spec, known := radarProviderSpecs[provider]
	if !known || !spec.KeyRequired {
		s.err(w, "unsupported radar provider", http.StatusBadRequest)
		return
	}
	if until, reason, _, active := s.providerBackoffActive("radar-" + provider); active {
		w.Header().Set("Retry-After", strconv.FormatInt(max(int64(1), int64(time.Until(until).Seconds())), 10))
		s.err(w, "radar provider is cooling down: "+reason, http.StatusServiceUnavailable)
		return
	}
	if !s.networkLikelyAvailable() {
		s.err(w, "radar unavailable: no network", http.StatusServiceUnavailable)
		return
	}
	if !s.radarAllowRequest(provider) {
		s.err(w, "radar request limit reached", http.StatusTooManyRequests)
		return
	}
	z, x, y, err := radarTileCoordinates(r)
	if err != nil {
		s.err(w, err.Error(), http.StatusBadRequest)
		return
	}
	target, err := s.radarTileURL(provider, z, x, y)
	if err != nil {
		s.err(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		s.err(w, "could not make radar request", http.StatusBadGateway)
		return
	}
	req.Header.Set("User-Agent", weatherOutboundUserAgent)
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		s.noteProviderBackoff("radar-"+provider, err)
		s.err(w, "radar provider request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		err := fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
		s.noteProviderBackoff("radar-"+provider, err)
		s.err(w, "radar provider is unavailable", http.StatusBadGateway)
		return
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		contentType = "image/png"
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil || len(b) == 0 {
		if err == nil {
			err = fmt.Errorf("empty tile response")
		}
		s.noteProviderBackoff("radar-"+provider, err)
		s.err(w, "radar provider returned no tile", http.StatusBadGateway)
		return
	}
	s.clearProviderBackoff("radar-" + provider)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=90")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(b)
}
