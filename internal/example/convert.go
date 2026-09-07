package example

import (
	"fmt"
	"strconv"
	"strings"
)

func IntToString(n int) string {
	return strconv.Itoa(n)
}

func StringToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func ToBinary(n int) string {
	return strconv.FormatInt(int64(n), 2)
}

func ToHex(n int) string {
	return fmt.Sprintf("%x", n)
}

func CelsiusToFahrenheit(c float64) float64 {
	return c*9/5 + 32
}

func FahrenheitToCelsius(f float64) float64 {
	return (f - 32) * 5 / 9
}

// TODO: handle negative bytes
func BytesToHuman(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", size, units[unit])
}

func SliceToCSV(items []string) string {
	return strings.Join(items, ",")
}

func CSVToSlice(csv string) []string {
	if csv == "" {
		return nil
	}
	return strings.Split(csv, ",")
}
