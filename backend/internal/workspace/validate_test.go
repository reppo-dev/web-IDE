package workspace

import "testing"

func TestValidateCodeFileRejectsProse(t *testing.T) {
	// This exact string was previously written into main.go by the swarm.
	cases := []struct {
		name    string
		path    string
		content string
	}{
		{"english prose", "main.go", "User Safety: safe"},
		{"persian sentence", "main.go", "سلام! چطور می‌تونم کمک کنم؟"},
		{"long explanation", "docs/notes.go", "The system should authenticate users before allowing them to access protected resources."},
		{"empty", "main.go", "   \n  "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateCodeFile(tc.path, tc.content); err == nil {
				t.Errorf("expected %q to be rejected as non-code", tc.content)
			}
		})
	}
}

func TestValidateCodeFileRejectsGoWithoutPackage(t *testing.T) {
	if err := ValidateCodeFile("main.go", "func main() { println(\"hi\") }"); err == nil {
		t.Error("expected a Go file without a package declaration to be rejected")
	}
}

func TestValidateCodeFileRejectsInvalidJSON(t *testing.T) {
	if err := ValidateCodeFile("data.json", "just some text"); err == nil {
		t.Error("expected invalid JSON to be rejected")
	}
}

func TestValidateCodeFileAcceptsRealCode(t *testing.T) {
	cases := []struct {
		path    string
		content string
	}{
		{"main.go", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hi\") }"},
		{"app.ts", "export const greet = (name: string) => { return `hello ${name}`; };"},
		{"script.py", "def main():\n    print('hi')\n"},
		{"index.html", "<!DOCTYPE html>\n<html><body><h1>hi</h1></body></html>"},
		{"config.json", "{\"name\": \"nexusai\"}"},
		{"main.rs", "fn main() { println!(\"hi\"); }"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if err := ValidateCodeFile(tc.path, tc.content); err != nil {
				t.Errorf("expected %s to be accepted, got: %v", tc.path, err)
			}
		})
	}
}