//go:build !race

package term

// raceDetectorEnabled reports whether the binary was built with -race.
const raceDetectorEnabled = false
