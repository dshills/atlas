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

const claudeMDSection = `## Code Search Protocol

Use this decision tree — in order — before reading any source file:

### Structural questions → atlas (always first)
- "Where is X defined?" → ` + "`atlas find symbol X --agent`" + `
- "What calls X?" → ` + "`atlas who-calls X --agent`" + `
- "What does X call?" → ` + "`atlas calls X --agent`" + `
- "What implements interface X?" → ` + "`atlas implementations X --agent`" + `
- "Which tests cover X?" → ` + "`atlas tests-for X --agent`" + `
- "What routes exist?" → ` + "`atlas list routes --agent`" + `
- "What changed?" → ` + "`atlas index --since HEAD~1 && atlas stale --agent`" + `

### Before reading a large file → summarize first
` + "`atlas summarize file <path> --agent`" + `
Only read the file directly if the summary is insufficient.

### Content/pattern questions → rg
- Error strings, log messages, string literals
- Comments, TODOs, inline notes
- Non-Go/TS files (YAML, SQL, Markdown)
- Unstaged files not yet indexed

### Never read source files to answer these questions
If atlas has the answer, do not use Read or Bash(cat).
Atlas is authoritative — its index is maintained by a PostToolUse hook on Write/Edit/MultiEdit.
`

type hookTarget struct {
	name         string
	displayName  string
	settingsPath func(string) string
}

var (
	claudeHookTarget = hookTarget{
		name:         "claude",
		displayName:  "Claude Code",
		settingsPath: settingsPath,
	}
	codexHookTarget = hookTarget{
		name:         "codex",
		displayName:  "Codex",
		settingsPath: codexSettingsPath,
	}
)

// HookCmd creates the `atlas hook` command with install/uninstall subcommands.
func HookCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage agent integration hooks",
		Long: `Manage agent integration hooks for automatic re-indexing.

Subcommands:
  install     Install PostToolUse hook (re-indexes after Write/Edit/MultiEdit)
  uninstall   Remove the Atlas hook
  status      Check if the hook is installed

By default, hook commands target Claude Code for backward compatibility.
Use --codex to target Codex instead.

The install command accepts --claude-md to also write Code Search Protocol
instructions to your project's CLAUDE.md, or --codex-md to install the Codex
hook and write instructions to AGENTS.md.`,
		Example: `  atlas hook install              # install the auto-reindex hook
  atlas hook install --claude-md  # install hook + write CLAUDE.md instructions
  atlas hook install --codex-md   # install Codex hook + write AGENTS.md instructions
  atlas hook status --codex       # check if Codex hook is installed
  atlas hook status               # check if hook is installed
  atlas hook uninstall            # remove the hook`,
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

func codexSettingsPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".codex", "hooks.json")
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
	// Check both PostToolUse (current) and PreToolUse (legacy).
	for _, event := range []string{"PostToolUse", "PreToolUse"} {
		eventHooks, ok := hooksMap[event]
		if !ok {
			continue
		}
		matcherList, ok := eventHooks.([]any)
		if !ok {
			continue
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
		"matcher": "Write|Edit|MultiEdit",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCommand,
				"timeout": float64(hookTimeout),
			},
		},
	}

	postToolUse, ok := hooksMap["PostToolUse"]
	if !ok {
		hooksMap["PostToolUse"] = []any{newEntry}
		return
	}
	matcherList, ok := postToolUse.([]any)
	if !ok {
		hooksMap["PostToolUse"] = []any{newEntry}
		return
	}
	hooksMap["PostToolUse"] = append(matcherList, newEntry)
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

	removed := false
	// Remove from both PostToolUse (current) and PreToolUse (legacy).
	for _, event := range []string{"PostToolUse", "PreToolUse"} {
		eventHooks, ok := hooksMap[event]
		if !ok {
			continue
		}
		matcherList, ok := eventHooks.([]any)
		if !ok {
			continue
		}

		var filtered []any
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
			delete(hooksMap, event)
		} else {
			hooksMap[event] = filtered
		}
	}

	if len(hooksMap) == 0 {
		delete(settings, "hooks")
	}

	return removed
}

