package session

import (
	"testing"

	"harness/internal/config"
)

func TestAgentCeilingBlocksToolToggleReenable(t *testing.T) {
	registry := testRegistryWithAgents(t, []config.Agent{
		{ID: "scout", Label: "Scout", ToolsEnabled: map[string]bool{"write_file": false}},
	})
	item, err := registry.Create("probe", "scout", "first", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if item.ToolEnabled("write_file") {
		t.Fatal("write_file enabled at creation under scout")
	}
	// The old defect: a session toggle could re-enable an agent-restricted tool.
	if ok, reason := item.ToggleTool("write_file", true); ok || reason == "" {
		t.Fatalf("ceiling violated: ok=%t reason=%q enabled=%t", ok, reason, item.ToolEnabled("write_file"))
	}
	if item.ToolEnabled("write_file") {
		t.Fatal("write_file enabled after denied toggle")
	}
	// Disabling an allowed tool still works, and re-enabling it is fine.
	if ok, _ := item.ToggleTool("read_file", false); !ok || item.ToolEnabled("read_file") {
		t.Fatal("allowed tool cannot be toggled off")
	}
	if ok, _ := item.ToggleTool("read_file", true); !ok || !item.ToolEnabled("read_file") {
		t.Fatal("allowed tool cannot be toggled on")
	}
}

func TestCeilingHidesToolInEnabledToolsAndSnapshot(t *testing.T) {
	registry := testRegistryWithAgents(t, []config.Agent{
		{ID: "scout", Label: "Scout", ToolsEnabled: map[string]bool{"write_file": false}},
	})
	item, err := registry.Create("probe", "scout", "first", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Simulate internal state drift: ToolsEnabled flipped without the ceiling check.
	item.mu.Lock()
	item.ToolsEnabled["write_file"] = true
	item.mu.Unlock()
	if item.ToolEnabled("write_file") {
		t.Fatal("ToolEnabled ignores ceiling")
	}
	if item.EnabledTools()["write_file"] {
		t.Fatal("EnabledTools ignores ceiling")
	}
	snap := item.Snapshot()
	for _, tool := range snap.Tools {
		if tool.Name == "write_file" && tool.Enabled {
			t.Fatal("Snapshot reports write_file enabled despite ceiling")
		}
	}
}

func TestSetAgentClearsCeiling(t *testing.T) {
	registry := testRegistryWithAgents(t, []config.Agent{
		{ID: "scout", Label: "Scout", ToolsEnabled: map[string]bool{"write_file": false}},
	})
	item, err := registry.Create("probe", "scout", "first", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := item.ToggleTool("write_file", true); ok {
		t.Fatal("toggle should be denied under scout ceiling")
	}
	if err := registry.SetAgent(item.ID, ""); err != nil {
		t.Fatal(err)
	}
	if ok, _ := item.ToggleTool("write_file", true); !ok || !item.ToolEnabled("write_file") {
		t.Fatal("ceiling not cleared after unbinding agent")
	}
}
