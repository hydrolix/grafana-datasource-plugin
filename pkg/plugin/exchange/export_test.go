package exchange

import "time"

// Seams the tests need and production does not. In the test build only.

func (s *Source) cachedToken(audience, subject string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.entries[key(audience, subject)]; e != nil {
		return e.token
	}
	return ""
}

func (s *Source) deadlineFor(audience, subject string) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.entries[key(audience, subject)]; e != nil {
		return e.deadline
	}
	return time.Time{}
}

// retryStateFor exposes the backoff ladder so a test can prove that a caller
// walking away is not recorded as the console having failed.
func (s *Source) retryStateFor(audience, subject string) (attempts int, nextTry time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.entries[key(audience, subject)]; e != nil {
		return e.attempts, e.nextTry
	}
	return 0, time.Time{}
}