func hasAtlasInstructions(content []byte) bool {
	return strings.Contains(string(content), "## Code Search Protocol") || strings.Contains(string(content), "## Atlas Index")
}

func writeClaudeMD(repoRoot string) (string, error) {
	return writeInstructionsMD(repoRoot, "CLAUDE.md")
}

func writeAgentsMD(repoRoot string) (string, error) {
	return writeInstructionsMD(repoRoot, "AGENTS.md")
}

func writeInstructionsMD(repoRoot, fileName string) (string, error) {
	mdPath := filepath.Join(repoRoot, fileName)

	existing, err := os.ReadFile(mdPath)
	if err != nil && !os.IsNotExist(err) {
		return mdPath, fmt.Errorf("reading %s: %w", fileName, err)
	}

	if hasAtlasInstructions(existing) {
		return mdPath, nil // already has Atlas section
	}

	f, err := os.OpenFile(mdPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return mdPath, fmt.Errorf("opening %s: %w", fileName, err)
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
		return mdPath, fmt.Errorf("writing %s: %w", fileName, err)
	}

	return mdPath, nil
}

func hookTargets(flagCodex, flagAll bool) []hookTarget {
	if flagAll {
		return []hookTarget{claudeHookTarget, codexHookTarget}
	}
	if flagCodex {
		return []hookTarget{codexHookTarget}
	}
	return []hookTarget{claudeHookTarget}
}

func installOutputKeys(target hookTarget, singleDefault bool) (hookKey, settingsKey string) {
	if singleDefault {
		return "Hook", "Settings"
	}
	return target.displayName + " Hook", target.displayName + " Settings"
}

