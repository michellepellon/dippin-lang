#!/bin/sh
# ABOUTME: Generate site/content/changelog.md from CHANGELOG.md.
# ABOUTME: Prepends Hugo front matter and strips the top-level heading.
set -e

INPUT="CHANGELOG.md"
OUTPUT="site/content/changelog.md"

if [ ! -f "$INPUT" ]; then
  echo "gen-changelog: $INPUT not found, skipping"
  exit 0
fi

# Write front matter
cat > "$OUTPUT" << 'HEADER'
---
title: "Changelog"
description: "Version history and release notes for dippin-lang."
navActive: "changelog"
layout: "changelog"
---
HEADER

# Strip the top-level heading and preamble, pass through the rest as markdown.
# CHANGELOG.md starts with "# Changelog" and a description line, then version sections.
awk '
  /^## \[v[0-9]/ { started=1 }
  started { print }
' "$INPUT" >> "$OUTPUT"
