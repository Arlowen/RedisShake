package ids

import (
	"regexp"
	"testing"
)

func TestNew(t *testing.T) {
	first, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	second, err := New()
	if err != nil {
		t.Fatalf("New() second error = %v", err)
	}
	if first == second {
		t.Fatalf("New() generated duplicate IDs: %q", first)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(first) {
		t.Fatalf("New() = %q, want 32 lowercase hex characters", first)
	}
}
