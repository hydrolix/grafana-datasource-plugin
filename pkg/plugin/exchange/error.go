package exchange

import (
	"errors"
	"fmt"
)

// Outcome is what a failed exchange means for the caller. The three are
// separate because the remedies differ and a panel should say which one
// happened: a refusal is about this person's access, unavailability is about
// the console, and a misconfiguration is about the operator.
type Outcome int

const (
	// Refused: the console answered, and the answer was no.
	Refused Outcome = iota + 1
	// Unavailable: the console did not answer, or answered that it could not
	// mint just now — a role removal converging, Keycloak unreachable,
	// provisioning deferred.
	Unavailable
	// Misconfigured: the operator's problem rather than this person's — no
	// delegate credential for this cluster, a credential the console refuses,
	// or an exchange URL that reaches something other than the console's
	// facade. The message never names the cluster or says which of those it
	// is: the credential map is instance-wide, and a message that
	// distinguished "no delegate for this host" from the rest would tell an
	// org admin on a shared Grafana which other customers' clusters it serves.
	Misconfigured
)

// String names the outcome for a log line, in a closed vocabulary.
func (o Outcome) String() string {
	switch o {
	case Refused:
		return "refused"
	case Unavailable:
		return "unavailable"
	case Misconfigured:
		return "misconfigured"
	default:
		return "error"
	}
}

// message is what a panel reads for the outcome, before any detail.
func (o Outcome) message() string {
	switch o {
	case Refused:
		return "exchange: the console refused this account access to this cluster"
	case Unavailable:
		return "exchange: the console could not be reached"
	case Misconfigured:
		return "exchange: no exchange configuration is usable"
	default:
		return "exchange: failed"
	}
}

// Error is every failure this package reports: an outcome, and the detail a
// panel should read after it. There is no other shape, so a caller finds the
// outcome and the detail with one errors.As and never has to walk a chain of
// wrappers to the right level. Detail never carries token material.
type Error struct {
	Outcome Outcome
	Detail  string
	// Cause, when set, is what errors.Is sees underneath — the configuration
	// error behind a misconfiguration, for one.
	Cause error
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Outcome.message()
	}
	return e.Outcome.message() + ": " + e.Detail
}

func (e *Error) Unwrap() error { return e.Cause }

// Is answers by outcome, so errors.Is(err, ErrRefused) holds for every refusal
// whatever its detail. The bare outcomes below are the vocabulary callers
// compare against.
func (e *Error) Is(target error) bool {
	var t *Error
	return errors.As(target, &t) && t.Outcome == e.Outcome
}

// The bare outcomes: returned as they are when there is nothing to add, and
// the targets for errors.Is everywhere else.
var (
	ErrRefused       = &Error{Outcome: Refused}
	ErrUnavailable   = &Error{Outcome: Unavailable}
	ErrMisconfigured = &Error{Outcome: Misconfigured}
)

// outcome names an error for a log line, in a closed vocabulary.
func outcome(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Outcome.String()
	}
	return Outcome(0).String()
}

// failf builds an Error with a formatted detail.
func failf(o Outcome, format string, args ...any) *Error {
	return &Error{Outcome: o, Detail: fmt.Sprintf(format, args...)}
}
