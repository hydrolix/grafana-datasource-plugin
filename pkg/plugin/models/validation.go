package models

// ValidationResult is the /validate response data. At most one of Error /
// Warning is set; none set means valid. Skipped: not a single SELECT.
type ValidationResult struct {
	Error   string `json:"error,omitempty"`
	Warning string `json:"warning,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}
