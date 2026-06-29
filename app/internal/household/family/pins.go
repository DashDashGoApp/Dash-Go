package family

import (
	"encoding/json"
	"fmt"
	"os"

	controlauth "github.com/DashDashGoApp/Dash-Go/app/internal/auth"
	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const InboxPinsSchema = 1

func PinsDefault() map[string]any {
	return map[string]any{"schema": InboxPinsSchema, "pins": map[string]any{}}
}

func PinRecord(raw any) map[string]any {
	row := jsonutil.Map(raw)
	hash := jsonutil.StringValue(row["hash"])
	salt := jsonutil.StringValue(row["salt"])
	iterations := jsonutil.Int(row["iterations"], 0)
	if hash == "" || salt == "" || iterations < 100000 || iterations > 1000000 {
		return nil
	}
	return map[string]any{"hash": hash, "salt": salt, "iterations": iterations}
}

func NormalizePins(raw map[string]any) map[string]any {
	out := PinsDefault()
	pins := map[string]any{}
	for rawID, rawRecord := range jsonutil.Map(raw["pins"]) {
		id := PersonID(rawID)
		record := PinRecord(rawRecord)
		if id == "" || record == nil {
			continue
		}
		pins[id] = record
	}
	out["pins"] = pins
	return out
}

func (s *Service) Pins() map[string]any {
	s.pinsMu.Lock()
	defer s.pinsMu.Unlock()
	return s.pinsLocked()
}

func (s *Service) pinsLocked() map[string]any {
	return NormalizePins(jsonutil.Map(readJSONDefault(s.pinsPath, PinsDefault())))
}

func (s *Service) WritePins(payload map[string]any) error {
	s.pinsMu.Lock()
	defer s.pinsMu.Unlock()
	return s.writePinsLocked(payload)
}

func (s *Service) writePinsLocked(payload map[string]any) error {
	payload = NormalizePins(payload)
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := fileio.WriteAtomic(s.pinsPath, data, 0600); err != nil {
		return err
	}
	return os.Chmod(s.pinsPath, 0600)
}

func (s *Service) PinConfigured(personID string) bool {
	personID = PersonID(personID)
	if personID == "" {
		return false
	}
	_, ok := jsonutil.Map(s.Pins()["pins"])[personID]
	return ok
}

func (s *Service) SetPIN(personID, pin string) error {
	personID = PersonID(personID)
	if personID == "" {
		return fmt.Errorf("person was not found")
	}
	payload, err := controlauth.NewPINPayload(pin, nil)
	if err != nil {
		return err
	}
	s.pinsMu.Lock()
	defer s.pinsMu.Unlock()
	pins := s.pinsLocked()
	rows := jsonutil.Map(pins["pins"])
	rows[personID] = map[string]any{
		"hash": payload["DASH_CONTROL_PIN_HASH"], "salt": payload["DASH_CONTROL_PIN_SALT"],
		"iterations": jsonutil.Int(payload["DASH_CONTROL_PIN_ITERATIONS"], 200000),
	}
	pins["pins"] = rows
	return s.writePinsLocked(pins)
}

func (s *Service) RemovePIN(personID string) error {
	personID = PersonID(personID)
	if personID == "" {
		return fmt.Errorf("person was not found")
	}
	s.pinsMu.Lock()
	pins := s.pinsLocked()
	rows := jsonutil.Map(pins["pins"])
	delete(rows, personID)
	pins["pins"] = rows
	err := s.writePinsLocked(pins)
	s.pinsMu.Unlock()
	if err != nil {
		return err
	}
	s.RevokeSessions(personID)
	return nil
}

func (s *Service) VerifyPIN(personID, pin string) bool {
	personID = PersonID(personID)
	record := jsonutil.Map(jsonutil.Map(s.Pins()["pins"])[personID])
	if len(record) == 0 {
		return true
	}
	return controlauth.VerifyPIN(pin, jsonutil.StringValue(record["salt"]), jsonutil.StringValue(record["hash"]), jsonutil.Int(record["iterations"], 0))
}
