package config

import "encoding/json"

const DefaultLogModeMinMB = 64

func (e EditorSettings) LogModeMinBytes() (int64, bool) {
	var threshold *MBThreshold
	if e.LargeFiles != nil {
		threshold = e.LargeFiles.LogModeMinMB
	}
	if threshold == nil || threshold.unset {
		return megabytes(nil, DefaultLogModeMinMB), true
	}
	if threshold.Off {
		return 0, false
	}
	return megabytes(&threshold.MB, DefaultLogModeMinMB), true
}

type MBThreshold struct {
	Off   bool
	MB    int
	unset bool
}

func (t *MBThreshold) UnmarshalJSON(data []byte) error {
	*t = MBThreshold{}
	if string(data) == "false" {
		t.Off = true
		return nil
	}
	if err := json.Unmarshal(data, &t.MB); err != nil || t.MB < 0 || string(data) == "null" {
		t.unset = true
	}
	return nil
}

func (t MBThreshold) MarshalJSON() ([]byte, error) {
	if t.Off {
		return []byte("false"), nil
	}
	return json.Marshal(t.MB)
}

func (l *LargeFileSettings) UnmarshalJSON(data []byte) error {
	type plain LargeFileSettings
	if err := json.Unmarshal(data, (*plain)(l)); err != nil {
		return err
	}
	if l.LogModeMinMB != nil && l.LogModeMinMB.unset {
		l.LogModeMinMB = nil
	}
	return nil
}
