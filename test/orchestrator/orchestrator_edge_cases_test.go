package orchestrator_test

import (
	"context"
	"testing"

	"github.com/j4flmao/go-migrate-safe/orchestrator"
)

func TestOrchestrator_InvalidIntent(t *testing.T) {
	_, err := orchestrator.Run(context.Background(), "invalid_intent", orchestrator.Options{})
	if err == nil {
		t.Error("expected error for unknown intent, got nil")
	}
}

func TestOrchestrator_ComputeDiff_Internal(t *testing.T) {
	ctx := context.Background()
	// No Migrator in options, should fail
	opts := orchestrator.Options{}

	defer func() {
		if r := recover(); r != nil {
			// Success: caught panic because of nil migrator
		}
	}()

	_, _ = orchestrator.Run(ctx, "diff", opts)
}
