package config

import (
	"testing"
	"time"
)

func TestDefaultSettingsLeaveStartupUpdateChecksDisabled(t *testing.T) {
	s := DefaultSettings()
	if s.Update.StartupCheck {
		t.Fatal("startup update checks must be disabled by default")
	}
	now := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)
	if s.Update.StartupCheckDue(time.Time{}, now) {
		t.Fatal("disabled default must not be due, even with no prior check")
	}
}

func TestStartupUpdateCheckFrequency(t *testing.T) {
	now := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)
	hourAgo := now.Add(-time.Hour)
	dayAgo := now.Add(-24 * time.Hour)
	justUnderDay := now.Add(-24*time.Hour + time.Second)
	justOverDay := now.Add(-24*time.Hour - time.Second)

	tests := []struct {
		name     string
		settings UpdateSettings
		last     time.Time
		want     bool
	}{
		{name: "disabled ignores a due interval", settings: UpdateSettings{StartupCheck: false, StartupCheckInterval: UpdateCheckDaily}, last: dayAgo, want: false},
		{name: "enabled with no prior check is due", settings: UpdateSettings{StartupCheck: true}, last: time.Time{}, want: true},
		{name: "daily is not due before 24 hours", settings: UpdateSettings{StartupCheck: true, StartupCheckInterval: UpdateCheckDaily}, last: justUnderDay, want: false},
		{name: "daily is due at exactly 24 hours", settings: UpdateSettings{StartupCheck: true, StartupCheckInterval: UpdateCheckDaily}, last: dayAgo, want: true},
		{name: "daily is due after 24 hours", settings: UpdateSettings{StartupCheck: true, StartupCheckInterval: UpdateCheckDaily}, last: justOverDay, want: true},
		{name: "empty interval is daily", settings: UpdateSettings{StartupCheck: true}, last: hourAgo, want: false},
		{name: "every launch is due even when recent", settings: UpdateSettings{StartupCheck: true, StartupCheckInterval: UpdateCheckEveryLaunch}, last: hourAgo, want: true},
		{name: "daily is due when last check is in the future", settings: UpdateSettings{StartupCheck: true, StartupCheckInterval: UpdateCheckDaily}, last: now.Add(24 * time.Hour), want: true},
		{name: "disabled stays off when last check is in the future", settings: UpdateSettings{StartupCheck: false, StartupCheckInterval: UpdateCheckDaily}, last: now.Add(24 * time.Hour), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.settings.StartupCheckDue(tt.last, now); got != tt.want {
				t.Fatalf("StartupCheckDue = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeStartupUpdateCheckInterval(t *testing.T) {
	s := DefaultSettings()
	s.Update.StartupCheckInterval = "weekly"
	normalizeSettings(&s)
	if s.Update.StartupCheckInterval != UpdateCheckDaily {
		t.Fatalf("unknown interval = %q, want %q", s.Update.StartupCheckInterval, UpdateCheckDaily)
	}
	s.Update.StartupCheckInterval = UpdateCheckEveryLaunch
	normalizeSettings(&s)
	if s.Update.StartupCheckInterval != UpdateCheckEveryLaunch {
		t.Fatalf("valid interval was rewritten to %q", s.Update.StartupCheckInterval)
	}
}
