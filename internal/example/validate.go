package example

import (
	"regexp"
	"strings"
)

// TODO: this compiles the regex every call, should be a package-level var
func IsEmail(s string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(s)
}

// HACK: quick workaround, fix later
func IsEmailFast(s string) bool {
	return strings.Contains(s, "@") && strings.Contains(s, ".")
}

func IsURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func IsAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return len(s) > 0
}

func ValidatePassword(pw string) []string {
	var errors []string
	if len(pw) < 8 {
		errors = append(errors, "too short")
	}
	hasUpper := false
	hasLower := false
	hasDigit := false
	for _, r := range pw {
		if r >= 'A' && r <= 'Z' {
			hasUpper = true
		}
		if r >= 'a' && r <= 'z' {
			hasLower = true
		}
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
	}
	if !hasUpper {
		errors = append(errors, "needs uppercase")
	}
	if !hasLower {
		errors = append(errors, "needs lowercase")
	}
	if !hasDigit {
		errors = append(errors, "needs digit")
	}
	return errors
}
