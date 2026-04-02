package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dshills/atlas/internal/output"
	"github.com/spf13/cobra"
)

const hookCommand = "atlas index 2>/dev/null"
const hookTimeout = 15000

const claudeMDSection = `## Atlas Index

This repository has an Atlas index for structural and semantic code queries.
Use atlas commands with --agent for compact JSON instead of reading source files:

- ` + "`atlas find symbol <name> --agent`" + ` — find symbol definitions
- ` + "`atlas who-calls <symbol> --agent`" + ` — find callers
- ` + "`atlas calls <symbol> --agent`" + ` — find callees
- ` + "`atlas implementations <interface> --agent`" + ` — find implementations
- ` + "`atlas tests-for <symbol> --agent`" + ` — find related tests
- ` + "`atlas summarize file <path> --agent`" + ` — get file summary
- ` + "`atlas list routes --agent`" + ` — list HTTP routes
- ` + "`atlas export graph --agent`" + ` — get full dependency graph

The index auto-updates via a PreToolUse hook. To manually re-index: ` + "`atlas index`" + `
`

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

// matcherContainsAtlasHook checks if a matcher entry's hooks array contains the atlas command.
func matcherContainsAtlasHook(entry map[string]any) bool {
	hooksArr, ok := entry["hooks"].([]any)
	if !ok {
		return false
	}
	for _, h := range hooksArr {
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
	matcherList, ok := preToolUse.([]any)
	if !ok {
		return false
	}
	for _, entry := range matcherList {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if matcherContainsAtlasHook(entryMap) {
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

	newEntry := map[string]any{
		"matcher": "Bash",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCommand,
				"timeout": float64(hookTimeout),
			},
		},
	}

	preToolUse, ok := hooksMap["PreToolUse"]
	if !ok {
		hooksMap["PreToolUse"] = []any{newEntry}
		return
	}
	matcherList, ok := preToolUse.([]any)
	if !ok {
		hooksMap["PreToolUse"] = []any{newEntry}
		return
	}
	hooksMap["PreToolUse"] = append(matcherList, newEntry)
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
	matcherList, ok := preToolUse.([]any)
	if !ok {
		return false
	}

	var filtered []any
	removed := false
	for _, entry := range matcherList {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		if matcherContainsAtlasHook(entryMap) {
			removed = true
			continue
		}
		filtered = append(filtered, entry)
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

func writeClaudeMD(repoRoot string) (string, error) {
	mdPath := filepath.Join(repoRoot, "CLAUDE.md")

	existing, err := os.ReadFile(mdPath)
	if err != nil && !os.IsNotExist(err) {
		return mdPath, fmt.Errorf("reading CLAUDE.md: %w", err)
	}

	if strings.Contains(string(existing), "## Atlas Index") {
		return mdPath, nil // already has Atlas section
	}

	f, err := os.OpenFile(mdPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return mdPath, fmt.Errorf("opening CLAUDE.md: %w", err)
	}
	defer func() { _ = f.Close() }()

	content := claudeMDSection
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n\n") {
		if strings.HasSuffix(string(existing), "\n") {
			content = "\n" + content
		} else {
			content = "\n\n" + content
		}
	}

	if _, err := f.WriteString(content); err != nil {
		return mdPath, fmt.Errorf("writing CLAUDE.md: %w", err)
	}

	return mdPath, nil
}

func hookInstallCmd(ctx *CLIContext) *cobra.Command {
	var flagClaudeMD bool

	cmd := &cobra.Command{
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

			hookStatus := "installed"
			if hasAtlasHook(settings) {
				hookStatus = "already installed"
			} else {
				addAtlasHook(settings)
				if err := saveSettings(path, settings); err != nil {
					return fmt.Errorf("writing settings: %w", err)
				}
			}

			kvs := []output.KV{
				{Key: "Hook", Value: hookStatus},
				{Key: "Settings", Value: path},
			}

			if flagClaudeMD {
				mdPath, err := writeClaudeMD(repoRoot)
				if err != nil {
					return err
				}
				mdStatus := "written"
				existing, _ := os.ReadFile(mdPath)
				if strings.Contains(string(existing), "## Atlas Index") {
					mdStatus = "already present"
				}
				kvs = append(kvs, output.KV{Key: "CLAUDE.md", Value: mdStatus})
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				return f.WriteText(kvs)
			}
			result := make(map[string]string)
			for _, kv := range kvs {
				result[kv.Key] = kv.Value
			}
			return f.Write(result)
		},
	}
	cmd.Flags().BoolVar(&flagClaudeMD, "claude-md", false, "Also write Atlas instructions to CLAUDE.md")
	return cmd
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
