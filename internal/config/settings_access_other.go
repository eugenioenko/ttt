//go:build !windows

package config

func preserveSettingsFileAccess(_, _ string) error { return nil }
