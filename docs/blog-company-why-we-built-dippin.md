# Why We Built a Language for AI Pipelines

Last March, one of our engineers spent forty minutes debugging a broken pipeline. The fix was a missing backslash in a DOT file — one character, buried inside a string that looked like this:

```
tool_command="set -eu\nmkdir -p .ai .ai/drafts .ai/sprints\nif [ ! -f
.ai/ledger.tsv ]; then\n  now=$(date -u +%Y-%m-%dT%H:%M:%SZ)\n  printf
'sprint_id\\ttitle\\tstatus\\tcreated_at\\tupdated_at\\n001\\tBootstrap
sprint\\tplanned\\t%s\\t%s\\n' \"$now\" \"$now\" > .ai/ledger.tsv\nfi\n
printf 'ledger-ready'"
```

That's a shell script. Six lines of bash — create a directory, write a TSV header if it doesn't exist, print a status message. Nothing exotic. But inside a DOT attribute, every newline becomes `\n`, every tab becomes `\\t`, every quote becomes `\"`. The script is technically there, but you can't read it. You certainly can't edit it with confidence.

We build Tracker, an AI pipeline orchestration system. Tracker runs multi-step workflows where LLM agents, tool calls, and human reviewers collaborate on complex tasks — code review, sprint execution, API design. These pipelines are directed graphs: nodes with prompts and models, edges with conditions, retry loops, parallel branches. For two years, we defined them in Graphviz DOT.

DOT worked when our pipelines were small. Five nodes, simple edges, short prompts. But our pipelines grew. Twenty-node workflows with multi-model consensus. Shell scripts that run test suites. System prompts with embedded markdown and JSON schemas. At some point, the authoring format stopped being invisible and started being the thing we fought with most.

We'd spent more time debugging escaped strings than writing prompts. That was the signal.

## What DOT couldn't give us

The escaped-string problem was the most visible pain, but it wasn't the only one.

DOT is a graph description language. It knows about nodes and edges and attributes. It does not know what an AI pipeline is. It has no opinion about whether `claude-sonnet-4-6` is a valid model name or `claude-sonet-4-6` is a typo. It won't tell you that a node is unreachable, that a retry loop has no exit condition, or that a tool command references a binary that doesn't exist on PATH. You find these things out at runtime — in production, when the pipeline fails or, worse, produces subtly wrong output.

We also had no way to test pipelines. LLM calls are non-deterministic; you can't assert on their output. But you can assert on the *shape* of execution: which nodes were visited, in what order, which branches were taken. We needed that, and DOT had no concept of it.

Cost was another blind spot. A pipeline that fans out to three LLM providers runs three sets of API calls. Before we could estimate that cost, someone had to manually count prompt tokens and look up pricing tables. For twenty pipelines, that's not sustainable.

## A language, not a format

So we built Dippin. Not a YAML schema, not a JSON config format — a language, with a parser, a grammar, and an opinion about how pipeline definitions should look.

That same shell script, in Dippin:

```
tool EnsureLedger
  label: "Ensure Ledger"
  command:
    set -eu
    mkdir -p .ai .ai/drafts .ai/sprints
    if [ ! -f .ai/ledger.tsv ]; then
      now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
      printf 'sprint_id\ttitle\tstatus\tcreated_at\tupdated_at\n001\tBootstrap sprint\tplanned\t%s\t%s\n' "$now" "$now" > .ai/ledger.tsv
    fi
    printf 'ledger-ready'
```

Indent after the colon. Write your script. No escaping, no quoting, no `\n`. The same rule applies to prompts — multi-line markdown with headers, bullet points, embedded code blocks, JSON examples. Write it the way you'd write it in a document. The parser handles the rest.

A complete workflow reads the way you'd explain it to a colleague:

```
workflow ReviewPipeline
  goal: "Draft, review, and publish a document"
  start: Start
  exit: Exit

  defaults
    provider: anthropic
    model: claude-sonnet-4-6

  agent Draft
    label: "Write Draft"
    prompt:
      Write a clear, concise technical document based on the
      provided requirements. Focus on accuracy and readability.

  agent Review
    label: "Review Draft"
    auto_status: true
    prompt:
      Review the draft for accuracy, clarity, and completeness.
      Return success if it meets standards, or fail with feedback.

  agent Publish
    label: Publish

  edges
    Start -> Draft
    Draft -> Review
    Review -> Publish  when ctx.outcome = success
    Review -> Draft    when ctx.outcome = fail
    Publish -> Exit
```

