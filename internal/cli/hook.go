package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dshills/atlas/internal/output"
	"github.com/spf13/cobra"
)

const hookCommand = "atlas index 2>/dev/null"
const hookTimeout = 15000

// HookCmd creates the `atlas hook` command with install/uninstall subcommands.
func HookCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage Claude Code integration hooks",
	}

	cmd.AddCommand(
		hookInstallCmd(ctx),
		hookUninstallCmd(ctx),
		hookStatusCmd(ctx),
	)

	return cmd
}

func settingsPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".claude", "settings.json")
}

func loadSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid settings JSON: %w", err)
	}
	return raw, nil
}

func saveSettings(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func hasAtlasHook(settings map[string]any) bool {
	hooks, ok := settings["hooks"]
	if !ok {
		return false
	}
	hooksMap, ok := hooks.(map[string]any)
	if !ok {
		return false
	}
	preToolUse, ok := hooksMap["PreToolUse"]
	if !ok {
		return false
	}
	hookList, ok := preToolUse.([]any)
	if !ok {
		return false
	}
	for _, h := range hookList {
		hookObj, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := hookObj["command"].(string); ok && cmd == hookCommand {
			return true
		}
	}
	return false
}

func addAtlasHook(settings map[string]any) {
	hooks, ok := settings["hooks"]
	if !ok {
		hooks = make(map[string]any)
		settings["hooks"] = hooks
	}
	hooksMap, ok := hooks.(map[string]any)
	if !ok {
		hooksMap = make(map[string]any)
		settings["hooks"] = hooksMap
	}

	newHook := map[string]any{
		"matcher": "Bash",
		"command": hookCommand,
		"timeout": float64(hookTimeout),
	}

	preToolUse, ok := hooksMap["PreToolUse"]
	if !ok {
		hooksMap["PreToolUse"] = []any{newHook}
		return
	}
	hookList, ok := preToolUse.([]any)
	if !ok {
		hooksMap["PreToolUse"] = []any{newHook}
		return
	}
	hooksMap["PreToolUse"] = append(hookList, newHook)
}

func removeAtlasHook(settings map[string]any) bool {
	hooks, ok := settings["hooks"]
	if !ok {
		return false
	}
	hooksMap, ok := hooks.(map[string]any)
	if !ok {
		return false
	}
	preToolUse, ok := hooksMap["PreToolUse"]
	if !ok {
		return false
	}
	hookList, ok := preToolUse.([]any)
	if !ok {
		return false
	}

	var filtered []any
	removed := false
	for _, h := range hookList {
		hookObj, ok := h.(map[string]any)
		if !ok {
			filtered = append(filtered, h)
			continue
		}
		if cmd, ok := hookObj["command"].(string); ok && cmd == hookCommand {
			removed = true
			continue
		}
		filtered = append(filtered, h)
	}

	if len(filtered) == 0 {
		delete(hooksMap, "PreToolUse")
	} else {
		hooksMap["PreToolUse"] = filtered
	}

	if len(hooksMap) == 0 {
		delete(settings, "hooks")
	}

	return removed
}

func hookInstallCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install Claude Code PreToolUse hook for automatic re-indexing",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := ctx.RepoRoot()
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			path := settingsPath(repoRoot)
			settings, err := loadSettings(path)
			if err != nil {
				return fmt.Errorf("reading settings: %w", err)
			}

			if hasAtlasHook(settings) {
				f := ctx.Formatter()
				if ctx.OutputMode() == output.ModeText {
					return f.WriteText([]output.KV{
						{Key: "Status", Value: "already installed"},
						{Key: "File", Value: path},
					})
				}
				return f.Write(map[string]string{"status": "already_installed", "file": path})
			}

			addAtlasHook(settings)

			if err := saveSettings(path, settings); err != nil {
				return fmt.Errorf("writing settings: %w", err)
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "Status", Value: "installed"},
					{Key: "File", Value: path},
					{Key: "Hook", Value: "PreToolUse (Bash) -> atlas index"},
				})
			}
			return f.Write(map[string]string{"status": "installed", "file": path})
		},
	}
}

func hookUninstallCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Claude Code Atlas hook",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := ctx.RepoRoot()
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			path := settingsPath(repoRoot)
			settings, err := loadSettings(path)
			if err != nil {
				return fmt.Errorf("reading settings: %w", err)
			}

			if !removeAtlasHook(settings) {
				f := ctx.Formatter()
				if ctx.OutputMode() == output.ModeText {
					return f.WriteText([]output.KV{
						{Key: "Status", Value: "not installed"},
					})
				}
				return f.Write(map[string]string{"status": "not_installed"})
			}

			if err := saveSettings(path, settings); err != nil {
				return fmt.Errorf("writing settings: %w", err)
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "Status", Value: "uninstalled"},
					{Key: "File", Value: path},
				})
			}
			return f.Write(map[string]string{"status": "uninstalled", "file": path})
		},
	}
}

func hookStatusCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check if Claude Code Atlas hook is installed",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := ctx.RepoRoot()
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			path := settingsPath(repoRoot)
			settings, err := loadSettings(path)
			if err != nil {
				return fmt.Errorf("reading settings: %w", err)
			}

			installed := hasAtlasHook(settings)
			status := "not installed"
			if installed {
				status = "installed"
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "Hook", Value: status},
					{Key: "File", Value: path},
				})
			}
			return f.Write(map[string]string{"status": status, "file": path})
		},
	}
}
