package loop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FestivalLoopConfig holds festival-level LOOP settings from fest.yaml.
type FestivalLoopConfig struct {
	MaxTotalIterations int `yaml:"max_total_iterations"`
}

// DefaultFestivalLoopConfig returns default festival loop settings.
func DefaultFestivalLoopConfig() *FestivalLoopConfig {
	return &FestivalLoopConfig{
		MaxTotalIterations: 10,
	}
}

// LoadFestivalLoopConfig loads LOOP settings from fest.yaml.
func LoadFestivalLoopConfig(festivalPath string) (*FestivalLoopConfig, error) {
	festYAML := filepath.Join(festivalPath, "fest.yaml")

	data, err := os.ReadFile(festYAML)
	if err != nil {
		return DefaultFestivalLoopConfig(), nil
	}

	var festConfig struct {
		Loop *FestivalLoopConfig `yaml:"loop,omitempty"`
	}

	if err := yaml.Unmarshal(data, &festConfig); err != nil {
		return nil, fmt.Errorf("parse fest.yaml: %w", err)
	}

	if festConfig.Loop == nil {
		return DefaultFestivalLoopConfig(), nil
	}

	if festConfig.Loop.MaxTotalIterations <= 0 {
		festConfig.Loop.MaxTotalIterations = 10
	}

	return festConfig.Loop, nil
}

// TotalIterations returns the total retry iteration count across all loops in a festival.
func TotalIterations(festivalPath string) (int, error) {
	total := 0

	err := filepath.Walk(festivalPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if info.IsDir() || filepath.Base(path) != "loop_state.jsonl" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var event StateEvent
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				continue
			}

			if event.Type == "loop_retry" {
				total++
			}
		}

		return nil
	})

	return total, err
}
