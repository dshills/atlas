# Atlas Claude Code Skill

An example [Claude Code Skill](https://docs.claude.com/en/docs/claude-code/skills) that teaches Claude to use Atlas as its primary code-navigation layer instead of grepping or reading files blindly.

## What it does

When installed, Claude will:

- Run `atlas find symbol`, `atlas who-calls`, `atlas implementations`, etc. before reading source files.
- Summarize large files with `atlas summarize file` before opening them.
- Fall back to `rg` only for content/pattern questions atlas can't answer (string literals, comments, non-indexed file types).
- Treat the atlas index as authoritative — kept fresh by the `PostToolUse` hook on Write/Edit/MultiEdit.

The result is fewer tokens spent re-reading files and more accurate answers to structural questions like "what calls this?" or "what implements this interface?".

## Install

Copy `SKILL.md` into the skills directory of whichever project (or your user-level config) you want it active in:

```bash
# Per-project
mkdir -p .claude/skills/atlas
cp examples/claude-skills/atlas/SKILL.md .claude/skills/atlas/SKILL.md

# Or user-level (applies to all projects)
mkdir -p ~/.claude/skills/atlas
cp examples/claude-skills/atlas/SKILL.md ~/.claude/skills/atlas/SKILL.md
```

Then in the target repo:

```bash
atlas init           # Create .atlas/ and initial index
atlas hook install   # Install the index-freshness hook
```

## Requirements

- `atlas` on `PATH` — see the [Atlas README](../../../README.md) for build/install instructions.
- Claude Code 2.x or later (skills support).
- A repo that has been initialized with `atlas init`.

## Customizing

`SKILL.md` is a plain Markdown file. Edit it to:

- Restrict the command set to what your team uses.
- Add project-specific workflow patterns (e.g. "before touching the billing package, run X").
- Tighten or loosen the "never read source directly" rule.

Changes take effect the next time Claude Code loads the skill.
