package main

import "testing"

func TestDeliveryDecision(t *testing.T) {
	tests := []struct {
		name         string
		source, want int
		invalid      bool
	}{
		{"small stays small", 640, 640, false},
		{"large is bounded", 4200, 1600, false},
		{"missing width", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deliveryWidth(tt.source)
			if (err != nil) != tt.invalid || got != tt.want {
				t.Fatalf("deliveryWidth(%d) = %d, %v", tt.source, got, err)
			}
		})
	}
}

func TestAssetKey(t *testing.T) {
	for _, tt := range []struct {
		creator, id, want string
		invalid           bool
	}{
		{"maya", "poster_07", "maya/poster_07.webp", false},
		{"maya", "../poster", "", true},
	} {
		got, err := assetKey(tt.creator, tt.id)
		if got != tt.want || (err != nil) != tt.invalid {
			t.Fatalf("assetKey(%q,%q) = %q, %v", tt.creator, tt.id, got, err)
		}
	}
}
