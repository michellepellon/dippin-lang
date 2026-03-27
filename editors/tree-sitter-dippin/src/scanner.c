// External scanner for Dippin's indentation-sensitive syntax.
// Produces INDENT, DEDENT, and NEWLINE tokens based on leading whitespace.
// Inspired by tree-sitter-python's scanner.

#include "tree_sitter/parser.h"
#include <stdio.h>
#include <string.h>

enum TokenType { INDENT, DEDENT, NEWLINE };

#define MAX_INDENT_DEPTH 128

typedef struct {
  uint16_t indent_stack[MAX_INDENT_DEPTH];
  uint16_t depth;
  bool pending_dedents;
  uint16_t target_indent;
} Scanner;

void *tree_sitter_dippin_external_scanner_create(void) {
  Scanner *s = calloc(1, sizeof(Scanner));
  s->depth = 0;
  s->indent_stack[0] = 0;
  s->pending_dedents = false;
  s->target_indent = 0;
  return s;
}

void tree_sitter_dippin_external_scanner_destroy(void *payload) {
  free(payload);
}

unsigned tree_sitter_dippin_external_scanner_serialize(void *payload,
                                                       char *buffer) {
  Scanner *s = (Scanner *)payload;
  unsigned size = 0;
  memcpy(buffer, &s->depth, sizeof(s->depth));
  size += sizeof(s->depth);
  unsigned stack_bytes = (s->depth + 1) * sizeof(uint16_t);
  memcpy(buffer + size, s->indent_stack, stack_bytes);
  size += stack_bytes;
  memcpy(buffer + size, &s->pending_dedents, sizeof(s->pending_dedents));
  size += sizeof(s->pending_dedents);
  memcpy(buffer + size, &s->target_indent, sizeof(s->target_indent));
  size += sizeof(s->target_indent);
  return size;
}

void tree_sitter_dippin_external_scanner_deserialize(void *payload,
                                                      const char *buffer,
                                                      unsigned length) {
  Scanner *s = (Scanner *)payload;
  if (length == 0) {
    s->depth = 0;
    s->indent_stack[0] = 0;
    s->pending_dedents = false;
    s->target_indent = 0;
    return;
  }
  unsigned offset = 0;
  memcpy(&s->depth, buffer, sizeof(s->depth));
  offset += sizeof(s->depth);
  unsigned stack_bytes = (s->depth + 1) * sizeof(uint16_t);
  memcpy(s->indent_stack, buffer + offset, stack_bytes);
  offset += stack_bytes;
  memcpy(&s->pending_dedents, buffer + offset, sizeof(s->pending_dedents));
  offset += sizeof(s->pending_dedents);
  memcpy(&s->target_indent, buffer + offset, sizeof(s->target_indent));
}

static uint16_t current_indent(Scanner *s) {
  return s->indent_stack[s->depth];
}

bool tree_sitter_dippin_external_scanner_scan(void *payload,
                                               TSLexer *lexer,
                                               const bool *valid_symbols) {
  Scanner *s = (Scanner *)payload;

  // Emit pending dedents before anything else.
  if (s->pending_dedents && valid_symbols[DEDENT]) {
    if (current_indent(s) > s->target_indent && s->depth > 0) {
      s->depth--;
      lexer->result_symbol = DEDENT;
      if (current_indent(s) <= s->target_indent) {
        s->pending_dedents = false;
      }
      return true;
    }
    s->pending_dedents = false;
  }

  // Look for newline.
  if (valid_symbols[NEWLINE] || valid_symbols[INDENT] || valid_symbols[DEDENT]) {
    // Skip non-newline whitespace.
    while (lexer->lookahead == ' ' || lexer->lookahead == '\t' ||
           lexer->lookahead == '\r') {
      lexer->advance(lexer, true);
    }

    if (lexer->lookahead != '\n') {
      return false;
    }

    // Consume newline.
    lexer->advance(lexer, true);

    // Skip blank lines and comment-only lines.
    for (;;) {
      uint16_t indent = 0;
      while (lexer->lookahead == ' ' || lexer->lookahead == '\t') {
        indent++;
        lexer->advance(lexer, true);
      }
      if (lexer->lookahead == '\n') {
        lexer->advance(lexer, true);
        continue;
      }
      if (lexer->lookahead == '#') {
        while (lexer->lookahead != '\n' && lexer->lookahead != 0) {
          lexer->advance(lexer, true);
        }
        if (lexer->lookahead == '\n') {
          lexer->advance(lexer, true);
          continue;
        }
      }

      // Found a non-blank, non-comment line. Compare indent.
      uint16_t cur = current_indent(s);
      if (indent > cur && valid_symbols[INDENT]) {
        // Emit NEWLINE first, then INDENT on next scan.
        if (s->depth + 1 < MAX_INDENT_DEPTH) {
          s->depth++;
          s->indent_stack[s->depth] = indent;
        }
        lexer->result_symbol = INDENT;
        lexer->mark_end(lexer);
        return true;
      } else if (indent < cur && valid_symbols[DEDENT]) {
        s->pending_dedents = true;
        s->target_indent = indent;
        s->depth--;
        lexer->result_symbol = DEDENT;
        lexer->mark_end(lexer);
        if (current_indent(s) <= indent) {
          s->pending_dedents = false;
        }
        return true;
      } else if (valid_symbols[NEWLINE]) {
        lexer->result_symbol = NEWLINE;
        lexer->mark_end(lexer);
        return true;
      }
      return false;
    }
  }

  return false;
}
