package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestMarkdownStyle(t *testing.T) {
	original := lipgloss.HasDarkBackground()
	t.Cleanup(func() { lipgloss.SetHasDarkBackground(original) })

	tests := []struct {
		name string
		dark bool
		want string
	}{
		{name: "dark background", dark: true, want: "dark"},
		{name: "light background", dark: false, want: "light"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lipgloss.SetHasDarkBackground(tt.dark)
			if got := MarkdownStyle(); got != tt.want {
				t.Errorf("MarkdownStyle() = %q, want %q", got, tt.want)
			}
		})
	}
}
