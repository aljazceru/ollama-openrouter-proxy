package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type orModels struct {
	Data []struct {
		ID                   string   `json:"id"`
		ContextLength        int      `json:"context_length"`
		SupportedParameters  []string `json:"supported_parameters"`
		TopProvider          struct {
			ContextLength int `json:"context_length"`
		} `json:"top_provider"`
		Pricing struct {
			Prompt     string `json:"prompt"`
			Completion string `json:"completion"`
		} `json:"pricing"`
	} `json:"data"`
}

// supportsToolUse checks if a model supports tool use by looking for "tools" in supported_parameters
func supportsToolUse(supportedParams []string) bool {
	for _, param := range supportedParams {
		if param == "tools" || param == "tool_choice" {
			return true
		}
	}
	return false
}

func fetchFreeModels(apiKey string) ([]string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	var result orModels
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	// Check if tool use filtering is enabled
	toolUseOnly := strings.ToLower(os.Getenv("TOOL_USE_ONLY")) == "true"
	
	type item struct {
		id  string
		ctx int
	}
	var items []item
	for _, m := range result.Data {
		if m.Pricing.Prompt == "0" && m.Pricing.Completion == "0" {
			// If tool use filtering is enabled, skip models that don't support tools
			if toolUseOnly && !supportsToolUse(m.SupportedParameters) {
				continue
			}
			
			ctx := m.TopProvider.ContextLength
			if ctx == 0 {
				ctx = m.ContextLength
			}
			items = append(items, item{id: m.ID, ctx: ctx})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ctx > items[j].ctx })
	models := make([]string, len(items))
	for i, it := range items {
		models[i] = it.id
	}
	return models, nil
}

func ensureFreeModelFile(apiKey, path string) ([]string, error) {
	// Allow cache TTL to be configured via environment
	cacheTTL := 24 * time.Hour
	if ttlStr := os.Getenv("CACHE_TTL_HOURS"); ttlStr != "" {
		if hours, err := time.ParseDuration(ttlStr + "h"); err == nil {
			cacheTTL = hours
		}
	}

	if stat, err := os.Stat(path); err == nil {
		// Check if cache is still fresh
		if time.Since(stat.ModTime()) < cacheTTL {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			var models []string
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					models = append(models, line)
				}
			}
			return models, nil
		}
		// Cache is stale, will fetch fresh models below
	}

	// Fetch fresh models from API
	models, err := fetchFreeModels(apiKey)
	if err != nil {
		// If fetch fails but we have a cached file (even if stale), use it
		if _, statErr := os.Stat(path); statErr == nil {
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				var cachedModels []string
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if line != "" {
						cachedModels = append(cachedModels, line)
					}
				}
				return cachedModels, nil
			}
		}
		return nil, err
	}

	// Save fresh models to cache
	_ = os.WriteFile(path, []byte(strings.Join(models, "\n")), 0644)
	return models, nil
}
