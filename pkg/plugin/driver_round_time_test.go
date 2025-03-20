package plugin

import (
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"testing"
	"time"
)

var defaultTimeRange = backend.TimeRange{
	To:   time.Unix(1740678412, 123456789),
	From: time.Unix(1740674812, 123456789),
}

func TestRoundToSecond(t *testing.T) {
	timeRange := roundTimeRange(defaultTimeRange, "1s")
	if timeRange.To != time.Unix(1740678412, 0) {
		t.Error("To time should be rounded to 1s")
	}
	if timeRange.From != time.Unix(1740674812, 0) {
		t.Error("From time should be rounded to 1s")
	}
}

func TestRoundToMinute(t *testing.T) {
	timeRange := roundTimeRange(defaultTimeRange, "1m")
	if timeRange.To != time.Unix(1740678420, 0) {
		t.Error("To time should be rounded to 1m")
	}
	if timeRange.From != time.Unix(1740674820, 0) {
		t.Error("From time should be rounded to 1m")
	}
}

func TestRoundToHour(t *testing.T) {
	timeRange := roundTimeRange(defaultTimeRange, "1h")
	if timeRange.To != time.Unix(1740679200, 0) {
		t.Error("To time should be rounded to 1h")
	}
	if timeRange.From != time.Unix(1740675600, 0) {
		t.Error("From time should be rounded to 1h")
	}
}

func TestRoundToZero(t *testing.T) {
	timeRange := roundTimeRange(defaultTimeRange, "0")
	if timeRange != defaultTimeRange {
		t.Error("TimeRange should not be rounded")
	}
}

func TestRoundEmpty(t *testing.T) {
	timeRange := roundTimeRange(defaultTimeRange, "")
	if timeRange != defaultTimeRange {
		t.Error("TimeRange should not be rounded")
	}
}

func TestRoundInvalid(t *testing.T) {
	timeRange := roundTimeRange(defaultTimeRange, "not valid duration")
	if timeRange != defaultTimeRange {
		t.Error("TimeRange should not be rounded")
	}
}
