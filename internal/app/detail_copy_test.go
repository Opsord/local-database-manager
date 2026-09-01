package app

import (
	"testing"
	"time"
)

func TestTokenizeValue(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"postgres", []string{"postgres"}},
		{"POSTGRES (DOCKER)", []string{"POSTGRES", "DOCKER"}},
		{"124.5MiB (Limit: 512MiB)", []string{"124.5MiB", "Limit:", "512MiB"}},
		{"postgresql://u:p@localhost:5432/db", []string{"postgresql://u:p@localhost:5432/db"}},
		{"", nil},
	}
	for _, tt := range tests {
		toks := tokenizeValue(tt.in)
		var got []string
		for _, tok := range toks {
			got = append(got, tok.Text)
		}
		if len(got) != len(tt.want) {
			t.Fatalf("%q: got %v want %v", tt.in, got, tt.want)
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Fatalf("%q[%d]: got %q want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

func TestHitTest(t *testing.T) {
	hits := []copyHit{
		{X: 10, Y: 5, W: 8, H: 1, Text: "postgres"},
		{X: 20, Y: 5, W: 4, H: 1, Text: "5432"},
	}
	if text, ok := hitTest(hits, 12, 5); !ok || text != "postgres" {
		t.Fatalf("got %q %v", text, ok)
	}
	if text, ok := hitTest(hits, 21, 5); !ok || text != "5432" {
		t.Fatalf("got %q %v", text, ok)
	}
	if _, ok := hitTest(hits, 0, 0); ok {
		t.Fatal("expected miss")
	}
}

func TestClickTrackerDoubleClick(t *testing.T) {
	var tr clickTracker
	t0 := time.Unix(0, 0)
	if tr.register(10, 5, t0) {
		t.Fatal("first click must not be double")
	}
	if !tr.register(10, 5, t0.Add(300*time.Millisecond)) {
		t.Fatal("expected double within window same cell")
	}
	if tr.register(10, 5, t0.Add(400*time.Millisecond)) {
		t.Fatal("after double, next click is a new first click")
	}
	if tr.register(40, 5, t0.Add(500*time.Millisecond)) {
		t.Fatal("far cell must reset, not double")
	}
	if tr.register(10, 5, t0.Add(2*time.Second)) {
		t.Fatal("outside 500ms must not double")
	}
}
