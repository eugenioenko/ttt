package debugadapter

type Config struct {
	Command        []string          `json:"command,omitempty"`
	Transport      string            `json:"transport,omitempty"`
	Address        string            `json:"address,omitempty"`
	AddressPattern string            `json:"addressPattern,omitempty"`
	Languages      []string          `json:"languages,omitempty"`
	Cwd            string            `json:"cwd,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Provider       string            `json:"provider,omitempty"`
}
