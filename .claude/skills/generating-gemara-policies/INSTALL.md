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
opkg install gh@complytime/complypack --skills generating-gemara-policies
```

**Or install locally:**
```bash
cd /path/to/complypack
opkg install .claude/skills/generating-gemara-policies
```

OpenPackage automatically installs to the correct location for your platform (Claude Code, Cursor, Codex, etc.).

**Learn more:** [OpenPackage Documentation](https://openpackage.dev/docs)

### Using Agent Package Manager (APM)

If your project uses `apm.yml`:

```yaml
# apm.yml
skills:
  - name: generating-gemara-policies
    source: github:complytime/complypack/.claude/skills/generating-gemara-policies
    platforms: [claude-code, cursor, codex, github-copilot]
```

Then run:
```bash
apm install
```

**Learn more:** [APM Documentation](https://microsoft.github.io/apm/)

### Using agentget

```bash
agentget install complytime/complypack/skills/generating-gemara-policies
```

**Learn more:** [agentget Documentation](https://agentget.sh/)

## Manual Installation by Platform

### Claude Code (Anthropic)

**Location:** `~/.claude/skills/generating-gemara-policies/`

Already installed at this location. Claude Code automatically discovers skills in `~/.claude/skills/`.

**Verify:**
```bash
ls ~/.claude/skills/generating-gemara-policies/SKILL.md
```

### Codex (if used)

**Location:** `~/.codex/skills/generating-gemara-policies/`

```bash
mkdir -p ~/.codex/skills/generating-gemara-policies
cp ~/.claude/skills/generating-gemara-policies/SKILL.md \
   ~/.codex/skills/generating-gemara-policies/
```

### Cursor / Other IDEs with AI

**Project-level skill (recommended):**

```bash
# In your project root
mkdir -p .claude/skills/generating-gemara-policies
cp ~/.claude/skills/generating-gemara-policies/SKILL.md \
   .claude/skills/generating-gemara-policies/
```

Add to `.gitignore` if you don't want to commit:
```
.claude/skills/
```

Or commit to share with team:
```bash
git add .claude/skills/
git commit -m "Add Gemara policy generation skill"
```

### Generic AI Agents (ChatGPT, etc.)

If using web-based AI without skill auto-loading:

1. **Copy skill content** to your prompt
2. **Prepend to request:**

```
I have a skill for generating Rego policies from Gemara controls.

[Paste SKILL.md content here]

Now, using this skill: Generate a policy for AC-1 targeting Kubernetes using Conftest.
```

### Project-Specific Installation (Share with Team)

**Option 1: Check into repo**
```bash
cd /path/to/complypack
mkdir -p .claude/skills
cp -r ~/.claude/skills/generating-gemara-policies .claude/skills/
git add .claude/skills/generating-gemara-policies/
git commit -m "docs: add Gemara policy generation skill"
```

**Option 2: Symlink (personal)**
```bash
cd /path/to/complypack
mkdir -p .claude/skills
ln -s ~/.claude/skills/generating-gemara-policies \
      .claude/skills/generating-gemara-policies
```

## Verification

After installation, test the skill is accessible:

**For Claude Code:**
```bash
claude --help  # Should show skills in available commands
```

**For other platforms:**
Ask the AI agent:
```
Do you have access to a skill called "generating-gemara-policies"?
```

If yes, it should describe the skill's purpose.

## Multi-Agent Sync

To keep the skill synchronized across different agents:

**Option 1: Central location with symlinks**
```bash
# Central skills directory
SKILLS_DIR=~/Documents/ai-skills

# Create central location
mkdir -p $SKILLS_DIR/generating-gemara-policies
cp SKILL.md $SKILLS_DIR/generating-gemara-policies/

# Symlink from each agent
ln -s $SKILLS_DIR/generating-gemara-policies ~/.claude/skills/
ln -s $SKILLS_DIR/generating-gemara-policies ~/.codex/skills/
```

**Option 2: Git repository**
```bash
# Create skills repo
cd ~/Documents
mkdir ai-skills && cd ai-skills
git init
cp -r ~/.claude/skills/generating-gemara-policies .
git add .
git commit -m "Initial commit: Gemara policy generation skill"

# Clone into each agent's directory
cd ~/.claude/skills
git clone ~/Documents/ai-skills/generating-gemara-policies

cd ~/.codex/skills
git clone ~/Documents/ai-skills/generating-gemara-policies
```

Update all locations:
```bash
cd ~/Documents/ai-skills/generating-gemara-policies
git pull
cd ~/.claude/skills/generating-gemara-policies && git pull
cd ~/.codex/skills/generating-gemara-policies && git pull
```

## Platform-Specific Notes

### Claude Code
- Automatically loads skills from `~/.claude/skills/`
- Project skills in `.claude/skills/` take precedence
- No configuration needed

### Cursor
- May require explicit `@` mention: `@generating-gemara-policies`
- Check Cursor settings for skill directory paths

### Generic LLMs
- No auto-loading - must include in prompt
- Consider using "custom instructions" or "system prompts" if available

## Updating the Skill

When the skill is updated:

```bash
# Update in central location
cd ~/Documents/ai-skills/generating-gemara-policies
# Edit SKILL.md

# If using git
git add SKILL.md
git commit -m "Update policy verification steps"
git push

# Pull to all agent locations
cd ~/.claude/skills/generating-gemara-policies && git pull
cd ~/.codex/skills/generating-gemara-policies && git pull
```

## Troubleshooting

**Skill not found:**
- Check skill is in correct directory for your platform
- Verify SKILL.md has proper frontmatter (name, description)
- Restart the AI agent/IDE

**Skill doesn't execute correctly:**
- Ensure MCP server is configured (for ComplyPack integration)
- Check agent has file write permissions
- Verify platform schemas are accessible

## Adding to ComplyPack Project

To make this skill available for all ComplyPack users:

```bash
cd /path/to/complypack
mkdir -p .claude/skills
cp -r ~/.claude/skills/generating-gemara-policies .claude/skills/
git add .claude/skills/
git commit -m "Add Gemara policy generation skill for team use"
git push
```

Team members will get the skill when they clone/pull the repo.
