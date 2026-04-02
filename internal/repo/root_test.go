package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRootWithFlag(t *testing.T) {
	dir := t.TempDir()
	root, err := FindRoot(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	// Should resolve to the temp dir
	expected, _ := filepath.EvalSymlinks(dir)
	if root != expected {
		t.Errorf("expected %s, got %s", expected, root)
	}
}

func TestFindRootWithConfig(t *testing.T) {
	dir := t.TempDir()
	root, err := FindRoot("", dir)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := filepath.EvalSymlinks(dir)
	if root != expected {
		t.Errorf("expected %s, got %s", expected, root)
	}
}

func TestFindRootGitDetection(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// findGitRoot from subdirectory should find parent .git
	root, err := findGitRoot(subDir)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := filepath.EvalSymlinks(dir)
	if root != expected {
		t.Errorf("expected %s, got %s", expected, root)
	}
}

func TestFindRootNoGit(t *testing.T) {
	dir := t.TempDir()
	_, err := findGitRoot(dir)
	if err == nil {
		t.Error("expected error when no .git found")
	}
}
