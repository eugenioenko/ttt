package debugadapter

import (
	"bytes"
	"encoding/json"
)

type FileLaunchOverride struct {
	Pattern   string         `json:"pattern"`
	Arguments map[string]any `json:"arguments"`
}

type Config struct {
	FileLaunch          map[string]any       `json:"fileLaunch,omitzero"`
	FileLaunchOverrides []FileLaunchOverride `json:"fileLaunchOverrides,omitempty"`
	Command             []string             `json:"command,omitempty"`
	Transport           string               `json:"transport,omitempty"`
	Address             string               `json:"address,omitempty"`
	AddressPattern      string               `json:"addressPattern,omitempty"`
	Languages           []string             `json:"languages,omitempty"`
	Markers             []string             `json:"markers,omitempty"`
	Configurations      []map[string]any     `json:"configurations,omitempty"`
	Cwd                 string               `json:"cwd,omitempty"`
	Env                 map[string]string    `json:"env,omitempty"`
	Provider            string               `json:"provider,omitempty"`
}

func (c *Config) UnmarshalJSON(data []byte) error {
	type plain Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode((*plain)(c))
}
