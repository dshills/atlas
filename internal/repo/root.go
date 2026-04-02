package repo

import (
	"os"
	"path/filepath"
)

// FindRoot determines the repository root following the precedence in Section 5.2:
// 1. Explicit --repo flag
// 2. Configured repo_root
// 3. Git root (walk up for .git/)
// 4. Current working directory
func FindRoot(flagRepo string, configRoot string) (string, error) {
	if flagRepo != "" {
		return canonicalize(flagRepo)
	}
	if configRoot != "" {
		return canonicalize(configRoot)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	gitRoot, err := findGitRoot(cwd)
	if err == nil {
		return gitRoot, nil
	}

	return canonicalize(cwd)
}

func findGitRoot(start string) (string, error) {
	dir, err := canonicalize(start)
	if err != nil {
		return "", err
	}

	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func canonicalize(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}
