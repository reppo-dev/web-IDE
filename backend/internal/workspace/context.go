package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ContextEngine assembles the real project state that an agent needs before it
// writes code. Without this, agents invent a codebase from the prompt alone
// and produce code that contradicts what is already on disk.
type ContextEngine struct {
	root string
	// maxFileBytes caps each file included in the prompt. Large generated or
	// vendored files would otherwise crowd out the code that matters.
	maxFileBytes int
	// maxTotalBytes caps the whole bundle so a big repository cannot produce a
	// prompt larger than the model's context window.
	maxTotalBytes int
}

// NewContextEngine builds a context engine rooted at the project directory.
func NewContextEngine(root string) *ContextEngine {
	return &ContextEngine{
		root:          root,
		maxFileBytes:  16 * 1024,
		maxTotalBytes: 120 * 1024,
	}
}

// skipDirs are directories that never carry project source worth reading.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true,
	"vendor": true, "__pycache__": true, ".venv": true, "target": true,
	".idea": true, ".vscode": true, "coverage": true,
}

// IsViewableFile reports whether the editor may open a file. Binary blobs,
// archives and build outputs would only render as noise, so they are refused
// rather than sent to the client.
func IsViewableFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".ico", ".webp",
		".zip", ".gz", ".tar", ".rar", ".7z", ".exe", ".dll", ".so", ".dylib",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".class", ".jar", ".war", ".o", ".a", ".lib", ".bin",
		".sqlite", ".sqlite3", ".db", ".woff", ".woff2", ".ttf", ".eot":
		return false
	}
	return true
}

// sourceExts are the file types worth sending to an agent.
var sourceExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".dart": true, ".py": true, ".rs": true, ".java": true, ".kt": true,
	".swift": true, ".rb": true, ".php": true, ".c": true, ".h": true,
	".cpp": true, ".hpp": true, ".cs": true, ".sql": true, ".sh": true,
	".yaml": true, ".yml": true, ".json": true, ".toml": true, ".md": true,
	".html": true, ".css": true, ".scss": true, ".vue": true, ".svelte": true,
	".xml": true, ".gradle": true, ".proto": true,
}

// ProjectFile is one file discovered on disk, relative to the project root.
type ProjectFile struct {
	RelPath string
	Size    int
	ModTime int64
}

