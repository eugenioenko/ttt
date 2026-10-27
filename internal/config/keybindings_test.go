package config

import (
	"encoding/json"
	"testing"
)

func TestParseSingleKey(t *testing.T) {
	steps, err := ParseKeyString("ctrl+b")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if !steps[0].Ctrl || steps[0].Rune != 'b' {
		t.Fatalf("expected Ctrl+b, got %+v", steps[0])
	}
}

func TestParseChord(t *testing.T) {
	steps, err := ParseKeyString("ctrl+k ctrl+c")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if !steps[0].Ctrl || steps[0].Rune != 'k' {
		t.Fatalf("step 0: expected Ctrl+k, got %+v", steps[0])
	}
	if !steps[1].Ctrl || steps[1].Rune != 'c' {
		t.Fatalf("step 1: expected Ctrl+c, got %+v", steps[1])
	}
}

func TestParseSpecialKeys(t *testing.T) {
	tests := []struct {
		input   string
		keyName string
	}{
		{"escape", "Escape"},
		{"esc", "Escape"},
		{"enter", "Enter"},
		{"return", "Enter"},
		{"f5", "F5"},
		{"pgup", "PgUp"},
		{"pagedown", "PgDn"},
		{"space", "Space"},
		{"tab", "Tab"},
	}

	for _, tt := range tests {
		steps, err := ParseKeyString(tt.input)
		if err != nil {
			t.Fatalf("ParseKeyString(%q): %v", tt.input, err)
		}
		if steps[0].KeyName != tt.keyName {
			t.Fatalf("ParseKeyString(%q): expected KeyName %q, got %q", tt.input, tt.keyName, steps[0].KeyName)
		}
	}
}

func TestParseModifiers(t *testing.T) {
	steps, err := ParseKeyString("ctrl+shift+f5")
	if err != nil {
		t.Fatal(err)
	}
	if !steps[0].Ctrl || !steps[0].Shift {
		t.Fatalf("expected Ctrl+Shift, got %+v", steps[0])
	}
	if steps[0].KeyName != "F5" {
		t.Fatalf("expected F5, got %q", steps[0].KeyName)
	}
}

func TestParseAltModifier(t *testing.T) {
	steps, err := ParseKeyString("alt+x")
	if err != nil {
		t.Fatal(err)
	}
	if !steps[0].Alt || steps[0].Rune != 'x' {
		t.Fatalf("expected Alt+x, got %+v", steps[0])
	}
}

func TestParseCaseInsensitive(t *testing.T) {
	steps, err := ParseKeyString("Ctrl+B")
	if err != nil {
		t.Fatal(err)
	}
	if !steps[0].Ctrl || steps[0].Rune != 'b' {
		t.Fatalf("expected ctrl+b, got %+v", steps[0])
	}
}

func TestParseErrors(t *testing.T) {
	tests := []string{
		"",
		"ctrl+",
		"ctrl+abc",
		"foo+b",
	}

	for _, input := range tests {
		_, err := ParseKeyString(input)
		if err == nil {
			t.Fatalf("ParseKeyString(%q): expected error", input)
		}
	}
}

func TestDefaultKeybindingsParse(t *testing.T) {
	kb := DefaultKeybindings()
	err := ParseKeyBindings(kb)
	if err != nil {
		t.Fatalf("DefaultKeybindings should parse without error: %v", err)
	}
	for _, b := range kb {
		if len(b.Steps) == 0 {
			t.Fatalf("binding %q has no steps after parsing", b.Key)
		}
	}
}

func TestDefaultClipboardKeybindings(t *testing.T) {
	want := map[string]string{
		"ctrl+c":       "editor.copy",
		"ctrl+insert":  "editor.copy",
		"ctrl+v":       "editor.paste",
		"shift+insert": "editor.paste",
	}
	for _, binding := range DefaultKeybindings() {
		if command, ok := want[binding.Key]; ok {
			delete(want, binding.Key)
			if binding.Command != command {
				t.Errorf("default binding %q = %q, want %q", binding.Key, binding.Command, command)
			}
		}
	}
	for key, command := range want {
		t.Errorf("missing default binding %q for %q", key, command)
	}
}

func TestIsChord(t *testing.T) {
	kb := []KeyBinding{
		{Key: "ctrl+k ctrl+c", Command: "test.chord"},
		{Key: "ctrl+b", Command: "test.single"},
	}
	ParseKeyBindings(kb)

	if !kb[0].IsChord() {
		t.Fatal("ctrl+k ctrl+c should be a chord")
	}
	if kb[1].IsChord() {
		t.Fatal("ctrl+b should not be a chord")
	}
}

func TestParseShiftTab(t *testing.T) {
	steps, err := ParseKeyString("shift+tab")
	if err != nil {
		t.Fatal(err)
	}
	if !steps[0].Shift || steps[0].KeyName != "Tab" {
		t.Fatalf("expected Shift+Tab, got %+v", steps[0])
	}
}

