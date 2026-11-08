package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type Release struct {
	Tag    string  `json:"tag_name"`
	Notes  string  `json:"body"`
	Assets []Asset `json:"assets"`
}

type Service struct {
	Client *http.Client
	URL    string
}

func Latest(ctx context.Context) (Release, error) { return (Service{}).Latest(ctx) }

func Apply(ctx context.Context, release Release, method Method) error {
	return (Service{}).Apply(ctx, release, method)
}

func (s Service) get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ttt-update")
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

func (s Service) Latest(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	url := s.URL
	if url == "" {
		url = "https://api.github.com/repos/eugenioenko/ttt/releases/latest"
	}
	body, err := s.get(ctx, url)
	if err != nil {
		return Release{}, err
	}
	defer body.Close()
	var release Release
	if err := json.NewDecoder(io.LimitReader(body, 2<<20)).Decode(&release); err != nil {
		return Release{}, err
	}
	if _, err := CompareVersions(release.Tag, "0.0.0"); err != nil || release.Tag == "dev" {
		return Release{}, fmt.Errorf("invalid release tag %q", release.Tag)
	}
	return release, nil
}

func AssetName(goos, goarch string) string {
	name := "ttt-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func (r Release) assetURL(name string) (string, error) {
	for _, asset := range r.Assets {
		if asset.Name == name && asset.URL != "" {
			return asset.URL, nil
		}
	}
	return "", fmt.Errorf("release %s has no %s asset", r.Tag, name)
}

func (s Service) Apply(ctx context.Context, release Release, method Method) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var command string
	var args []string
	switch method.Kind {
	case Homebrew:
		command, args = "brew", []string{"upgrade", "ttt"}
	case Scoop:
		// Scoop is a PowerShell script, not an executable accepted by CreateProcess.
		command, args = "powershell.exe", []string{"-NoProfile", "-NonInteractive", "-Command", "scoop update ttt"}
	case GoInstall:
		command, args = "go", []string{"install", "github.com/eugenioenko/ttt/cmd/ttt@latest"}
	case Binary:
		return s.replace(ctx, release, method.Path, AssetName(runtime.GOOS, runtime.GOARCH), nil)
	default:
		return fmt.Errorf("installation method does not support updates")
	}
	cmd := exec.CommandContext(ctx, command, args...)
	output := &tailWriter{}
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w\n%s", strings.Join(append([]string{command}, args...), " "), err, output.data)
	}
	return nil
}

type tailWriter struct{ data []byte }

func (w *tailWriter) Write(p []byte) (int, error) {
	const limit = 8192
	n := len(p)
	if len(p) >= limit {
		w.data = append(w.data[:0], p[len(p)-limit:]...)
	} else {
		w.data = append(w.data, p...)
		if len(w.data) > limit {
			w.data = w.data[len(w.data)-limit:]
		}
	}
	return n, nil
}
