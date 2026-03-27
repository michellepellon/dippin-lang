#!/bin/sh
# Sync the navigation bar across all site pages from a single source.
#
# Usage: ./scripts/sync-nav.sh
#
# Reads the nav template from site/_layout/nav.html and replaces the
# <nav>...</nav> block in every page. Handles path prefixes and
# active states automatically.
set -e

NAV_TEMPLATE="site/_layout/nav.html"

if [ ! -f "$NAV_TEMPLATE" ]; then
  echo "sync-nav: $NAV_TEMPLATE not found, skipping"
  exit 0
fi

# inject_nav <file> <prefix> <blog_href> <active_key>
inject_nav() {
  file="$1"
  prefix="$2"
  blog_href="$3"
  active="$4"
  tmp=$(mktemp)
  nav_tmp=$(mktemp)

  # Build nav with prefix
  sed -e "s|{{PREFIX}}|$prefix|g" -e "s|{{BLOG_HREF}}|$blog_href|g" "$NAV_TEMPLATE" > "$nav_tmp"

  # Set active class
  case "$active" in
    docs)         sed -i '' 's|>Docs</a>| class="active">Docs</a>|;s|>Language</a>| class="active">Language</a>|' "$nav_tmp" ;;
    playground)   sed -i '' 's|>Playground</a>| class="active">Playground</a>|' "$nav_tmp" ;;
    blog)         sed -i '' 's|>Blog</a>| class="active">Blog</a>|' "$nav_tmp" ;;
    cli)          sed -i '' 's|>CLI</a>| class="active">CLI</a>|' "$nav_tmp" ;;
    language)     sed -i '' 's|>Language</a>| class="active">Language</a>|' "$nav_tmp" ;;
    testing)      sed -i '' 's|>Testing</a>| class="active">Testing</a>|' "$nav_tmp" ;;
    validation)   sed -i '' 's|>Validation</a>| class="active">Validation</a>|' "$nav_tmp" ;;
    analysis)     sed -i '' 's|>Analysis</a>| class="active">Analysis</a>|' "$nav_tmp" ;;
    architecture) sed -i '' 's|>Architecture</a>| class="active">Architecture</a>|' "$nav_tmp" ;;
    editors)      sed -i '' 's|>Editors</a>| class="active">Editors</a>|' "$nav_tmp" ;;
    changelog)    sed -i '' 's|>Changelog</a>| class="active">Changelog</a>|' "$nav_tmp" ;;
  esac

  # Replace <nav>...</nav> block: print lines before nav, insert new nav,
  # skip old nav lines, print lines after nav
  awk '
    /<nav class="nav">/ { in_nav=1; while((getline line < "'"$nav_tmp"'") > 0) print line; next }
    /<\/nav>/           { if(in_nav) { in_nav=0; next } }
    in_nav              { next }
    { print }
  ' "$file" > "$tmp" && mv "$tmp" "$file"
  rm -f "$nav_tmp"
}

echo "Syncing nav..."

# Root pages
inject_nav site/index.html         ""   "blog/index.html" ""
inject_nav site/cli.html           ""   "blog/index.html" cli
inject_nav site/language.html      ""   "blog/index.html" language
inject_nav site/testing.html       ""   "blog/index.html" testing
inject_nav site/validation.html    ""   "blog/index.html" validation
inject_nav site/analysis.html      ""   "blog/index.html" analysis
inject_nav site/architecture.html  ""   "blog/index.html" architecture
inject_nav site/editors.html       ""   "blog/index.html" editors
inject_nav site/playground.html    ""   "blog/index.html" playground
inject_nav site/changelog.html     ""   "blog/index.html" changelog

# Blog pages
for f in site/blog/*.html; do
  [ -f "$f" ] || continue
  inject_nav "$f" "../" "index.html" blog
done

echo "Done."
