package trash

import (
	"fmt"
	"net/url"
	"strings"
)

// Pass the path as data: interpolating it into the script would execute filenames.
// Foundation handles recovery metadata without Finder or Apple Events permission.
const macScript = `use framework "Foundation"
on run argv
    set sourceURL to current application's NSURL's fileURLWithPath:(item 1 of argv)
    set manager to current application's NSFileManager's defaultManager()
    set {didTrash, destinationURL, trashError} to manager's trashItemAtURL:sourceURL resultingItemURL:(reference) |error|:(reference)
    if not didTrash then error (trashError's localizedDescription() as text)
    return destinationURL's absoluteString() as text
end run`

func move(path string) error {
	_, err := moveMac(path)
	return err
}

func moveMac(path string) (string, error) {
	output, err := run("/usr/bin/osascript", "-l", "AppleScript", "-e", macScript, path)
	if err != nil {
		return "", err
	}
	destination, err := url.Parse(strings.TrimSpace(string(output)))
	if err != nil {
		return "", err
	}
	if destination.Scheme != "file" || destination.Path == "" {
		return "", fmt.Errorf("macOS returned an invalid Trash location: %s", output)
	}
	return destination.Path, nil
}