func TestLoadKeybindingsDictSingle(t *testing.T) {
	data := []byte(`{"file.save": "ctrl+s", "editor.undo": "ctrl+z"}`)
	kb, err := LoadKeybindings(data)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, b := range kb {
		found[b.Command] = b.Key
	}
	if found["file.save"] != "ctrl+s" {
		t.Errorf("expected ctrl+s for file.save, got %q", found["file.save"])
	}
	if found["editor.undo"] != "ctrl+z" {
		t.Errorf("expected ctrl+z for editor.undo, got %q", found["editor.undo"])
	}
}

func TestLoadKeybindingsDictMulti(t *testing.T) {
	data := []byte(`{"editor.duplicateLine": ["alt+shift+up", "alt+shift+down"]}`)
	kb, err := LoadKeybindings(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(kb) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(kb))
	}
	keys := map[string]bool{}
	for _, b := range kb {
		if b.Command != "editor.duplicateLine" {
			t.Errorf("unexpected command %q", b.Command)
		}
		keys[b.Key] = true
	}
	if !keys["alt+shift+up"] || !keys["alt+shift+down"] {
		t.Errorf("expected both keys, got %v", keys)
	}
}

func TestLoadKeybindingsClearedEmptyString(t *testing.T) {
	data := []byte(`{"editor.undo": ""}`)
	kb, err := LoadKeybindings(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(kb) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(kb))
	}
	if kb[0].Command != "editor.undo" || kb[0].Key != "" {
		t.Errorf("expected cleared binding, got %+v", kb[0])
	}
}

func TestLoadKeybindingsClearedEmptyArray(t *testing.T) {
	data := []byte(`{"editor.undo": []}`)
	kb, err := LoadKeybindings(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(kb) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(kb))
	}
	if kb[0].Command != "editor.undo" || kb[0].Key != "" {
		t.Errorf("expected cleared binding, got %+v", kb[0])
	}
}

