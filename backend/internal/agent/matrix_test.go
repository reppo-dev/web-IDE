package agent

import (
	"context"
	"testing"
)

func TestMatrixEngine_CrossExamination(t *testing.T) {
	mockClient := &mockLLMClient{
		response: `{"agrees": true, "score_given": 92.5, "feedback": "Solid interface design."}`,
	}

	engine := NewMatrixEngine(mockClient)
	sCtx := &SwarmContext{
		ProjectID:  "matrix-test",
		Round:      1,
		UserPrompt: "Build gRPC microservice",
	}

	matrix, err := engine.ExecuteCrossExamination(context.Background(), sCtx)
	if err != nil {
		t.Fatalf("cross examination failed: %v", err)
	}

	// 5 agents inspecting 4 peers each = 20 total pairwise critiques
	if len(matrix.Critiques) != 20 {
		t.Errorf("expected 20 pairwise critiques, got %d", len(matrix.Critiques))
	}

	if matrix.ConsensusPct != 100.0 {
		t.Errorf("expected 100%% consensus, got %.2f", matrix.ConsensusPct)
	}

	if !matrix.Convergence {
		t.Errorf("expected convergence to be true")
	}
}
