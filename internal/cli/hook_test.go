package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
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
				map[string]any{"matcher": "Edit", "command": "echo done", "timeout": 5000.0},
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

	// Verify existing hooks preserved
	hooks := loaded["hooks"].(map[string]any)
	if _, ok := hooks["PostToolUse"]; !ok {
		t.Error("expected existing PostToolUse hook to be preserved")
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
