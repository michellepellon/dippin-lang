#!/bin/sh
# Generate site/content/changelog.md from CHANGELOG.md.
# Outputs Hugo-compatible markdown with front matter and version-card HTML.
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

# Convert CHANGELOG.md sections into version-card HTML.
generate_body() {
  awk '
  BEGIN { inv=0; ins=0; inl=0 }

  /^## \[v[0-9]/ {
    close_all()
    # Extract version: ## [v0.11.0] ...
    ver = $0
    sub(/^## \[/, "", ver)
    sub(/\].*/, "", ver)
    # Extract date: ... — 2026-03-27
    dt = $0
    idx = index(dt, "— ")
    if (idx > 0) dt = substr(dt, idx+4)
    else { idx = index(dt, "- "); if (idx > 0) dt = substr(dt, idx+2); else dt = "" }

    printf "    <div class=\"version-card\">\n"
    printf "      <div class=\"version-header\">\n"
    printf "        <span class=\"version-tag\">%s</span>\n", ver
    printf "        <span class=\"version-date\">%s</span>\n", dt
    printf "      </div>\n"
    inv = 1
    next
  }

  /^### / && inv {
    if (inl) { printf "      </ul>\n"; inl = 0 }
    section = substr($0, 5)
    printf "      <h4>%s</h4>\n", section
    ins = 1
    next
  }

  /^- / && inv {
    if (!inl) { printf "      <ul>\n"; inl = 1 }
    line = substr($0, 3)
    gsub(/</, "\\&lt;", line)
    gsub(/>/, "\\&gt;", line)
    # Bold+code: **`text`**
    while (match(line, /\*\*`[^`]+`\*\*/)) {
      pre = substr(line, 1, RSTART-1)
      tok = substr(line, RSTART+3, RLENGTH-6)
      post = substr(line, RSTART+RLENGTH)
      line = pre "<strong><code>" tok "</code></strong>" post
    }
    # Bold: **text**
    while (match(line, /\*\*[^*]+\*\*/)) {
      pre = substr(line, 1, RSTART-1)
      tok = substr(line, RSTART+2, RLENGTH-4)
      post = substr(line, RSTART+RLENGTH)
      line = pre "<strong>" tok "</strong>" post
    }
    # Inline code: `text`
    while (match(line, /`[^`]+`/)) {
      pre = substr(line, 1, RSTART-1)
      tok = substr(line, RSTART+1, RLENGTH-2)
      post = substr(line, RSTART+RLENGTH)
      line = pre "<code>" tok "</code>" post
    }
    printf "        <li>%s</li>\n", line
    next
  }

  { next }

  function close_all() {
    if (inl) { printf "      </ul>\n"; inl = 0 }
    ins = 0
    if (inv) {
      printf "      <a class=\"gh-link\" href=\"https://github.com/2389-research/dippin-lang/releases/tag/%s\">View on GitHub &rarr;</a>\n", ver
      printf "    </div>\n\n"
      inv = 0
    }
  }

  END { close_all() }
  ' "$INPUT"
}

generate_body >> "$OUTPUT"
