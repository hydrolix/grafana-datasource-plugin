package exchange

import "time"

// Seams the tests need and production does not. In the test build only.

func (s *Source) cachedToken(slot Slot) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.entries[key(slot)]; e != nil {
		return e.token
	}
	return ""
}

func (s *Source) deadlineFor(slot Slot) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.entries[key(slot)]; e != nil {
		return e.deadline
	}
	return time.Time{}
}

// entryCount is the size of the cache, so a test can prove it shrinks.
func (s *Source) entryCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// retryStateFor exposes the backoff ladder so a test can prove that a caller
// walking away is not recorded as the console having failed.
func (s *Source) retryStateFor(slot Slot) (attempts int, nextTry time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.entries[key(slot)]; e != nil {
		return e.attempts, e.nextTry
	}
	return 0, time.Time{}
}
