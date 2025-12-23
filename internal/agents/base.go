package agents

import (
	"os/exec"
	"regexp"
	"sync"
)

// BaseProvider contains common provider functionality
type BaseProvider struct {
	BinaryPath   string
	modelsCache  []ModelInfo
	modelsCached bool
	mu           sync.RWMutex
}

// IsAvailable checks if the CLI binary is available in PATH
func (b *BaseProvider) IsAvailable() bool {
	_, err := exec.LookPath(b.BinaryPath)
	return err == nil
}

// ParseModelsFromHelp parses models from CLI help output using the provided pattern
// Returns nil if parsing fails
func (b *BaseProvider) ParseModelsFromHelp(helpText string, pattern *regexp.Regexp, modelExtractor func(string) []ModelInfo) []ModelInfo {
	matches := pattern.FindStringSubmatch(helpText)
	if len(matches) < 2 {
		return nil
	}
	return modelExtractor(matches[1])
}

// GetCachedModels returns cached models using double-check locking
// If not cached, calls the fetcher function to populate the cache
func (b *BaseProvider) GetCachedModels(fetcher func() []ModelInfo) []ModelInfo {
	b.mu.RLock()
	if b.modelsCached {
		defer b.mu.RUnlock()
		return b.modelsCache
	}
	b.mu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Double-check after acquiring write lock
	if b.modelsCached {
		return b.modelsCache
	}

	models := fetcher()
	if len(models) > 0 {
		b.modelsCache = models
		b.modelsCached = true
	}

	return b.modelsCache
}

// ModelsToNames extracts enabled model names from ModelInfo slice
func ModelsToNames(models []ModelInfo) []string {
	if len(models) == 0 {
		return nil
	}
	var names []string
	for _, m := range models {
		if m.Enabled {
			names = append(names, m.Name)
		}
	}
	return names
}

// ParseQuotedModels extracts model names from quoted strings like "model1", "model2"
func ParseQuotedModels(text string) []ModelInfo {
	re := regexp.MustCompile(`"([^"]+)"`)
	matches := re.FindAllStringSubmatch(text, -1)

	var models []ModelInfo
	for _, m := range matches {
		if len(m) >= 2 {
			models = append(models, ModelInfo{
				Name:    m[1],
				Enabled: true,
			})
		}
	}
	return models
}

// ParseCommaSeparatedModels extracts model names from comma-separated text
func ParseCommaSeparatedModels(text string) []ModelInfo {
	re := regexp.MustCompile(`[a-zA-Z0-9._-]+`)
	matches := re.FindAllString(text, -1)

	var models []ModelInfo
	for _, name := range matches {
		models = append(models, ModelInfo{
			Name:    name,
			Enabled: true,
		})
	}
	return models
}
