package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

// runRefreshWeatherCLI is used by the health guard after a verified network
// recovery. It only replaces the blended cache after at least one provider
// returned valid data, so a transient offline state cannot poison last-good
// weather with an error payload.
func (a *app) runRefreshWeatherCLI(args []string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	_, err := a.refreshGoWeatherLive(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "weather refresh skipped:", err)
		return 1
	}
	fmt.Println("weather refresh complete")
	return 0
}
