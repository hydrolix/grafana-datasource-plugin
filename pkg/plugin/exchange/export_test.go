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
