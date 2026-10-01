package weather

import (
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

// A failing air-quality endpoint used to be re-attempted on every weather refresh
// (and once per browser retry), because only successful responses were cached. The
// failure marker is what stops that, so its behaviour is pinned here.
func TestWeatherAQIFailureCooldownSuppressesRepeatAttempts(t *testing.T) {
	dir := t.TempDir()
	s := &Service{cacheDir: dir}

	if until := s.weatherAQIFailureUntil("key-a"); !until.IsZero() {
		t.Fatalf("a location that never failed must not be cooling down, got %v", until)
	}

	s.weatherAQINoteFailure("key-a")
	until := s.weatherAQIFailureUntil("key-a")
	if until.IsZero() {
		t.Fatal("a recorded failure must open a cooldown")
	}
	if remaining := time.Until(until); remaining <= 0 || remaining > weatherAQIFailureCooldown {
		t.Fatalf("cooldown remaining=%v, want within (0, %v]", remaining, weatherAQIFailureCooldown)
	}

	if other := s.weatherAQIFailureUntil("key-b"); !other.IsZero() {
		t.Fatal("a different cache key (moved location) must not inherit the cooldown")
	}

	s.weatherAQINoteSuccess("key-a")
	if until := s.weatherAQIFailureUntil("key-a"); !until.IsZero() {
		t.Fatal("a success must clear the cooldown so a recovered endpoint caches normally")
	}

	expired := weatherAQIFailureState{CacheKey: "key-a", FailedAt: time.Now().Add(-2 * weatherAQIFailureCooldown).Unix()}
	if err := fileio.WriteJSON(weatherAQIFailurePath(dir), expired); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if until := s.weatherAQIFailureUntil("key-a"); !until.IsZero() {
		t.Fatal("an expired cooldown must let the next refresh probe again")
	}

	untimed := weatherAQIFailureState{CacheKey: "key-a"}
	if err := fileio.WriteJSON(weatherAQIFailurePath(dir), untimed); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if until := s.weatherAQIFailureUntil("key-a"); !until.IsZero() {
		t.Fatal("a marker without a failure time must not block air quality forever")
	}
}

func TestWeatherAQIFailureCooldownStaysShortEnoughToRecover(t *testing.T) {
	// The cache TTL is the household-visible freshness promise; the cooldown exists
	// to stop doomed retries, not to hide a recovered provider for longer than the
	// cached value would have been considered fresh.
	if weatherAQIFailureCooldown >= weatherAQICacheTTL {
		t.Fatalf("failure cooldown %v must be shorter than the cache TTL %v", weatherAQIFailureCooldown, weatherAQICacheTTL)
	}
}
