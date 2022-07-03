package walkman

import (
	"testing"
)

func TestSliceContains(t *testing.T) {
	l := []string{"golang", "Downloads", "github.com"}

	for _, d := range l {
		if !slice_contains(l, d) {
			t.Errorf("slice %+v should contain %s", l, d)
		}
	}

	// Non-existent dir
	if slice_contains(l, "go-prog") {
		t.Errorf("Expected slice_contains to return false for dir: go-prog")
	}
}
