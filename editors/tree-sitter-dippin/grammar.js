// Tree-sitter grammar for the Dippin workflow language.
// Dippin is indentation-sensitive (like Python); INDENT/DEDENT tokens
// are produced by the external scanner in src/scanner.c.

module.exports = grammar({
  name: "dippin",

  externals: ($) => [$._indent, $._dedent, $._newline],

  extras: ($) => [/[ \t\r]/, $.comment],

  word: ($) => $.identifier,

  rules: {
    source_file: ($) => $.workflow_decl,

    workflow_decl: ($) =>
      seq("workflow", $.identifier, $._newline, $._indent, $.workflow_body, $._dedent),

    workflow_body: ($) =>
      repeat1(
        choice(
          $.workflow_field,
          $.defaults_section,
          $.node_decl,
          $.edges_section,
          $.stylesheet_section,
          $._newline
        )
      ),

    workflow_field: ($) =>
      seq(
        choice("goal", "start", "exit"),
        ":",
        $.field_value,
        $._newline
      ),

    // ── Defaults ──────────────────────────────────────────────
    defaults_section: ($) =>
      seq("defaults", $._newline, $._indent, repeat1($.defaults_field), $._dedent),

    defaults_field: ($) =>
      seq($.field_name, ":", $.field_value, $._newline),

    // ── Nodes ─────────────────────────────────────────────────
    node_decl: ($) =>
      choice(
        $.agent_node,
        $.human_node,
        $.tool_node,
        $.subgraph_node,
        $.parallel_node,
        $.fan_in_node
      ),

    agent_node: ($) =>
      seq("agent", $.identifier, $._newline, $._indent, repeat1($.node_field), $._dedent),

    human_node: ($) =>
      seq("human", $.identifier, $._newline, $._indent, repeat1($.node_field), $._dedent),

    tool_node: ($) =>
      seq("tool", $.identifier, $._newline, $._indent, repeat1($.node_field), $._dedent),

    subgraph_node: ($) =>
      seq("subgraph", $.identifier, $._newline, $._indent, repeat1($.node_field), $._dedent),

    parallel_node: ($) =>
      seq("parallel", $.identifier, "->", $.identifier_list, $._newline),

    fan_in_node: ($) =>
      seq("fan_in", $.identifier, "<-", $.identifier_list, $._newline),

    node_field: ($) => seq($.field_name, ":", $.field_value, $._newline),

    // ── Edges ─────────────────────────────────────────────────
    edges_section: ($) =>
      seq("edges", $._newline, $._indent, repeat1($.edge_entry), $._dedent),

    edge_entry: ($) =>
      seq(
        $.identifier,
        "->",
        $.identifier,
        repeat($.edge_attr),
        $._newline
      ),

    edge_attr: ($) =>
      choice(
        seq("when", $.condition),
        seq("label", ":", $.field_value),
        seq("weight", ":", $.field_value),
        seq("restart", ":", $.field_value)
      ),

    condition: ($) => $.or_expr,

    or_expr: ($) => prec.left(1, seq($.and_expr, repeat(seq("or", $.and_expr)))),

    and_expr: ($) => prec.left(2, seq($.compare_expr, repeat(seq("and", $.compare_expr)))),

    compare_expr: ($) =>
      prec.left(
        3,
        seq(
          $.operand,
          optional(
            seq(optional("not"), $.compare_op, $.operand)
          )
        )
      ),

    compare_op: ($) =>
      choice("==", "!=", "=", "contains", "startswith", "endswith", "in"),

    operand: ($) => choice($.variable, $.string, $.identifier),

    variable: ($) => seq($.identifier, ".", $.identifier),

    // ── Stylesheet ────────────────────────────────────────────
    stylesheet_section: ($) =>
      seq(
        "stylesheet",
        ":",
        $._newline,
        $._indent,
        repeat1($.stylesheet_rule),
        $._dedent
      ),

    stylesheet_rule: ($) =>
      seq(
        $.selector,
        $._newline,
        $._indent,
        repeat1(seq($.field_name, ":", $.field_value, $._newline)),
        $._dedent
      ),

    selector: ($) =>
      choice(
        "*",
        seq(".", $.identifier),
        seq("#", $.identifier),
        $.identifier
      ),

    // ── Shared ────────────────────────────────────────────────
    field_name: ($) => $.identifier,

    field_value: ($) =>
      choice($.string, $.multiline_block, $.raw_inline),

    raw_inline: ($) => /[^\n]+/,

    multiline_block: ($) =>
      seq($._newline, $._indent, $.block_content, $._dedent),

    block_content: ($) => repeat1(choice($.block_line, $._newline)),

    block_line: ($) => /[^\n]+/,

    identifier_list: ($) => seq($.identifier, repeat(seq(",", $.identifier))),

    identifier: ($) => /[a-zA-Z0-9][a-zA-Z0-9_\-.]*/,

    string: ($) =>
      seq('"', repeat(choice(/[^"\\]+/, /\\./)), '"'),

    comment: ($) => token(seq("#", /.*/)),
  },
});
