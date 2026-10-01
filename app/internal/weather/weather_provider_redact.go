package weather

import (
	"errors"
	"regexp"
)

// weatherSecretQueryPattern matches the credential query parameters Dash-Go
// sends to keyed providers. Provider failures quote the whole request URL — Go's
// http client renders a timeout as
// `Get "https://host/path?key=SECRET": context deadline exceeded` — so without
// redaction the credential is written to the kiosk's rate-state file on disk and
// rendered on the Control surface. Longer parameter names are listed first so
// that `subscription-key` is never matched as a bare `key`.
var weatherSecretQueryPattern = regexp.MustCompile(`(?i)\b(subscription-key|client_secret|api_key|apikey|app_id|appid|token|key)=[^&\s"]*`)

// redactWeatherErrorGo masks the value of every credential query parameter while
// leaving the rest of the message intact, because the host, path, and failure
// reason are what make the message useful for diagnosis.
func redactWeatherErrorGo(msg string) string {
	if msg == "" {
		return msg
	}
	return weatherSecretQueryPattern.ReplaceAllString(msg, "$1=[redacted]")
}

// redactWeatherError applies that redaction to a provider error. It is called
// once, at the point a live fetch fails, so every downstream consumer is covered
// without each one having to remember: the persisted rate state, the stale
// marker, and the status payload the household can see in Control.
func redactWeatherError(err error) error {
	if err == nil {
		return nil
	}
	original := err.Error()
	redacted := redactWeatherErrorGo(original)
	if redacted == original {
		return err
	}
	return errors.New(redacted)
}
