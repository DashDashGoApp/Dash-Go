package weather

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const weatherLocationCacheTTL = 30 * 24 * time.Hour

type weatherLocationCache struct {
	Schema   int    `json:"schema"`
	Provider string `json:"provider"`
	Location string `json:"location"`
	Value    string `json:"value"`
	SavedAt  int64  `json:"savedAt"`
}

func weatherLocationIdentity(cfg Config) string {
	return fmt.Sprintf("%.5f,%.5f", cfg.Lat, cfg.Lon)
}

func weatherLocationCachePath(cfg Config, provider string) string {
	return filepath.Join(cfg.CacheDir, "weather-location-"+strings.ToLower(provider)+".json")
}

func weatherLocationCacheRead(cfg Config, provider string) (string, bool) {
	path := weatherLocationCachePath(cfg, provider)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || time.Since(info.ModTime()) > weatherLocationCacheTTL {
		return "", false
	}
	var entry weatherLocationCache
	body := readJSONDefaultMap(path)
	entry.Schema = jsonutil.Int(body["schema"], 0)
	entry.Provider = jsonutil.StringValue(body["provider"])
	entry.Location = jsonutil.StringValue(body["location"])
	entry.Value = jsonutil.StringValue(body["value"])
	if entry.Schema != 1 || entry.Provider != provider || entry.Location != weatherLocationIdentity(cfg) || entry.Value == "" {
		return "", false
	}
	return entry.Value, true
}

func weatherLocationCacheWrite(cfg Config, provider, value string) {
	value = strings.TrimSpace(value)
	if value == "" || cfg.CacheDir == "" {
		return
	}
	_ = os.MkdirAll(cfg.CacheDir, 0755)
	_ = fileio.WriteJSON(weatherLocationCachePath(cfg, provider), weatherLocationCache{
		Schema: 1, Provider: provider, Location: weatherLocationIdentity(cfg), Value: value, SavedAt: time.Now().UnixMilli(),
	})
}
