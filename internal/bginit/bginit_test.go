package bginit

import "testing"

func TestBackgroundIsDark(t *testing.T) {
	tests := []struct {
		name      string
		colorFGBG string
		want      bool
	}{
		{"unset defaults dark", "", true},
		{"no separator defaults dark", "15", true},
		{"unparsable background defaults dark", "15;default", true},
		{"black background", "15;0", true},
		{"blue background", "15;4", true},
		{"bright black background diverges from termenv, stays dark", "15;8", true},
		{"negative background defaults dark", "15;-1", true},
		{"out-of-range background defaults dark", "0;99", true},
		{"light grey background", "0;7", false},
		{"white background", "0;15", false},
		{"bright yellow background", "0;11", false},
		{"three fields uses last", "15;default;0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backgroundIsDark(tt.colorFGBG); got != tt.want {
				t.Errorf("backgroundIsDark(%q) = %v, want %v", tt.colorFGBG, got, tt.want)
			}
		})
	}
}
