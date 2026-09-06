package session

import (
	"testing"

	"harness/internal/config"
	"harness/internal/events"
)

func testRegistryWithAgents(t *testing.T, agents []config.Agent) *Registry {
	t.Helper()
	bus := events.NewBus()
	writers, err := events.NewWriters(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { writers.Close() })
	cfg := config.Defaults(t.TempDir())
	cfg.Servers[0].ID = "first"
	cfg.Agents = agents
	snapshot := cfg
	profiles := map[string]*config.Profile{cfg.Servers[0].ID: &cfg.Servers[0]}
	return NewRegistry(bus, writers, func(id string) (*config.Profile, bool) {
		p, ok := profiles[id]
		return p, ok
	}, 40, func() config.Config { return snapshot })
}

func TestCreateAppliesAgentToolRestrictions(t *testing.T) {
	registry := testRegistryWithAgents(t, []config.Agent{
		{ID: "scout", Label: "Scout", ToolsEnabled: map[string]bool{"write_file": false, "shell": false}},
	})
	item, err := registry.Create("probe", "scout", "first", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if item.AgentID != "scout" {
		t.Fatalf("agent_id=%q", item.AgentID)
	}
	if item.ToolEnabled("write_file") || item.ToolEnabled("shell") {
		t.Fatalf("restricted tools enabled: %+v", item.ToolsEnabled)
	}
	if !item.ToolEnabled("read_file") || !item.ToolEnabled("remember") {
		t.Fatalf("unrestricted tools disabled: %+v", item.ToolsEnabled)
	}
}

func TestCreateWithoutAgentKeepsAllTools(t *testing.T) {
	registry := testRegistryWithAgents(t, nil)
	item, err := registry.Create("probe", "", "first", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range config.AllToolNames {
		if !item.ToolEnabled(name) {
			t.Fatalf("%s disabled for unbound session", name)
		}
	}
}

func TestSetAgentRebindsTools(t *testing.T) {
	registry := testRegistryWithAgents(t, []config.Agent{
		{ID: "tester", Label: "Tester", ToolsEnabled: map[string]bool{"edit_file": false}},
	})
	item, err := registry.Create("probe", "", "first", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !item.ToolEnabled("edit_file") {
		t.Fatalf("expected full tool set before agent binding")
	}
	if err := registry.SetAgent(item.ID, "tester"); err != nil {
		t.Fatal(err)
	}
	if item.ToolEnabled("edit_file") {
		t.Fatalf("edit_file still enabled under tester")
	}
	if err := registry.SetAgent(item.ID, ""); err != nil {
		t.Fatal(err)
	}
	if !item.ToolEnabled("edit_file") {
		t.Fatalf("edit_file not restored after clearing agent")
	}
}

func TestSetAgentRejectsUnknownAgent(t *testing.T) {
	registry := testRegistryWithAgents(t, nil)
	item, err := registry.Create("probe", "", "first", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.SetAgent(item.ID, "ghost"); err == nil {
		t.Fatal("expected unknown agent error")
	}
}

func TestAgentInUseBlocksAndReports(t *testing.T) {
	registry := testRegistryWithAgents(t, []config.Agent{{ID: "coder", Label: "Coder"}})
	item, err := registry.Create("probe", "coder", "first", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if sessionID, used := registry.AgentInUse("coder"); !used || sessionID != item.ID {
		t.Fatalf("AgentInUse=%q,%t", sessionID, used)
	}
	if _, used := registry.AgentInUse("tester"); used {
		t.Fatal("tester reported in use")
	}
}
