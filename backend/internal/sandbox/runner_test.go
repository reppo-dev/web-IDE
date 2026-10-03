package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestSandboxRunner_GoValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nexus-sandbox-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create valid Go file
	validFile := filepath.Join(tempDir, "main.go")
	_ = os.WriteFile(validFile, []byte("package main\n\nfunc main() {}\n"), 0644)

	runner := NewRunner(5 * time.Second)
	res := runner.VerifyCodebase(context.Background(), tempDir, []domain.CodeFile{
		{Path: "main.go", Language: "go"},
	})

	if res.Language != "go" {
		t.Errorf("expected go language, got %s", res.Language)
	}
	if !res.Success {
		t.Errorf("expected successful go verification, got: %s", res.CompilerOutput)
	}
}

func TestSandboxRunner_DartStaticValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nexus-dart-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Valid Dart file with balanced braces
	dartCode := `
import 'package:flutter/material.dart';

void main() {
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return const MaterialApp(home: Scaffold());
  }
}
`
	runner := NewRunner(5 * time.Second)
	res := runner.VerifyCodebase(context.Background(), tempDir, []domain.CodeFile{
		{Path: "lib/main.dart", Language: "dart", Content: dartCode},
	})

	if res.Language != "dart" {
		t.Errorf("expected dart language, got %s", res.Language)
	}
	if !res.Success {
		t.Errorf("expected successful dart validation, got: %s", res.CompilerOutput)
	}
}

func TestSandboxRunner_DartUnbalancedBraces(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nexus-dart-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	brokenDart := "void main() { runApp(null);" // Missing closing brace

	runner := NewRunner(5 * time.Second)
	res := runner.verifyStaticSyntax(tempDir, []domain.CodeFile{
		{Path: "lib/broken.dart", Language: "dart", Content: brokenDart},
	}, "dart")

	if res.Success {
		t.Errorf("expected failure for unbalanced braces, got success")
	}
	if len(res.Diagnostics) == 0 {
		t.Errorf("expected diagnostic issue for unbalanced braces")
	}
}

func TestSandboxRunner_DiagnosticParsing(t *testing.T) {
	sampleOutput := `
main.go:14:2: undefined: fmt.Println
main.go:20:5: warning: variable declared but not used
`
	errors, diags := parseCompilerErrors(sampleOutput)
	if len(errors) < 2 {
		t.Errorf("expected at least 2 errors, got %d", len(errors))
	}
	if len(diags) < 2 {
		t.Errorf("expected 2 diagnostics, got %d", len(diags))
	}
	if diags[0].Line != 14 || diags[0].Column != 2 {
		t.Errorf("expected line 14 col 2, got line %d col %d", diags[0].Line, diags[0].Column)
	}
	if diags[1].Severity != "warning" {
		t.Errorf("expected warning severity, got %s", diags[1].Severity)
	}
}

func TestSandboxRunner_LanguageDetection(t *testing.T) {
	runner := NewRunner(5 * time.Second)

	files := []domain.CodeFile{
		{Path: "lib/main.dart"},
		{Path: "lib/widgets/card.dart"},
		{Path: "pubspec.yaml"},
	}
	detected := runner.detectLanguage("", files)
	if detected != "dart" {
		t.Errorf("expected dart detected, got %s", detected)
	}

	tsFiles := []domain.CodeFile{
		{Path: "src/App.tsx"},
		{Path: "src/index.ts"},
	}
	detectedTS := runner.detectLanguage("", tsFiles)
	if detectedTS != "typescript" {
		t.Errorf("expected typescript detected, got %s", detectedTS)
	}
}