func TestLoadKeybindingsLegacyArray(t *testing.T) {
	data := []byte(`[{"key": "ctrl+s", "command": "file.save"}]`)
	kb, err := LoadKeybindings(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(kb) != 1 || kb[0].Key != "ctrl+s" || kb[0].Command != "file.save" {
		t.Errorf("unexpected binding %+v", kb)
	}
}

func TestLoadKeybindingsInvalid(t *testing.T) {
	_, err := LoadKeybindings([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMergeKeybindingsNoOverrides(t *testing.T) {
	defaults := []KeyBinding{
		{Key: "ctrl+s", Command: "file.save"},
		{Key: "ctrl+z", Command: "editor.undo"},
	}
	merged := MergeKeybindings(defaults, nil)
	if len(merged) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(merged))
	}
}

func TestMergeKeybindingsOverride(t *testing.T) {
	defaults := []KeyBinding{
		{Key: "ctrl+s", Command: "file.save"},
		{Key: "ctrl+z", Command: "editor.undo"},
	}
	overrides := []KeyBinding{
		{Key: "ctrl+shift+s", Command: "file.save"},
	}
	merged := MergeKeybindings(defaults, overrides)
	found := map[string]string{}
	for _, b := range merged {
		found[b.Command] = b.Key
	}
	if found["file.save"] != "ctrl+shift+s" {
		t.Errorf("expected override ctrl+shift+s, got %q", found["file.save"])
	}
	if found["editor.undo"] != "ctrl+z" {
		t.Errorf("expected default ctrl+z preserved, got %q", found["editor.undo"])
	}
}

func TestMergeKeybindingsCleared(t *testing.T) {
	defaults := []KeyBinding{
		{Key: "ctrl+s", Command: "file.save"},
		{Key: "ctrl+z", Command: "editor.undo"},
	}
	overrides := []KeyBinding{
		{Key: "", Command: "editor.undo"},
	}
	merged := MergeKeybindings(defaults, overrides)
	for _, b := range merged {
		if b.Command == "editor.undo" {
			t.Fatal("cleared binding should not appear in merged result")
		}
	}
	if len(merged) != 1 || merged[0].Command != "file.save" {
		t.Errorf("expected only file.save, got %+v", merged)
	}
}

func TestMergeKeybindingsNewDefault(t *testing.T) {
	defaults := []KeyBinding{
		{Key: "ctrl+s", Command: "file.save"},
		{Key: "ctrl+k y", Command: "view.keybindings"},
	}
	overrides := []KeyBinding{
		{Key: "ctrl+shift+s", Command: "file.save"},
	}
	merged := MergeKeybindings(defaults, overrides)
	found := map[string]string{}
	for _, b := range merged {
		found[b.Command] = b.Key
	}
	if found["view.keybindings"] != "ctrl+k y" {
		t.Errorf("new default should be preserved, got %q", found["view.keybindings"])
	}
}

func TestSaveKeybindingsOnlyOverrides(t *testing.T) {
	defaults := DefaultKeybindings()
	// Change one binding
	bindings := make([]KeyBinding, len(defaults))
	copy(bindings, defaults)
	for i, b := range bindings {
		if b.Command == "file.save" {
			bindings[i].Key = "ctrl+shift+s"
			break
		}
	}

	defaultMap := make(map[string][]string)
	for _, kb := range defaults {
		defaultMap[kb.Command] = append(defaultMap[kb.Command], kb.Key)
	}
	currentMap := make(map[string][]string)
	for _, kb := range bindings {
		currentMap[kb.Command] = append(currentMap[kb.Command], kb.Key)
	}

	// Count diffs
	diffs := 0
	for cmd, keys := range currentMap {
		if !stringSliceEqual(keys, defaultMap[cmd]) {
			diffs++
		}
	}
	if diffs != 1 {
		t.Fatalf("expected 1 diff, got %d", diffs)
	}
}

func TestSaveKeybindingsClearedShowsEmpty(t *testing.T) {
	defaults := DefaultKeybindings()
	// Remove one binding
	var bindings []KeyBinding
	for _, b := range defaults {
		if b.Command != "file.save" {
			bindings = append(bindings, b)
		}
	}

	defaultMap := make(map[string][]string)
	for _, kb := range defaults {
		defaultMap[kb.Command] = append(defaultMap[kb.Command], kb.Key)
	}
	currentMap := make(map[string][]string)
	for _, kb := range bindings {
		currentMap[kb.Command] = append(currentMap[kb.Command], kb.Key)
	}

	// file.save should be detected as cleared
	if _, exists := currentMap["file.save"]; exists {
		t.Fatal("file.save should not be in current map")
	}
	if _, exists := defaultMap["file.save"]; !exists {
		t.Fatal("file.save should be in default map")
	}
}

func TestSaveKeybindingsAllDefaultsNoFile(t *testing.T) {
	defaults := DefaultKeybindings()
	defaultMap := make(map[string][]string)
	for _, kb := range defaults {
		defaultMap[kb.Command] = append(defaultMap[kb.Command], kb.Key)
	}
	currentMap := make(map[string][]string)
	for _, kb := range defaults {
		currentMap[kb.Command] = append(currentMap[kb.Command], kb.Key)
	}

	diffs := 0
	for cmd, keys := range currentMap {
		if !stringSliceEqual(keys, defaultMap[cmd]) {
			diffs++
		}
	}
	clears := 0
	for cmd := range defaultMap {
		if _, exists := currentMap[cmd]; !exists {
			clears++
		}
	}
	if diffs != 0 || clears != 0 {
		t.Fatalf("no diffs expected for identical bindings, got %d diffs %d clears", diffs, clears)
	}
}

func TestRoundTripOverride(t *testing.T) {
	defaults := []KeyBinding{
		{Key: "ctrl+s", Command: "file.save"},
		{Key: "ctrl+z", Command: "editor.undo"},
	}

	// Simulate an override saved as dict
	saved := []byte(`{"file.save": "ctrl+shift+s"}`)
	overrides, err := LoadKeybindings(saved)
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeKeybindings(defaults, overrides)

	found := map[string]string{}
	for _, b := range merged {
		found[b.Command] = b.Key
	}
	if found["file.save"] != "ctrl+shift+s" {
		t.Errorf("override not applied: got %q", found["file.save"])
	}
	if found["editor.undo"] != "ctrl+z" {
		t.Errorf("default not preserved: got %q", found["editor.undo"])
	}
}

func TestRoundTripCleared(t *testing.T) {
	defaults := []KeyBinding{
		{Key: "ctrl+s", Command: "file.save"},
		{Key: "ctrl+z", Command: "editor.undo"},
	}

	saved := []byte(`{"editor.undo": ""}`)
	overrides, err := LoadKeybindings(saved)
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeKeybindings(defaults, overrides)

	if len(merged) != 1 || merged[0].Command != "file.save" {
		t.Errorf("expected only file.save after clear, got %+v", merged)
	}
}

func TestRoundTripClearedEmptyArray(t *testing.T) {
	defaults := []KeyBinding{
		{Key: "ctrl+s", Command: "file.save"},
		{Key: "ctrl+z", Command: "editor.undo"},
	}

	saved := []byte(`{"editor.undo": []}`)
	overrides, err := LoadKeybindings(saved)
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeKeybindings(defaults, overrides)

	if len(merged) != 1 || merged[0].Command != "file.save" {
		t.Errorf("expected only file.save after clear via [], got %+v", merged)
	}
}

func TestMarshalOrderedMap(t *testing.T) {
	m := map[string]any{
		"b": "val_b",
		"a": "val_a",
	}
	data, err := marshalOrderedMap(m, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, data)
	}
	if parsed["a"] != "val_a" || parsed["b"] != "val_b" {
		t.Errorf("unexpected values: %v", parsed)
	}
}
