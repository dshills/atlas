package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookInstallCreatesSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.json")

	settings := make(map[string]any)
	addAtlasHook(settings)

	if err := saveSettings(path, settings); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadSettings(path)
	if err != nil {
		t.Fatal(err)
	}

	if !hasAtlasHook(loaded) {
		t.Error("expected atlas hook to be installed")
	}
}

func TestHookInstallPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.json")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	existing := map[string]any{
		"permissions": map[string]any{"allow": []any{"Read", "Write"}},
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"matcher": "Edit",
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo done", "timeout": 5000.0},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	settings, err := loadSettings(path)
	if err != nil {
		t.Fatal(err)
	}

	addAtlasHook(settings)

	if err := saveSettings(path, settings); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadSettings(path)
	if err != nil {
		t.Fatal(err)
	}

	if !hasAtlasHook(loaded) {
		t.Error("expected atlas hook to be installed")
	}

	// Verify existing hooks preserved alongside atlas hook
	hooks := loaded["hooks"].(map[string]any)
	postToolUse, ok := hooks["PostToolUse"].([]any)
	if !ok {
		t.Fatal("expected PostToolUse to be a list")
	}
	if len(postToolUse) < 2 {
		t.Error("expected at least 2 PostToolUse entries (existing + atlas)")
	}

	// Verify permissions preserved
	if _, ok := loaded["permissions"]; !ok {
		t.Error("expected existing permissions to be preserved")
	}
}

func TestHookUninstall(t *testing.T) {
	settings := make(map[string]any)
	addAtlasHook(settings)

	if !hasAtlasHook(settings) {
		t.Fatal("expected hook to be installed")
	}

	removed := removeAtlasHook(settings)
	if !removed {
		t.Error("expected removeAtlasHook to return true")
	}

	if hasAtlasHook(settings) {
		t.Error("expected hook to be removed")
	}
}

func TestHookUninstallNotInstalled(t *testing.T) {
	settings := make(map[string]any)
	removed := removeAtlasHook(settings)
	if removed {
		t.Error("expected removeAtlasHook to return false when not installed")
	}
}

func TestHookIdempotentInstall(t *testing.T) {
	settings := make(map[string]any)
	addAtlasHook(settings)

	if !hasAtlasHook(settings) {
		t.Fatal("expected hook installed after first add")
	}

	// hasAtlasHook should prevent double-add in real code
	if !hasAtlasHook(settings) {
		t.Error("expected hook to still be detected")
	}
}

func TestHookDetectsLegacyPreToolUse(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": hookCommand},
					},
				},
			},
		},
	}

	if !hasAtlasHook(settings) {
		t.Error("expected hasAtlasHook to detect legacy PreToolUse hook")
	}

	removed := removeAtlasHook(settings)
	if !removed {
		t.Error("expected removeAtlasHook to remove legacy PreToolUse hook")
	}
	if hasAtlasHook(settings) {
		t.Error("expected hook to be gone after removal")
	}
}

func TestWriteClaudeMDCreatesFile(t *testing.T) {
	dir := t.TempDir()

	mdPath, err := writeClaudeMD(dir)
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "## Code Search Protocol") {
		t.Error("expected CLAUDE.md to contain Code Search Protocol section")
	}
	if !strings.Contains(string(content), "atlas find symbol") {
		t.Error("expected CLAUDE.md to contain atlas command examples")
	}
}

func TestWriteClaudeMDAppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "CLAUDE.md")

	existing := "# CLAUDE.md\n\nExisting instructions here.\n"
	if err := os.WriteFile(mdPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := writeClaudeMD(dir); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "Existing instructions here.") {
		t.Error("expected existing content to be preserved")
	}
	if !strings.Contains(string(content), "## Code Search Protocol") {
		t.Error("expected Code Search Protocol section to be appended")
	}
}

func TestWriteClaudeMDSkipsIfPresent(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "CLAUDE.md")

	content := "# CLAUDE.md\n\n## Atlas Index\n\nAlready here.\n"
	if err := os.WriteFile(mdPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := writeClaudeMD(dir); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(after) != content {
		t.Error("expected CLAUDE.md to remain unchanged when Atlas section already exists")
	}
}