func hookInstallCmd(ctx *CLIContext) *cobra.Command {
	var flagClaudeMD bool
	var flagCodex bool
	var flagCodexMD bool
	var flagAgentsMD bool
	var flagAll bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install PostToolUse hook for automatic re-indexing",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := ctx.RepoRoot()
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			if flagCodexMD || flagAgentsMD {
				flagCodex = true
			}

			targets := hookTargets(flagCodex, flagAll || (flagCodex && flagClaudeMD))
			singleDefault := len(targets) == 1 && targets[0].name == "claude" && !flagCodex && !flagAll

			var kvs []output.KV
			for _, target := range targets {
				path := target.settingsPath(repoRoot)
				settings, err := loadSettings(path)
				if err != nil {
					return fmt.Errorf("reading %s settings: %w", target.displayName, err)
				}

				hookStatus := "installed"
				if hasAtlasHook(settings) {
					hookStatus = "already installed"
				} else {
					addAtlasHook(settings)
					if err := saveSettings(path, settings); err != nil {
						return fmt.Errorf("writing %s settings: %w", target.displayName, err)
					}
				}

				hookKey, settingsKey := installOutputKeys(target, singleDefault)
				kvs = append(kvs,
					output.KV{Key: hookKey, Value: hookStatus},
					output.KV{Key: settingsKey, Value: path},
				)
			}

			if flagClaudeMD {
				existing, _ := os.ReadFile(filepath.Join(repoRoot, "CLAUDE.md"))
				alreadyPresent := hasAtlasInstructions(existing)
				if _, err := writeClaudeMD(repoRoot); err != nil {
					return err
				}
				mdStatus := "written"
				if alreadyPresent {
					mdStatus = "already present"
				}
				kvs = append(kvs, output.KV{Key: "CLAUDE.md", Value: mdStatus})
			}
			if flagCodexMD || flagAgentsMD {
				existing, _ := os.ReadFile(filepath.Join(repoRoot, "AGENTS.md"))
				alreadyPresent := hasAtlasInstructions(existing)
				if _, err := writeAgentsMD(repoRoot); err != nil {
					return err
				}
				mdStatus := "written"
				if alreadyPresent {
					mdStatus = "already present"
				}
				kvs = append(kvs, output.KV{Key: "AGENTS.md", Value: mdStatus})
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
	cmd.Flags().BoolVar(&flagCodex, "codex", false, "Install hook in .codex/hooks.json instead of .claude/settings.json")
	cmd.Flags().BoolVar(&flagCodexMD, "codex-md", false, "Install Codex hook and write Atlas instructions to AGENTS.md")
	cmd.Flags().BoolVar(&flagAgentsMD, "agents-md", false, "Also write Atlas instructions to AGENTS.md")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Install hooks for all supported agents")
	return cmd
}

func hookUninstallCmd(ctx *CLIContext) *cobra.Command {
	var flagCodex bool
	var flagAll bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Atlas hook",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := ctx.RepoRoot()
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			targets := hookTargets(flagCodex, flagAll)
			var kvs []output.KV
			jsonResult := make(map[string]string)
			for _, target := range targets {
				path := target.settingsPath(repoRoot)
				settings, err := loadSettings(path)
				if err != nil {
					return fmt.Errorf("reading %s settings: %w", target.displayName, err)
				}

				status := "not installed"
				if removeAtlasHook(settings) {
					if err := saveSettings(path, settings); err != nil {
						return fmt.Errorf("writing %s settings: %w", target.displayName, err)
					}
					status = "uninstalled"
				}

				if len(targets) == 1 && target.name == "claude" && !flagCodex && !flagAll {
					kvs = append(kvs, output.KV{Key: "Status", Value: status})
					if status == "uninstalled" {
						kvs = append(kvs, output.KV{Key: "File", Value: path})
					}
					jsonStatus := "not_installed"
					if status == "uninstalled" {
						jsonStatus = "uninstalled"
						jsonResult["file"] = path
					}
					jsonResult["status"] = jsonStatus
					continue
				}

				kvs = append(kvs,
					output.KV{Key: target.displayName, Value: status},
					output.KV{Key: target.displayName + " File", Value: path},
				)
				jsonResult[target.name+"_status"] = strings.ReplaceAll(status, " ", "_")
				jsonResult[target.name+"_file"] = path
			}
			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				return f.WriteText(kvs)
			}
			return f.Write(jsonResult)
		},
	}
	cmd.Flags().BoolVar(&flagCodex, "codex", false, "Remove hook from .codex/hooks.json instead of .claude/settings.json")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Remove hooks for all supported agents")
	return cmd
}

func hookStatusCmd(ctx *CLIContext) *cobra.Command {
	var flagCodex bool
	var flagAll bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check if Atlas hook is installed",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := ctx.RepoRoot()
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			targets := hookTargets(flagCodex, flagAll)
			var kvs []output.KV
			jsonResult := make(map[string]string)
			for _, target := range targets {
				path := target.settingsPath(repoRoot)
				settings, err := loadSettings(path)
				if err != nil {
					return fmt.Errorf("reading %s settings: %w", target.displayName, err)
				}

				status := "not installed"
				if hasAtlasHook(settings) {
					status = "installed"
				}

				if len(targets) == 1 && target.name == "claude" && !flagCodex && !flagAll {
					kvs = append(kvs,
						output.KV{Key: "Hook", Value: status},
						output.KV{Key: "File", Value: path},
					)
					jsonResult["status"] = status
					jsonResult["file"] = path
					continue
				}

				kvs = append(kvs,
					output.KV{Key: target.displayName, Value: status},
					output.KV{Key: target.displayName + " File", Value: path},
				)
				jsonResult[target.name+"_status"] = status
				jsonResult[target.name+"_file"] = path
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				return f.WriteText(kvs)
			}
			return f.Write(jsonResult)
		},
	}
	cmd.Flags().BoolVar(&flagCodex, "codex", false, "Check hook in .codex/hooks.json instead of .claude/settings.json")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Check hooks for all supported agents")
	return cmd
}
