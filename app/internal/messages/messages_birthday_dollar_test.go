package messages

import (
	"os"
	"strings"
	"testing"
)

func TestBirthdayNamesWithDollarSignSurvivePersistence(t *testing.T) {
	s := testService(t)
	want := "Toy $100 Fund"
	first := []any{map[string]any{"id": 1, "name": want, "date": "03-15"}}
	if saved := s.saveBirthdays(first); len(saved) != 1 {
		t.Fatalf("first save=%v", saved)
	}
	// Second save takes the regex path that previously expanded "$1" into a
	// capture group and corrupted the persisted name.
	saved := s.saveBirthdays(first)
	if len(saved) != 1 {
		t.Fatalf("second save=%v", saved)
	}
	if saved[0].(map[string]any)["name"] != want {
		t.Fatalf("returned name=%v want %q", saved[0].(map[string]any)["name"], want)
	}
	b, err := os.ReadFile(s.configLocal)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), want) {
		t.Fatalf("persisted file lost the name: %s", string(b))
	}
	reloaded := s.loadBirthdays()
	if len(reloaded) != 1 || reloaded[0].(map[string]any)["name"] != want {
		t.Fatalf("reload=%v want name %q", reloaded, want)
	}
}
