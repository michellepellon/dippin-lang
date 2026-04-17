#!/bin/sh
# ABOUTME: Assemble docs/generated-spec.md from existing doc sources.
# ABOUTME: Concatenates llm-reference.md and skill.md sections into one agent-facing spec.
set -e

OUTPUT="docs/generated-spec.md"
EMBED_OUTPUT="cmd/dippin/generated-spec.md"
WEB_OUTPUT="site/static/llms-full.txt"
LLM_REF="docs/llm-reference.md"
SKILL="site/static/skill.md"

# Verify sources exist
for f in "$LLM_REF" "$SKILL"; do
  if [ ! -f "$f" ]; then
    echo "gen-spec: $f not found" >&2
    exit 1
  fi
done

cat > "$OUTPUT" << 'HEADER'
# Dippin Language Specification

Complete reference for AI agents generating `.dip` workflow files. This document is the canonical, self-contained spec for the dippin DSL and CLI toolchain.

Install: `go install github.com/2389-research/dippin-lang/cmd/dippin@latest`

Generate from template: `dippin new minimal`, `dippin new parallel`, `dippin new conditional`, `dippin new review-loop`, `dippin new human-gate`

HEADER

# --- Section 1: Grammar from llm-reference.md ---
# Extract everything from "## Grammar" through end of file, skipping the title line and first ---
sed -n '/^## Grammar/,$ p' "$LLM_REF" >> "$OUTPUT"

# --- Section 2: Syntax details from skill.md ---
# Extract from "## File Structure" up to but not including "## Documentation"
# This skips the frontmatter, title, and Quick Start (which overlap with header above)

{
  echo ""
  echo "---"
  echo ""
  sed -n '/^## File Structure/,/^## Documentation/ { /^## Documentation/d; p; }' "$SKILL"

  echo ""
  echo "---"
  echo ""
  echo "## Documentation"
  echo ""
  echo "- [Language Reference](https://dippin.org/language/)"
  echo "- [CLI Reference](https://dippin.org/cli/)"
  echo "- [Validation & Linting](https://dippin.org/validation/)"
  echo "- [Scenario Testing](https://dippin.org/testing/)"
  echo "- [Analysis Tools](https://dippin.org/analysis/)"
  echo "- [Playground](https://dippin.org/playground/)"
  echo "- [GitHub](https://github.com/2389-research/dippin-lang)"
} >> "$OUTPUT"

# Keep the checked-in release copies in sync with the scratch output.
for output in "$EMBED_OUTPUT" "$WEB_OUTPUT"; do
  cp "$OUTPUT" "$output"
done

echo "gen-spec: wrote $OUTPUT"
