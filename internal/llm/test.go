package llm

import (
	"context"
	"fmt"
	"time"
)

// TestConnection verifies a Config by making a minimal Complete() call.
// Uses a 10s timeout and caps max_tokens to 1 to keep the probe cheap.
func TestConnection(cfg Config) error {
	probe := cfg
	if probe.MaxTokens == 0 || probe.MaxTokens > 16 {
		probe.MaxTokens = 16
	}
	client, err := New(probe)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := client.Complete(ctx, "", []Message{{Role: "user", Content: "ping"}}); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	return nil
}
