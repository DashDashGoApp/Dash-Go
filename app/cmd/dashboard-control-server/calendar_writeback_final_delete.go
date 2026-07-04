package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

const (
	calendarWritebackFinalDeleteFile                 = "calendar-writeback-final-delete.json"
	calendarWritebackFinalDeleteSchema               = 1
	calendarWritebackFinalDeleteMaxAutomaticAttempts = 2
)

// Kept as a variable so focused tests can exercise recovery without waiting.
var calendarWritebackFinalDeleteRetryDelay = 30 * time.Second

type calendarWritebackFinalDeleteStore struct {
	Schema int                                           `json:"schema"`
	Pairs  map[string]calendarWritebackFinalDeletePermit `json:"pairs"`
}

type calendarWritebackFinalDeletePermit struct {
	Source     string `json:"source"`
	Pair       string `json:"pair"`
	Collection string `json:"collection"`
	Attempts   int    `json:"attempts"`
}

func (a *app) calendarWritebackFinalDeletePath() string {
	return filepath.Join(a.configDir, calendarWritebackFinalDeleteFile)
}

func validCalendarWritebackPair(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 96 {
		return false
	}
	for _, r := range value {
		if !(r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func newCalendarWritebackFinalDeleteStore() calendarWritebackFinalDeleteStore {
	return calendarWritebackFinalDeleteStore{
		Schema: calendarWritebackFinalDeleteSchema,
		Pairs:  map[string]calendarWritebackFinalDeletePermit{},
	}
}

func validateCalendarWritebackFinalDeleteStore(store calendarWritebackFinalDeleteStore) error {
	if store.Schema != calendarWritebackFinalDeleteSchema {
		return errors.New("unsupported final-delete sync authorization")
	}
	if store.Pairs == nil {
		return errors.New("final-delete sync authorization pairs are missing")
	}
	for key, permit := range store.Pairs {
		if !validCalendarWritebackPair(key) || permit.Pair != key {
			return errors.New("invalid final-delete sync pair")
		}
		if strings.TrimSpace(permit.Source) == "" || strings.TrimSpace(permit.Collection) == "" || !filepath.IsAbs(permit.Collection) {
			return errors.New("invalid final-delete sync authorization")
		}
		if permit.Attempts < 0 {
			return errors.New("invalid final-delete sync attempt count")
		}
	}
	return nil
}

func (a *app) loadCalendarWritebackFinalDeleteStoreLocked() (calendarWritebackFinalDeleteStore, error) {
	path := a.calendarWritebackFinalDeletePath()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return newCalendarWritebackFinalDeleteStore(), nil
	}
	if err != nil {
		return calendarWritebackFinalDeleteStore{}, fmt.Errorf("read final-delete sync authorization: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var store calendarWritebackFinalDeleteStore
	if err := decoder.Decode(&store); err != nil {
		return calendarWritebackFinalDeleteStore{}, fmt.Errorf("invalid final-delete sync authorization: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return calendarWritebackFinalDeleteStore{}, errors.New("invalid final-delete sync authorization trailing data")
		}
		return calendarWritebackFinalDeleteStore{}, fmt.Errorf("invalid final-delete sync authorization trailing data: %w", err)
	}
	if err := validateCalendarWritebackFinalDeleteStore(store); err != nil {
		return calendarWritebackFinalDeleteStore{}, err
	}
	return store, nil
}

func (a *app) saveCalendarWritebackFinalDeleteStoreLocked(store calendarWritebackFinalDeleteStore) error {
	if err := validateCalendarWritebackFinalDeleteStore(store); err != nil {
		return err
	}
	path := a.calendarWritebackFinalDeletePath()
	if len(store.Pairs) == 0 {
		if err := fileio.RemoveDurable(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear final-delete sync authorization: %w", err)
		}
		return nil
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := fileio.WriteAtomic(path, raw, 0600); err != nil {
		return fmt.Errorf("save final-delete sync authorization: %w", err)
	}
	return nil
}

// recordCalendarWritebackMutation persists the narrow authority created by a
// verified final local deletion. Any later non-final mutation for the same pair
// clears it before the next queued sync can run.
func (a *app) recordCalendarWritebackMutation(source, pair, collection string, finalDelete bool) error {
	pair = strings.TrimSpace(pair)
	if pair == "" || pair == "__legacy__" {
		return nil
	}
	if !validCalendarWritebackPair(pair) {
		return errors.New("invalid private calendar sync pair")
	}
	a.writebackSyncMu.Lock()
	defer a.writebackSyncMu.Unlock()
	store, err := a.loadCalendarWritebackFinalDeleteStoreLocked()
	if err != nil {
		return err
	}
	if !finalDelete {
		if _, ok := store.Pairs[pair]; !ok {
			return nil
		}
		delete(store.Pairs, pair)
		return a.saveCalendarWritebackFinalDeleteStoreLocked(store)
	}
	eligible, err := a.calendarWritebackService().FinalDeleteEligible(source, pair, collection)
	if err != nil {
		return fmt.Errorf("verify final-delete sync authorization: %w", err)
	}
	if !eligible {
		return errors.New("final-delete sync authorization requires an empty local collection")
	}
	store.Pairs[pair] = calendarWritebackFinalDeletePermit{
		Source:     strings.TrimSpace(source),
		Pair:       pair,
		Collection: filepath.Clean(collection),
		Attempts:   0,
	}
	return a.saveCalendarWritebackFinalDeleteStoreLocked(store)
}

// finalDeletePermissionLocked returns whether a queued run may be armed. The
// persisted tuple must still resolve to the exact registered source/pair/
// collection and its collection must still have no regular .ics items.
func (a *app) finalDeletePermissionLocked(source, pair string, automatic bool) (armed, exists bool, err error) {
	pair = strings.TrimSpace(pair)
	if pair == "" || pair == "__legacy__" || !validCalendarWritebackPair(pair) {
		return false, false, nil
	}
	store, err := a.loadCalendarWritebackFinalDeleteStoreLocked()
	if err != nil {
		return false, false, err
	}
	permit, ok := store.Pairs[pair]
	if !ok {
		return false, false, nil
	}
	if permit.Source != strings.TrimSpace(source) {
		delete(store.Pairs, pair)
		if err := a.saveCalendarWritebackFinalDeleteStoreLocked(store); err != nil {
			return false, false, err
		}
		return false, false, nil
	}
	eligible, checkErr := a.calendarWritebackService().FinalDeleteEligible(permit.Source, permit.Pair, permit.Collection)
	if checkErr != nil {
		// A transient registry or storage read error must not silently discard a
		// verified final-delete authorization. The caller fails closed for this
		// run and can recover it once the trusted registry is readable again.
		return false, true, fmt.Errorf("recheck final-delete sync authorization: %w", checkErr)
	}
	if !eligible {
		delete(store.Pairs, pair)
		if err := a.saveCalendarWritebackFinalDeleteStoreLocked(store); err != nil {
			return false, false, err
		}
		return false, false, nil
	}
	if automatic && permit.Attempts >= calendarWritebackFinalDeleteMaxAutomaticAttempts {
		return false, true, nil
	}
	return true, true, nil
}

// beginFinalDeleteAttempt durably records a targeted force-delete attempt
// before the wrapper starts. After a crash, startup recovery can make at most
// the remaining bounded retry attempts, rather than silently falling back to an
// unarmed empty-collection sync.
func (a *app) beginFinalDeleteAttempt(source, pair string, automatic bool) (armed bool, attempts int, err error) {
	a.writebackSyncMu.Lock()
	defer a.writebackSyncMu.Unlock()
	armed, exists, err := a.finalDeletePermissionLocked(source, pair, automatic)
	if err != nil || !armed {
		return armed, 0, err
	}
	store, err := a.loadCalendarWritebackFinalDeleteStoreLocked()
	if err != nil {
		return false, 0, err
	}
	permit := store.Pairs[strings.TrimSpace(pair)]
	if !exists || permit.Source != strings.TrimSpace(source) {
		return false, 0, nil
	}
	permit.Attempts++
	store.Pairs[permit.Pair] = permit
	if err := a.saveCalendarWritebackFinalDeleteStoreLocked(store); err != nil {
		return false, 0, err
	}
	return true, permit.Attempts, nil
}

func (a *app) clearFinalDeletePermission(source, pair string) error {
	pair = strings.TrimSpace(pair)
	if pair == "" || pair == "__legacy__" || !validCalendarWritebackPair(pair) {
		return nil
	}
	a.writebackSyncMu.Lock()
	defer a.writebackSyncMu.Unlock()
	store, err := a.loadCalendarWritebackFinalDeleteStoreLocked()
	if err != nil {
		return err
	}
	permit, ok := store.Pairs[pair]
	if !ok || permit.Source != strings.TrimSpace(source) {
		return nil
	}
	delete(store.Pairs, pair)
	return a.saveCalendarWritebackFinalDeleteStoreLocked(store)
}

func (a *app) finalDeletePermitsForRecovery() ([]calendarWritebackFinalDeletePermit, error) {
	a.writebackSyncMu.Lock()
	defer a.writebackSyncMu.Unlock()
	store, err := a.loadCalendarWritebackFinalDeleteStoreLocked()
	if err != nil {
		return nil, err
	}
	out := make([]calendarWritebackFinalDeletePermit, 0, len(store.Pairs))
	for _, permit := range store.Pairs {
		out = append(out, permit)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pair < out[j].Pair })
	return out, nil
}

// resumeCalendarWritebackFinalDeletes runs only when the long-lived HTTP server
// starts. CLI helpers and cron's ordinary full sync never inherit this authority.
func (a *app) resumeCalendarWritebackFinalDeletes() {
	permits, err := a.finalDeletePermitsForRecovery()
	if err != nil {
		return
	}
	for _, permit := range permits {
		a.queueCalendarWritebackSync(permit.Source, permit.Pair, true)
	}
}

func (a *app) scheduleCalendarWritebackFinalDeleteRetry(source, pair string) {
	pair = strings.TrimSpace(pair)
	if pair == "" || pair == "__legacy__" {
		return
	}
	a.writebackSyncMu.Lock()
	if a.writebackFinalDeleteRetries == nil {
		a.writebackFinalDeleteRetries = map[string]bool{}
	}
	if a.writebackFinalDeleteRetries[pair] {
		a.writebackSyncMu.Unlock()
		return
	}
	a.writebackFinalDeleteRetries[pair] = true
	a.writebackSyncMu.Unlock()

	time.AfterFunc(calendarWritebackFinalDeleteRetryDelay, func() {
		a.writebackSyncMu.Lock()
		delete(a.writebackFinalDeleteRetries, pair)
		a.writebackSyncMu.Unlock()
		a.queueCalendarWritebackSync(source, pair, true)
	})
}