You can read that. A new engineer can read that. The conditional edges say what they mean: if the review passes, publish; if it fails, go back to drafting. No manual required.

## Tooling follows language

Once you have a real language — with a grammar, a parser, and a typed intermediate representation — tooling becomes possible in ways that a config format can never support.

Dippin ships with 39 diagnostic checks. Nine catch structural errors: your file references a node that doesn't exist, or declares a start node that has no outgoing edges. Thirty catch semantic problems: an unknown model name (DIP108), a tool command with no timeout (DIP111), a condition that references a variable without its namespace prefix (DIP120). Every diagnostic has a code, an explanation, and a fix suggestion. Run `dippin explain DIP108` and it tells you what went wrong and what to do about it.

We can now test pipelines. Dippin's scenario testing lets you inject context values and assert on execution paths — which nodes were visited, which weren't, what order they ran in. The tests are deterministic even though the underlying LLM calls are not. Our CI runs `dippin check` on every push; a broken pipeline never reaches production.

Cost estimation followed naturally. `dippin cost` counts prompt tokens, applies per-model pricing, and accounts for retry loops:

```
$ dippin cost complexity_cleanup.dip
═══ Cost Estimate ═════════════════════════════════════════
                                Min Expected      Max
  ──────────────────────── ──────── ──────── ────────
  TOTAL                       $0.65    $0.65    $2.66
```

`dippin optimize` then suggests where cheaper models would do the same job. Our code review pipeline dropped from $0.65 to $0.02 expected cost after following its suggestions.

The toolchain kept growing. An LSP server for real-time diagnostics in your editor. A WASM playground to try Dippin in the browser. A `watch` command for instant feedback on save. A tree-sitter grammar for syntax highlighting. A semantic diff tool that reports "the model changed from opus to sonnet on this node" instead of a raw text diff. A migration tool that converts DOT files with structural parity verification.

None of this would be possible with DOT. Not because DOT is bad software — it's excellent at what it does. But it's a graph description format, and we needed a pipeline authoring language.

## What this means in practice

The numbers are nice. The real change is how it feels to work on pipelines.

Pipeline authors think about pipeline logic now, not string escaping. A new team member reads a `.dip` file and understands the workflow without a walkthrough. When someone pushes a change, CI validates it structurally, checks it semantically, runs the test scenarios, and estimates the cost delta. The change either passes or it doesn't — no ambiguity.

The feedback loop between Tracker and the language has gotten tight. Last week, Tracker needed to force LLM APIs to return structured JSON — a feature that all three providers support, but that requires specific API parameters to activate. In DOT, adding this would have meant inventing an attribute convention, documenting it somewhere, and hoping people used it correctly. In Dippin, we added `response_format` and `response_schema` as first-class fields with four lint rules to catch mistakes. The Tracker adapter picked them up automatically. Request to production in a day.

That's the real payoff of treating your pipeline definitions as a first-class language: the entire system gets smarter every time you add a feature. The parser validates it. The formatter normalizes it. The linter catches misuse. The cost estimator accounts for it. The LSP shows it in your editor. You add one thing and it works everywhere, because the language is the single source of truth.

## Try it

Dippin is [open source](https://github.com/2389-research/dippin-lang). We built it for ourselves, but the problem it solves isn't unique to us. If you're building multi-step LLM pipelines — with conditional routing, human checkpoints, tool calls, retry logic — and you're defining them in YAML or JSON or DOT or raw config files, you're probably dealing with some version of the same friction we were.

Install it, point it at a workflow, and see what it finds.

```
go install github.com/2389-research/dippin-lang/cmd/dippin@latest
```

The best developer tools are the ones that disappear. They don't make you think about the tool — they let you think about the problem. That's what we set out to build, and after sixteen releases, five production pipelines, and a team that no longer dreads editing prompts, we think we're getting close.
