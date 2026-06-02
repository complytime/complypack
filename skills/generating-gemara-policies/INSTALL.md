# Installing the Gemara Policy Generation Skill

This skill works with any AI agent that can:
- Read markdown documentation
- Access MCP servers or file-based catalogs
- Generate Rego code
- Write files to disk

## Quick Install (Recommended)

### Using OpenPackage (OPKG)

**Install from GitHub:**
```bash
opkg install gh@complytime/complypack/skills/generating-gemara-policies
```

**Or install locally:**
```bash
cd /path/to/complypack
opkg install skills/generating-gemara-policies
```

**Interactive selection:**
```bash
opkg install gh@complytime/complypack --interactive
# Then select: skills > generating-gemara-policies
```

OpenPackage automatically installs to the correct location for your platform (Claude Code, Cursor, Windsurf, etc.).

**Learn more:** [OpenPackage Documentation](https://openpackage.dev/docs)

## Manual Installation by Platform

### Claude Code (Anthropic)

**Location:** `~/.claude/skills/generating-gemara-policies/`

```bash
mkdir -p ~/.claude/skills/generating-gemara-policies
cp skills/generating-gemara-policies/SKILL.md ~/.claude/skills/generating-gemara-policies/
```

Claude Code automatically discovers skills in `~/.claude/skills/`.

**Verify:**
```bash
ls ~/.claude/skills/generating-gemara-policies/SKILL.md
```

### Cursor

**Location:** `.cursor/skills/generating-gemara-policies/` (project-level)

```bash
mkdir -p .cursor/skills/generating-gemara-policies
cp skills/generating-gemara-policies/SKILL.md .cursor/skills/generating-gemara-policies/
```

May require explicit `@` mention: `@generating-gemara-policies`

### Windsurf

**Location:** `.windsurf/skills/generating-gemara-policies/` (project-level)

```bash
mkdir -p .windsurf/skills/generating-gemara-policies
cp skills/generating-gemara-policies/SKILL.md .windsurf/skills/generating-gemara-policies/
```

### Other AI Agents

**For agents that support skill directories:**
Check your agent's documentation for the skills directory location, then copy `SKILL.md` there.

**For web-based AI without skill auto-loading:**

1. Copy skill content to your prompt
2. Prepend to request:

```
I have a skill for generating Rego policies from Gemara controls.

[Paste SKILL.md content here]

Now, using this skill: Generate a policy for AC-1 targeting Kubernetes using Conftest.
```

## Project-Level Installation

To include this skill in your project for team sharing:

```bash
cd /path/to/your/project

# For Claude Code projects
mkdir -p .claude/skills
cp /path/to/complypack/skills/generating-gemara-policies/SKILL.md \
   .claude/skills/generating-gemara-policies/

# For Cursor projects
mkdir -p .cursor/skills
cp /path/to/complypack/skills/generating-gemara-policies/SKILL.md \
   .cursor/skills/generating-gemara-policies/

git add .claude/skills/ .cursor/skills/
git commit -m "Add Gemara policy generation skill"
```

## Verification

After installation, test the skill is accessible:

**For agents with CLI:**
```bash
<agent-cli> --help  # Should show skills if supported
```

**For all agents:**
Ask the AI:
```
Do you have access to a skill called "generating-gemara-policies"?
```

If yes, it should describe the skill's purpose.

## Troubleshooting

**Skill not found:**
- Check skill is in correct directory for your platform
- Verify SKILL.md has proper frontmatter (name, description)
- Restart the AI agent/IDE

**Skill doesn't execute correctly:**
- Ensure MCP server is configured (for ComplyPack integration)
- Check agent has file write permissions
- Verify platform schemas are accessible

## Updating the Skill

When the skill is updated in the ComplyPack repo:

```bash
cd /path/to/complypack
git pull

# Re-copy to your platform's skills directory
cp skills/generating-gemara-policies/SKILL.md ~/.claude/skills/generating-gemara-policies/
# or
cp skills/generating-gemara-policies/SKILL.md .cursor/skills/generating-gemara-policies/
# etc.
```

Or use OpenPackage to update:
```bash
opkg update generating-gemara-policies
```
