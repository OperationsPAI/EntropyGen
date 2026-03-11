package controller

import (
	"testing"
)

func TestExpandMemoryLimit(t *testing.T) {
	tests := []struct {
		name       string
		currentMi  int64
		initialMi  int64
		wantNewMi  int64
		wantExpand bool
	}{
		{
			name:       "512Mi to 768Mi (1.5x, already 128Mi aligned)",
			currentMi:  512,
			initialMi:  512,
			wantNewMi:  768,
			wantExpand: true,
		},
		{
			name:       "300Mi rounds up to 512Mi (128Mi boundary)",
			currentMi:  300,
			initialMi:  256,
			wantNewMi:  512,
			wantExpand: true,
		},
		{
			name:       "already at max (2048Mi cap)",
			currentMi:  2048,
			initialMi:  512,
			wantNewMi:  2048,
			wantExpand: false,
		},
		{
			name:      "small initial, hits 2x initial cap before 2Gi",
			currentMi: 900,
			initialMi: 512,
			// max = max(1024, 2048) = 2048; 900*1.5=1350 -> round to 1408; 1408 < 2048
			wantNewMi:  1408,
			wantExpand: true,
		},
		{
			name:      "at max(2x initial) when initial is large",
			currentMi: 4096,
			initialMi: 2048,
			// max = max(4096, 2048) = 4096; currentMi == max -> no expand
			wantNewMi:  4096,
			wantExpand: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, expand := expandMemoryLimit(tt.currentMi, tt.initialMi)
			if got != tt.wantNewMi {
				t.Errorf("expandMemoryLimit(%d, %d) = %d, want %d", tt.currentMi, tt.initialMi, got, tt.wantNewMi)
			}
			if expand != tt.wantExpand {
				t.Errorf("expandMemoryLimit(%d, %d) expand = %v, want %v", tt.currentMi, tt.initialMi, expand, tt.wantExpand)
			}
		})
	}
}
