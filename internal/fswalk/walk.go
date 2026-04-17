package fswalk

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// FileCandidate represents a file discovered during walking.
type FileCandidate struct {
	Path        string // relative to repo root
	AbsPath     string
	Size        int64
	ModTime     int64 // Unix timestamp
	Language    string
	IsGenerated bool
}

// Walk discovers files under root, applying include/exclude globs and max size filter.
func Walk(root string, include, exclude []string, maxFileSize int64) ([]FileCandidate, error) {
	var candidates []FileCandidate

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == ".atlas" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		// Normalize to forward slashes for glob matching
		relSlash := filepath.ToSlash(rel)

		if !matchesInclude(relSlash, include) {
			return nil
		}
		if matchesExclude(relSlash, exclude) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if maxFileSize > 0 && info.Size() > maxFileSize {
			return nil
		}

		lang := DetectLanguage(path)
		if lang == "" {
			return nil
		}

		generated := isGenerated(path, relSlash)

		candidates = append(candidates, FileCandidate{
			Path:        rel,
			AbsPath:     path,
			Size:        info.Size(),
			ModTime:     info.ModTime().Unix(),
			Language:    lang,
			IsGenerated: generated,
		})

		return nil
	})

	return candidates, err
}

// StatCandidate returns a FileCandidate for a single relative path under
// root, applying the same include/exclude/size/language filters as Walk.
// The boolean return is false if the file is filtered out, unreadable, or
// has no recognized language. Callers use this for incremental mode to
// avoid walking the entire tree when the diff is tiny.
func StatCandidate(root, relPath string, include, exclude []string, maxFileSize int64) (FileCandidate, bool) {
	relSlash := filepath.ToSlash(relPath)

	if !matchesInclude(relSlash, include) {
		return FileCandidate{}, false
	}
	if matchesExclude(relSlash, exclude) {
		return FileCandidate{}, false
	}

	absPath := filepath.Join(root, relPath)
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return FileCandidate{}, false
	}

	if maxFileSize > 0 && info.Size() > maxFileSize {
		return FileCandidate{}, false
	}

	lang := DetectLanguage(absPath)
	if lang == "" {
		return FileCandidate{}, false
	}

	return FileCandidate{
		Path:        relPath,
		AbsPath:     absPath,
		Size:        info.Size(),
		ModTime:     info.ModTime().Unix(),
		Language:    lang,
		IsGenerated: isGenerated(absPath, relSlash),
	}, true
}

// DetectLanguage returns the language for a file based on extension.
func DetectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	default:
		return ""
	}
}

func matchesInclude(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return true // no include filter means include all
	}
	for _, p := range patterns {
		if matched, _ := doublestar.Match(p, path); matched {
			return true
		}
	}
	return false
}

func matchesExclude(path string, patterns []string) bool {
	for _, p := range patterns {
		if matched, _ := doublestar.Match(p, path); matched {
			return true
		}
	}
	return false // no exclude patterns or no match means not excluded
}

func isGenerated(absPath, relPath string) bool {
	if strings.Contains(relPath, "generated") || strings.Contains(relPath, "gen/") {
		return true
	}

	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 256)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}

	firstLine := string(buf[:n])
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	return strings.HasPrefix(firstLine, "// Code generated")
}
