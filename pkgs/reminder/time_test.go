package reminder

import (
	"testing"
	"time"
)

func TestParseRelativeDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"5m":     5 * time.Minute,
		"1h":     time.Hour,
		"1h30m":  90 * time.Minute,
		"2d":     48 * time.Hour,
		"1d2h5m": 26*time.Hour + 5*time.Minute,
	}
	for spec, want := range cases {
		got, err := ParseRelativeDuration(spec)
		if err != nil {
			t.Fatalf("%s: %v", spec, err)
		}
		if got != want {
			t.Fatalf("%s: got %v want %v", spec, got, want)
		}
	}
}

func TestParseTimeSpecCron(t *testing.T) {
	kind, runAt, cronExpr, err := ParseTimeSpec("0 8 * * *")
	if err != nil {
		t.Fatalf("parse cron: %v", err)
	}
	if kind != ScheduleCron || cronExpr != "0 8 * * *" || runAt == nil {
		t.Fatalf("unexpected cron parse: kind=%s expr=%s runAt=%v", kind, cronExpr, runAt)
	}
}
