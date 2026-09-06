package agent

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"harness/internal/config"
	"harness/internal/session"
)

type PromptRenderer struct {
	mu         sync.RWMutex
	path, text string
	agents     func(id string) (config.Agent, bool)
}

func LoadTemplate(path string) (*PromptRenderer, error) {
	r := &PromptRenderer{path: path}
	if err := r.Reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// SetAgents installs the roster resolver used to render {{persona}} and
// {{agent}} from the session's agent.
func (r *PromptRenderer) SetAgents(resolver func(id string) (config.Agent, bool)) {
	r.mu.Lock()
	r.agents = resolver
	r.mu.Unlock()
}
func (r *PromptRenderer) Reload() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return fmt.Errorf("system prompt %s: %w", r.path, err)
	}
	r.mu.Lock()
	r.text = string(data)
	r.mu.Unlock()
	return nil
}
func (r *PromptRenderer) Render(profile *config.Profile, s *session.Session, toolNames []string, memory string) string {
	r.mu.RLock()
	template := r.text
	resolver := r.agents
	r.mu.RUnlock()
	if profile.SystemPromptOverride != "" {
		template = profile.SystemPromptOverride
	}
	agentLabel, persona := "", ""
	if resolver != nil && s.AgentID != "" {
		if item, ok := resolver(s.AgentID); ok {
			agentLabel = item.Label
			if agentLabel == "" {
				agentLabel = item.ID
			}
			persona = strings.TrimSpace(item.Persona)
		}
	}
	value := strings.ReplaceAll(template, "{{workspace}}", s.Workspace)
	value = strings.ReplaceAll(value, "{{tools}}", strings.Join(toolNames, ", "))
	value = strings.ReplaceAll(value, "{{memory}}", memory)
	value = strings.ReplaceAll(value, "{{agent}}", agentLabel)
	value = strings.ReplaceAll(value, "{{persona}}", persona)
	value = strings.ReplaceAll(value, "{{os_context}}", operatingSystemContext())
	value = strings.ReplaceAll(value, "{{date}}", time.Now().Format("2006-01-02"))
	if memory == "" {
		value = strings.TrimRight(value, "\r\n")
	}
	if persona == "" {
		value = collapseBlankLines(value)
	}
	return value
}

// collapseBlankLines removes lines left empty by an unset persona or memory
// slot so the rendered system prompt carries no dangling whitespace.
func collapseBlankLines(value string) string {
	lines := strings.Split(value, "\n")
	out := lines[:0]
	for _, line := range lines {
		if strings.TrimSpace(strings.TrimRight(line, "\r")) == "" && len(out) > 0 && strings.TrimSpace(strings.TrimRight(out[len(out)-1], "\r")) == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
