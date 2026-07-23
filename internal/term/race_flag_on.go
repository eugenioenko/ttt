//go:build race

package term

// raceDetectorEnabled reports whether the binary was built with -race. Tests
// that deliberately induce a data race to verify recovery (e.g. closing the
// event queue while a send is in flight) use this to skip under the detector,
// where the intentional close/send is correctly flagged.
const raceDetectorEnabled = true
