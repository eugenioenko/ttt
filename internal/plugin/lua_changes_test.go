package plugin

import "testing"

func TestOpenChangesPassesConfigAndReturnsHostID(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{})
	defer cleanup()
	p.OpenChanges = func(opts ChangesOptions) (string, error) {
		if opts.ID != "one" || opts.Title != "Review" || opts.Description != "Description" || opts.Diff != "patch" {
			t.Errorf("options = %+v", opts)
		}
		return "host-tab", nil
	}
	if err := p.State.DoString(`
local ttt = require("ttt")
assert(ttt.open_changes({id="one", title="Review", description="Description", diff="patch"}) == "host-tab")
assert(not pcall(ttt.open_changes, {diff=""}))
`); err != nil {
		t.Fatal(err)
	}
}
