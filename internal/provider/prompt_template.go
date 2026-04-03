package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type PromptTemplate struct {
	ID          string
	Name        string
	Version     string
	Content     string
	Variables   []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Description string
}

type PromptTemplateManager interface {
	Get(ctx context.Context, name string) (*PromptTemplate, error)
	GetVersion(ctx context.Context, name, version string) (*PromptTemplate, error)
	Save(ctx context.Context, template PromptTemplate) error
	List(ctx context.Context) ([]PromptTemplate, error)
	Render(ctx context.Context, name string, vars map[string]string) (string, error)
	LatestVersion(ctx context.Context, name string) (string, error)
}

type InMemoryPromptManager struct {
	templates map[string]map[string]PromptTemplate
	mu        sync.RWMutex
}

func NewInMemoryPromptManager() *InMemoryPromptManager {
	return &InMemoryPromptManager{
		templates: make(map[string]map[string]PromptTemplate),
	}
}

func (m *InMemoryPromptManager) Get(ctx context.Context, name string) (*PromptTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, ok := m.templates[name]
	if !ok {
		return nil, nil
	}

	var latest *PromptTemplate
	for _, v := range versions {
		if latest == nil || v.CreatedAt.After(latest.CreatedAt) {
			latest = &v
		}
	}
	return latest, nil
}

func (m *InMemoryPromptManager) GetVersion(ctx context.Context, name, version string) (*PromptTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, ok := m.templates[name]
	if !ok {
		return nil, nil
	}

	t, ok := versions[version]
	if !ok {
		return nil, nil
	}
	return &t, nil
}

func (m *InMemoryPromptManager) Save(ctx context.Context, template PromptTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.templates[template.Name] == nil {
		m.templates[template.Name] = make(map[string]PromptTemplate)
	}
	m.templates[template.Name][template.Version] = template
	return nil
}

func (m *InMemoryPromptManager) List(ctx context.Context) ([]PromptTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []PromptTemplate
	for _, versions := range m.templates {
		for _, v := range versions {
			result = append(result, v)
		}
	}
	return result, nil
}

func (m *InMemoryPromptManager) Render(ctx context.Context, name string, vars map[string]string) (string, error) {
	template, err := m.Get(ctx, name)
	if err != nil {
		return "", err
	}
	if template == nil {
		return "", nil
	}

	content := template.Content
	for key, value := range vars {
		content = replaceAll(content, "{{"+key+"}}", value)
	}
	return content, nil
}

func (m *InMemoryPromptManager) LatestVersion(ctx context.Context, name string) (string, error) {
	template, err := m.Get(ctx, name)
	if err != nil {
		return "", err
	}
	if template == nil {
		return "", nil
	}
	return template.Version, nil
}

func replaceAll(s, old, new string) string {
	result := ""
	for {
		idx := findSubstring(s, old)
		if idx == -1 {
			break
		}
		result += s[:idx] + new
		s = s[idx+len(old):]
	}
	return result + s
}

func findSubstring(s, sub string) int {
	n := len(sub)
	for i := 0; i <= len(s)-n; i++ {
		if s[i:i+n] == sub {
			return i
		}
	}
	return -1
}

func HashPrompt(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
