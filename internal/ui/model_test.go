package ui

import (
	"testing"
	"time"
)

func TestInitSpeed(t *testing.T) {
	start := time.Now()
	_, err := InitModelWithConfig(Config{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("InitModelWithConfig failed: %v", err)
	}

	t.Logf("InitModelWithConfig took: %v", elapsed)

	if elapsed > 200*time.Millisecond {
		t.Errorf("InitModelWithConfig took %v, expected under 200ms", elapsed)
	}
}
