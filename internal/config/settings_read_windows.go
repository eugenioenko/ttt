//go:build windows

package config

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func readSettingsFileBytes(path string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		data, err := os.ReadFile(path)
		if err == nil || !errors.Is(err, windows.ERROR_SHARING_VIOLATION) || attempt == 4 {
			return data, err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
}
