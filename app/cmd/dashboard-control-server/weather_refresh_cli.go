package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

// runRefreshWeatherCLI is used by the health guard after a verified network
// recovery. It only replaces the blended cache after at least one provider
// returned valid data, so a transient offline state cannot poison last-good
// weather with an error payload.
func (a *app) runRefreshWeatherCLI(args []string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	payload, err := a.fetchGoWeather(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "weather refresh skipped:", err)
		return 1
	}
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "weather-cache.json"), payload); err != nil {
		fmt.Fprintln(os.Stderr, "weather cache write failed:", err)
		return 1
	}
	fmt.Println("weather refresh complete")
	return 0
}
