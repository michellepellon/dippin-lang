#!/bin/sh
# Generate site/changelog.html from CHANGELOG.md.
# Called by pre-commit hook when CHANGELOG.md is staged.
set -e

INPUT="CHANGELOG.md"
OUTPUT="site/changelog.html"

if [ ! -f "$INPUT" ]; then
  echo "gen-changelog-html: $INPUT not found, skipping"
  exit 0
fi

# Convert markdown to HTML body content using sed-friendly parsing.
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

# Write header
cat > "$OUTPUT" << 'HEADER'
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Changelog — Dippin</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Fraunces:ital,opsz,wght@0,9..144,300;0,9..144,500;0,9..144,700;1,9..144,400&family=IBM+Plex+Mono:wght@400;500;600&family=Nunito:wght@400;500;600;700&display=swap" rel="stylesheet">
<link rel="stylesheet" href="style.css">
<style>
.version-card { background: white; border: 1.5px solid var(--border); border-radius: 14px; padding: 2rem; margin-bottom: 1.5rem; position: relative; overflow: hidden; }
.version-card::before { content: ''; position: absolute; left: 0; top: 0; bottom: 0; width: 4px; border-radius: 14px 0 0 14px; }
.version-card:nth-child(4n+1)::before { background: var(--green-deep); }
.version-card:nth-child(4n+2)::before { background: var(--lavender-deep); }
.version-card:nth-child(4n+3)::before { background: var(--yellow-deep); }
.version-card:nth-child(4n+4)::before { background: var(--pink-deep); }
.version-header { display: flex; align-items: baseline; gap: 1rem; margin-bottom: 1rem; flex-wrap: wrap; }
.version-tag { font-family: var(--mono); font-size: 1.3rem; font-weight: 700; color: var(--text); }
.version-date { font-family: var(--mono); font-size: 0.8rem; color: var(--text-soft); }
.version-card h4 { font-size: 0.85rem; font-weight: 700; color: var(--text); margin: 1.2rem 0 0.5rem; text-transform: uppercase; letter-spacing: 0.05em; }
.version-card h4:first-of-type { margin-top: 0; }
.version-card ul { margin: 0; padding-left: 1.3rem; }
.version-card li { color: var(--text-mid); font-size: 0.88rem; line-height: 1.7; margin-bottom: 0.3rem; }
.version-card li strong { color: var(--text); }
.version-card li code { font-size: 0.78em; }
.gh-link { font-family: var(--mono); font-size: 0.75rem; color: var(--green-deep); margin-top: 0.8rem; display: inline-block; }
</style>
</head>
<body>

<!-- Floating dots -->
<div class="dots">
  <div class="dot"></div><div class="dot"></div><div class="dot"></div>
  <div class="dot"></div><div class="dot"></div><div class="dot"></div>
  <div class="dot"></div><div class="dot"></div><div class="dot"></div>
  <div class="dot"></div><div class="dot"></div><div class="dot"></div>
</div>

<nav class="nav">
<!-- placeholder: sync-nav.sh replaces this block -->
</nav>

<section class="doc">
<div class="container">
  <div class="doc-header">
    <div class="section-label">History</div>
    <h1>Changelog</h1>
    <p>All notable changes to dippin-lang. Versions follow <a href="https://semver.org/">semver</a>.</p>
  </div>

  <div class="doc-body">
HEADER

# Generate version cards from CHANGELOG.md
generate_body >> "$OUTPUT"

# Write footer
cat >> "$OUTPUT" << 'FOOTER'
  </div>
</div>
</section>

<footer>
<div class="container">
  <div class="footer-inner">
    <div class="footer-brand">Dippin</div>
    <div class="footer-links">
      <a href="https://github.com/2389-research/dippin-lang">GitHub</a>
      <a href="https://github.com/2389-research/dippin-lang/tree/main/docs">Docs</a>
      <a href="changelog.html">Changelog</a>
    </div>
  </div>
  <div class="footer-copy">
    MIT License &middot; Built by <a href="https://github.com/2389-research">2389 Research</a>
  </div>
</div>
</footer>

<script src="highlight.js"></script>
</body>
</html>
FOOTER
