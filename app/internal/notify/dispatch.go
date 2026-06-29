package notify

import (
	"strings"
	"time"

	apprise "github.com/unraid/apprise-go"
)

// DefaultSend is the production Apprise-Go transport. It must not log raw
// routes, message content, or provider response details.
func DefaultSend(routes []string, title, body string, warning bool) error {
	client := apprise.New()
	for _, route := range routes {
		if err := client.Add(route); err != nil {
			return err
		}
	}
	kind := apprise.NotifyInfo
	if warning {
		kind = apprise.NotifyWarning
	}
	return client.Send(body, apprise.WithTitle(title), apprise.WithNotifyType(kind))
}

// Start begins the one bounded queue consumer. It is idempotent so normal
// startup and focused tests cannot create duplicate delivery loops. A stopped
// process-lifetime service cannot be restarted; the runtime constructs one
// notifier for its entire lifetime.
func (s *Service) Start() {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	if s.started || s.stopping {
		s.lifecycleMu.Unlock()
		return
	}
	s.started = true
	s.workerDone = make(chan struct{})
	queue := s.queue
	stopCh := s.stopCh
	done := s.workerDone
	s.lifecycleMu.Unlock()
	go s.run(queue, stopCh, done)
}

func (s *Service) run(queue <-chan Event, stopCh <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case event := <-queue:
			s.Deliver(event)
		case <-stopCh:
			// Stop first rejects new work under lifecycleMu, then this worker
			// drains every event that was accepted before that boundary. Deliver
			// includes the persisted delivery-state update before it returns.
			for {
				select {
				case event := <-queue:
					s.Deliver(event)
				default:
					return
				}
			}
		}
	}
}

// Stop is safe to call repeatedly. It prevents all future queue writes,
// drains work already accepted by Enqueue, and waits for the single delivery
// worker to finish its final persisted state update. It deliberately does not
// close queue: a concurrent request must be rejected safely, never panic.
func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	if !s.stopping {
		s.stopping = true
		close(s.stopCh)
	}
	done := s.workerDone
	s.lifecycleMu.Unlock()
	if done != nil {
		<-done
	}
}

func trimTimes(rows []time.Time, now time.Time) []time.Time {
	cutoff := now.Add(-time.Minute)
	kept := rows[:0]
	for _, at := range rows {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	return kept
}

func (s *Service) deliveryAllowed(personID string) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recent = trimTimes(s.recent, now)
	s.recipientRecent[personID] = trimTimes(s.recipientRecent[personID], now)
	if s.inFlight || len(s.recent) >= allAttemptsMinute || len(s.recipientRecent[personID]) >= recipientAttemptsMinute {
		return false
	}
	s.inFlight = true
	s.recent = append(s.recent, now)
	s.recipientRecent[personID] = append(s.recipientRecent[personID], now)
	return true
}

func (s *Service) deliveryFinished() {
	s.mu.Lock()
	s.inFlight = false
	s.mu.Unlock()
}

func (s *Service) Enqueue(event Event) {
	if s == nil {
		return
	}
	event.PersonID = s.normalizePersonID(event.PersonID)
	if event.PersonID == "" || strings.TrimSpace(event.Title) == "" || strings.TrimSpace(event.Body) == "" {
		return
	}
	s.lifecycleMu.Lock()
	if s.stopping {
		s.lifecycleMu.Unlock()
		return
	}
	dropped := false
	select {
	case s.queue <- event:
	default:
		dropped = true
	}
	s.lifecycleMu.Unlock()
	if dropped {
		s.RecordDeliveryState(event.PersonID, "rate-limited")
	}
}

// Deliver performs one synchronous delivery attempt. The queue calls it in the
// background, while the installer CLI deliberately uses it directly for a
// deterministic test route result.
func (s *Service) Deliver(event Event) {
	if s == nil {
		return
	}
	if event.MessageID != "" && !s.messageStillCurrent(event) {
		return
	}
	if !s.deliveryAllowed(event.PersonID) {
		s.RecordDeliveryState(event.PersonID, "rate-limited")
		return
	}
	routes := s.RoutesForPerson(event.PersonID)
	if len(routes) == 0 {
		s.deliveryFinished()
		return
	}
	result := make(chan error, 1)
	go func() { result <- s.sendFn(routes, event.Title, event.Body, event.Warning) }()
	select {
	case err := <-result:
		s.deliveryFinished()
		if err != nil {
			s.RecordDeliveryState(event.PersonID, "failed")
			return
		}
		s.RecordDeliveryState(event.PersonID, "delivered")
	case <-time.After(deliveryDeadline):
		// Keep the single-flight gate closed until the underlying provider returns.
		// This prevents retries or concurrent duplicate deliveries after a timeout.
		s.RecordDeliveryState(event.PersonID, "timeout")
		go func() { <-result; s.deliveryFinished() }()
	}
}

func PreviewText(sender, text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len([]rune(text)) > 180 {
		text = string([]rune(text)[:180]) + "…"
	}
	if sender == "" {
		return text
	}
	return "From " + sender + ": " + text
}
