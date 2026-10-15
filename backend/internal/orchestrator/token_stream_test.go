package orchestrator

import (
	"testing"
	"time"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestStreamTokenBroadcastsEventTokenChunk(t *testing.T) {
	orch := &Orchestrator{
		subscribers: make([]chan SwarmEvent, 0),
	}

	ch := orch.Subscribe()
	defer orch.Unsubscribe(ch)

	orch.streamToken("proj-1", 1, "T-001", domain.RoleCoder, "coder-2", "import 'package:flutter/material.dart';")

	select {
	case evt := <-ch:
		if evt.Type != EventTokenChunk {
			t.Fatalf("expected EventTokenChunk, got %s", evt.Type)
		}
		if evt.TaskID != "T-001" {
			t.Errorf("expected TaskID 'T-001', got %s", evt.TaskID)
		}
		if evt.StreamAgent != "coder-2" {
			t.Errorf("expected StreamAgent 'coder-2', got %s", evt.StreamAgent)
		}
		if evt.Agent != domain.RoleCoder {
			t.Errorf("expected Agent RoleCoder, got %s", evt.Agent)
		}
		if evt.Chunk != "import 'package:flutter/material.dart';" {
			t.Errorf("unexpected chunk: %s", evt.Chunk)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for token chunk event")
	}
}

func TestSwarmContext_StreamTokenDispatchesCorrectly(t *testing.T) {
	var capturedRole domain.AgentRole
	var capturedAs string
	var capturedTaskID string
	var capturedChunk string

	sCtx := &agent.SwarmContext{
		ActiveTask: &domain.ProjectTask{
			ID:    "T-042",
			Title: "Build Auth Provider",
		},
		TokenStreamer: func(role domain.AgentRole, as, taskID, chunk string) {
			capturedRole = role
			capturedAs = as
			capturedTaskID = taskID
			capturedChunk = chunk
		},
	}

	sCtx.StreamToken(domain.RoleCoder, "coder-1", "class AuthProvider {}")

	if capturedRole != domain.RoleCoder {
		t.Errorf("expected RoleCoder, got %v", capturedRole)
	}
	if capturedAs != "coder-1" {
		t.Errorf("expected 'coder-1', got %s", capturedAs)
	}
	if capturedTaskID != "T-042" {
		t.Errorf("expected 'T-042', got %s", capturedTaskID)
	}
	if capturedChunk != "class AuthProvider {}" {
		t.Errorf("expected chunk 'class AuthProvider {}', got %s", capturedChunk)
	}
}
