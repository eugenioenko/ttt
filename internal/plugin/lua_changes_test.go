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

func TestOpenChangesRejectsInvalidOptions(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{})
	defer cleanup()
	p.OpenChanges = func(opts ChangesOptions) (string, error) { return opts.ID, nil }
	if err := p.State.DoString(`
local ttt=require("ttt")
for _,field in ipairs({"diff","title","description","id"}) do
  for _,value in ipairs({true,42,{},function() end}) do
    local cfg={diff="patch"}; cfg[field]=value
    local ok,err=pcall(ttt.open_changes,cfg)
    assert(not ok and tostring(err):find(field),field)
  end
end
assert(not pcall(ttt.open_changes,{diff="patch",title=""}))
assert(not pcall(ttt.open_changes,{diff="patch",id=" "}))
assert(ttt.open_changes({diff="patch",title="",id="explicit"})=="explicit")
assert(ttt.open_changes({diff="patch"})=="Changes")
`); err != nil {
		t.Fatal(err)
	}
}
