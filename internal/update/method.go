package update

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

type Kind int

const (
	Binary Kind = iota
	Homebrew
	Scoop
	GoInstall
	Nix
	Dev
)

type Method struct {
	Kind Kind
	Path string
}

func DetectMethod(exePath string, info *debug.BuildInfo) Method {
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}
	m := Method{Path: exePath}
	path := strings.ReplaceAll(exePath, `\`, "/")
	switch {
	case strings.HasPrefix(path, "/nix/store/"):
		m.Kind = Nix
	case strings.Contains(path, "/Cellar/"), strings.Contains(path, "/linuxbrew/"):
		m.Kind = Homebrew
	case strings.Contains(strings.ToLower(path), "/scoop/apps/"):
		m.Kind = Scoop
	case info != nil && info.Main.Version != "" && info.Main.Version != "(devel)":
		bins := []string{os.Getenv("GOBIN")}
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			if home, err := os.UserHomeDir(); err == nil {
				gopath = filepath.Join(home, "go")
			}
		}
		for _, root := range filepath.SplitList(gopath) {
			bins = append(bins, filepath.Join(root, "bin"))
		}
		for _, bin := range bins {
			if bin == "" {
				continue
			}
			if resolved, err := filepath.EvalSymlinks(bin); err == nil {
				bin = resolved
			}
			if filepath.Clean(filepath.Dir(exePath)) == filepath.Clean(bin) {
				m.Kind = GoInstall
				break
			}
		}
	}
	return m
}

func CurrentMethod(version string) (Method, error) {
	if version == "dev" {
		return Method{Kind: Dev}, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return Method{}, err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return Method{}, err
	}
	info, _ := debug.ReadBuildInfo()
	return DetectMethod(exe, info), nil
}
