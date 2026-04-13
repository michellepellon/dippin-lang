// External scanner for Dippin's indentation-sensitive syntax.
// Produces INDENT, DEDENT, and NEWLINE tokens.
//
// At each newline, the scanner measures the next line's indentation and
// emits one of INDENT (deeper), DEDENT (shallower), or NEWLINE (same).
// Multi-level dedents emit one DEDENT per call via pending_dedents.

#include "tree_sitter/parser.h"
#include <string.h>

enum TokenType { INDENT, DEDENT, NEWLINE };

#define MAX_DEPTH 128

typedef struct {
  uint16_t stack[MAX_DEPTH];
  uint16_t depth;
  bool pending_dedents;
  uint16_t target;
} Scanner;

// ── Lifecycle ────────────────────────────────────────────

void *tree_sitter_dippin_external_scanner_create(void) {
  Scanner *s = calloc(1, sizeof(Scanner));
  return s;
}

void tree_sitter_dippin_external_scanner_destroy(void *payload) {
  free(payload);
}

unsigned tree_sitter_dippin_external_scanner_serialize(void *payload,
                                                       char *buffer) {
  Scanner *s = (Scanner *)payload;
  unsigned n = 0;
  memcpy(buffer + n, &s->depth, sizeof(s->depth));
  n += sizeof(s->depth);
  unsigned sb = (s->depth + 1) * sizeof(uint16_t);
  memcpy(buffer + n, s->stack, sb);
  n += sb;
  memcpy(buffer + n, &s->pending_dedents, sizeof(s->pending_dedents));
  n += sizeof(s->pending_dedents);
  memcpy(buffer + n, &s->target, sizeof(s->target));
  n += sizeof(s->target);
  return n;
}

void tree_sitter_dippin_external_scanner_deserialize(void *payload,
                                                      const char *buffer,
                                                      unsigned length) {
  Scanner *s = (Scanner *)payload;
  if (length == 0) {
    s->depth = 0;
    s->stack[0] = 0;
    s->pending_dedents = false;
    s->target = 0;
    return;
  }
  unsigned n = 0;
  memcpy(&s->depth, buffer + n, sizeof(s->depth));
  n += sizeof(s->depth);
  unsigned sb = (s->depth + 1) * sizeof(uint16_t);
  memcpy(s->stack, buffer + n, sb);
  n += sb;
  memcpy(&s->pending_dedents, buffer + n, sizeof(s->pending_dedents));
  n += sizeof(s->pending_dedents);
  memcpy(&s->target, buffer + n, sizeof(s->target));
}

// ── Helpers ──────────────────────────────────────────────

static uint16_t cur(Scanner *s) { return s->stack[s->depth]; }

// Measure indent of the next non-blank, non-comment line.
// Consumes \n, blank lines, and comment lines.
// Returns the indent of the first real line, or 0 at EOF.
// After return, the lexer is positioned at the first non-space
// character of that line.
static uint16_t next_line_indent(TSLexer *lexer) {
  // Consume the newline.
  if (lexer->lookahead == '\n') {
    lexer->advance(lexer, true);
  }

  for (;;) {
    uint16_t indent = 0;
    while (lexer->lookahead == ' ' || lexer->lookahead == '\t') {
      indent++;
      lexer->advance(lexer, true);
    }
    if (lexer->eof(lexer)) return 0;
    if (lexer->lookahead == '\n') {
      lexer->advance(lexer, true);
      continue; // blank line
    }
    if (lexer->lookahead == '#') {
      while (lexer->lookahead != '\n' && !lexer->eof(lexer))
        lexer->advance(lexer, true);
      if (lexer->lookahead == '\n') {
        lexer->advance(lexer, true);
        continue; // comment-only line
      }
      return 0; // comment at EOF
    }
    return indent;
  }
}

// ── Main scan ────────────────────────────────────────────

bool tree_sitter_dippin_external_scanner_scan(void *payload,
                                               TSLexer *lexer,
                                               const bool *valid) {
  Scanner *s = (Scanner *)payload;

  // 1. Drain pending multi-level dedents.
  if (s->pending_dedents && valid[DEDENT]) {
    if (cur(s) > s->target && s->depth > 0) {
      s->depth--;
      lexer->result_symbol = DEDENT;
      if (cur(s) <= s->target) s->pending_dedents = false;
      return true;
    }
    s->pending_dedents = false;
  }

  // 2. Only operate at line boundaries.
  if (!valid[NEWLINE] && !valid[INDENT] && !valid[DEDENT]) return false;

  // 3. Skip horizontal whitespace before the newline.
  while (lexer->lookahead == ' ' || lexer->lookahead == '\t' ||
         lexer->lookahead == '\r')
    lexer->advance(lexer, true);

  // 4. At EOF, emit dedents to close open blocks.
  if (lexer->eof(lexer)) {
    if (valid[DEDENT] && s->depth > 0) {
      s->depth--;
      lexer->result_symbol = DEDENT;
      return true;
    }
    if (valid[NEWLINE]) {
      lexer->result_symbol = NEWLINE;
      return true;
    }
    return false;
  }

  // 5. Must see a newline character.
  if (lexer->lookahead != '\n') return false;

  // 6. Measure next real line's indent (consumes \n + whitespace).
  uint16_t indent = next_line_indent(lexer);
  lexer->mark_end(lexer);

  uint16_t c = cur(s);

  // 7. Indent increased → push.
  if (indent > c && valid[INDENT]) {
    if (s->depth + 1 < MAX_DEPTH) {
      s->depth++;
      s->stack[s->depth] = indent;
    }
    lexer->result_symbol = INDENT;
    return true;
  }

  // 8. Indent decreased → pop (may trigger pending_dedents).
  if (indent < c && valid[DEDENT]) {
    s->pending_dedents = true;
    s->target = indent;
    s->depth--;
    lexer->result_symbol = DEDENT;
    if (cur(s) <= indent) s->pending_dedents = false;
    return true;
  }

  // 9. Same level → newline.
  if (valid[NEWLINE]) {
    lexer->result_symbol = NEWLINE;
    return true;
  }

  return false;
}
