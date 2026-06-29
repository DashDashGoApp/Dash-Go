package auth_test

import (
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/auth"
)

func TestTimeoutNormalizationAndPINPayload(t *testing.T) {
	if got := auth.NormalizeTimeout("every-open"); got != "every_open" {
		t.Fatalf("normalized timeout = %q", got)
	}
	payload, err := auth.NewPINPayload("2468", "60")
	if err != nil {
		t.Fatal(err)
	}
	if !auth.VerifyPIN("2468", payload["DASH_CONTROL_PIN_SALT"], payload["DASH_CONTROL_PIN_HASH"], 200000) {
		t.Fatal("payload did not verify its source PIN")
	}
	if auth.VerifyPIN("0000", payload["DASH_CONTROL_PIN_SALT"], payload["DASH_CONTROL_PIN_HASH"], 200000) {
		t.Fatal("wrong PIN verified")
	}
}

func TestSessionExpiryModesAndTokens(t *testing.T) {
	now := time.Unix(100, 0)
	if got := auth.SessionExpiry("until_reboot", now); got != nil {
		t.Fatal("until_reboot should not expire")
	}
	if got := auth.SessionExpiry("60", now); got == nil || !got.Equal(now.Add(time.Minute)) {
		t.Fatalf("60 second expiry = %v", got)
	}
	if first, second := auth.NewToken(), auth.NewToken(); first == "" || first == second {
		t.Fatalf("tokens not distinct: %q %q", first, second)
	}
}