// ListSourceFiles walks the project and returns its source files, sorted so
// results are stable between runs.
func (e *ContextEngine) ListSourceFiles() ([]ProjectFile, error) {
	if e.root == "" {
		return nil, fmt.Errorf("no workspace path configured")
	}
	var found []ProjectFile

	err := filepath.Walk(e.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// An unreadable path should not abort the whole walk.
			return nil
		}
		if info.IsDir() {
			// Dot-directories are skipped too, but never the walk root itself.
			if skipDirs[info.Name()] || (strings.HasPrefix(info.Name(), ".") && info.Name() != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !sourceExts[ext] {
			return nil
		}
		rel, relErr := filepath.Rel(e.root, path)
		if relErr != nil {
			return nil
		}
		if IsSecretPath(rel) || IsSecretPath(path) {
			return nil
		}
		found = append(found, ProjectFile{
			RelPath: filepath.ToSlash(rel),
			Size:    int(info.Size()),
			ModTime: info.ModTime().Unix(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(found, func(i, j int) bool { return found[i].RelPath < found[j].RelPath })
	return found, nil
}

// relevantKeywords pulls distinctive words out of a user prompt so file
// selection can rank candidates. Common English and Persian stop words are
// dropped so that "the" or "یک" never decides relevance.
func relevantKeywords(prompt string) []string {
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "add": true,
		"create": true, "build": true, "make": true, "new": true, "code": true,
		"file": true, "files": true, "project": true, "system": true, "using": true,
		"that": true, "this": true, "should": true, "from": true, "into": true,
		"یک": true, "و": true, "در": true, "به": true, "از": true, "با": true,
		"را": true, "که": true, "برای": true, "است": true, "این": true,
		"می": true, "های": true, "روی": true, "کن": true, "بده": true,
	}

	fields := strings.FieldsFunc(strings.ToLower(prompt), func(r rune) bool {
		return !(r == '_' || r == '-' || r == '.' || r == '/' || r == '\\' ||
			(r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			(r >= 0x0600 && r <= 0x06FF))
	})

	seen := make(map[string]bool)
	var out []string
	for _, f := range fields {
		if len(f) < 3 || stop[f] || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// rankFiles scores each file against the prompt keywords. A path match is the
// strongest signal, then matches inside the file's own content.
func rankFiles(e *ContextEngine, files []ProjectFile, keywords []string) map[string]int {
	scores := make(map[string]int)
	if len(keywords) == 0 {
		for _, f := range files {
			scores[f.RelPath] = 1
		}
		return scores
	}
	for _, f := range files {
		lower := strings.ToLower(f.RelPath)
		score := 0
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				score += 10
			}
		}
		// Content matches are a weaker signal than a path match, but they are
		// what lets a request naming a concept ("login") find the file that
		// implements it even when the name does not repeat the keyword.
		if score == 0 {
			if full, err := SafeReadPath(e.root, f.RelPath); err == nil {
				if data, readErr := os.ReadFile(full); readErr == nil {
					body := strings.ToLower(string(data))
					for _, kw := range keywords {
						if strings.Contains(body, kw) {
							score++
						}
					}
				}
			}
		}
		scores[f.RelPath] = score
	}
	return scores
}

// SelectRelevantFiles picks the files most likely to matter for a request.
func (e *ContextEngine) SelectRelevantFiles(prompt string, limit int) ([]ProjectFile, error) {
	files, err := e.ListSourceFiles()
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	keywords := relevantKeywords(prompt)
	scores := rankFiles(e, files, keywords)

	// When nothing matched the keywords, fall back to recency: the most
	// recently touched files are what the user is most likely working on.
	if maxScore := maxInt(scores); maxScore == 0 {
		sort.Slice(files, func(i, j int) bool { return files[i].ModTime > files[j].ModTime })
	} else {
		sort.SliceStable(files, func(i, j int) bool {
			return scores[files[i].RelPath] > scores[files[j].RelPath]
		})
	}

	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	return files, nil
}

// BuildCodeContext reads the selected files and renders them as a prompt block.
// Files are included whole until the byte budget runs out; anything skipped is
// listed by name so the model still knows what it has not seen.
func (e *ContextEngine) BuildCodeContext(prompt string, maxFiles int) (string, error) {
	files, err := e.SelectRelevantFiles(prompt, maxFiles)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}

	var b strings.Builder
	remaining := e.maxTotalBytes
	omitted := 0

	for _, f := range files {
		abs, safeErr := SafeReadPath(e.root, f.RelPath)
		if safeErr != nil {
			omitted++
			continue
		}
		data, readErr := os.ReadFile(abs)
		if readErr != nil {
			omitted++
			continue
		}
		if len(data) > e.maxFileBytes {
			data = data[:e.maxFileBytes]
		}
		if len(data) > remaining {
			data = data[:remaining]
		}

		truncated := len(data) < int(f.Size)

		remaining -= len(data)
		b.WriteString(fmt.Sprintf("\n--- FILE: %s ---\n", f.RelPath))
		b.Write(data)
		if truncated {
			b.WriteString("\n... [truncated]")
		}
		b.WriteString("\n")
	}

	if omitted > 0 {
		b.WriteString(fmt.Sprintf("\n[%d more project files not shown]\n", omitted))
	}
	return b.String(), nil
}

// BuildStructure renders a cheap map of the whole project so an agent knows
// what exists without paying to read every file.
func (e *ContextEngine) BuildStructure() (string, error) {
	files, err := e.ListSourceFiles()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("Existing project files:\n")
	for _, f := range files {
		fmt.Fprintf(&b, "- %s (%d bytes)\n", f.RelPath, f.Size)
	}
	return b.String(), nil
}

func maxInt(m map[string]int) int {
	max := 0
	for _, v := range m {
		if v > max {
			max = v
		}
	}
	return max
}
