package models

// ValidationResult is what the /validate resource reports back to the query
// editor's validation bar. At most one of Error / Warning is set; neither set
// (and Skipped false) means the query is valid. Skipped marks statements the
// validator does not dry-run (DESCRIBE, SHOW, multi-statement input, …).
type ValidationResult struct {
	Error   string `json:"error,omitempty"`
	Warning string `json:"warning,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}
