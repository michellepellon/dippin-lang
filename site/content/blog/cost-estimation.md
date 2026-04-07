---
title: "Cost Estimation: Know Before You Run"
date: "2026-03-31"
description: "Estimate per-run pipeline costs before spending real money on LLM calls. Use dippin cost and dippin optimize to find savings."
tagStyle: "tutorial"
tagLabel: "TUTORIAL"
readTime: "6 min read"
related:
  - url: "multi-line-prompts.html"
    title: "Multi-line Prompts"
    summary: "The #1 reason Dippin exists -- write prompts without escaping."
  - url: "conditional-edges.html"
    title: "Conditional Edges"
    summary: "Route pipelines based on LLM output with the when keyword."
---

You've got a branching pipeline with conditional edges. Before you run it against
real APIs, let's find out what it'll cost. Dippin's `cost` command
gives you a per-run estimate from the `.dip` file alone -- no API calls
required.

## <span class="step-num">1</span> dippin cost

Let's use `complexity_cleanup.dip` from the examples directory -- a
real pipeline with tool nodes, retry loops, and a mix of models. Run
`dippin cost` on it:

<pre>
<span class="hl-dim">$</span> <span class="hl-shcmd">dippin cost complexity_cleanup.dip</span>
&#x2550;&#x2550;&#x2550; Cost Estimate &#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;
                            Min Expected      Max
  &#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500; &#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500; &#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500; &#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;
  TOTAL                       $0.65    $0.65    $2.66

&#x2500;&#x2500;&#x2500; By Provider &#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;
  anthropic                   $0.65    $0.65    $2.66

&#x2500;&#x2500;&#x2500; Top Cost Drivers &#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;
  Refactor                    $1.46 (max)  anthropic/claude-opus-4-6
  Triage                      $0.50 (max)  anthropic/claude-opus-4-6
  QualityGate                 $0.49 (max)  anthropic/claude-opus-4-6
  Done                        $0.21 (max)  anthropic/claude-sonnet-4-6
  RemeasureCognitive          $0.00 (max)  /
</pre>

The output has three sections worth understanding. The top section shows three
columns: **min**, **expected**, and **max**.
Min is the cost if every conditional branch takes the cheapest path. Max is the
worst case if every node runs every retry. Expected sits in the middle -- the
most likely path through the pipeline.

The **By Provider** section breaks down cost by API provider. This
pipeline is all Anthropic, so it's a single line -- but in a multi-provider
pipeline you'd see OpenAI, Google, and others listed separately. Useful for
knowing which vendor bill is going to grow.

The **Top Cost Drivers** section ranks each node by its maximum
cost. `Refactor` leads at $1.46 max, followed by `Triage`
and `QualityGate`. Notice `RemeasureCognitive` at the
bottom: $0.00, with no provider listed. That's a tool node -- more on that
in a moment.

## <span class="step-num">2</span> How Estimation Works

The cost estimator counts your prompt tokens (the text in your `prompt:`
block maps to input tokens), applies a heuristic multiplier for expected output
tokens, and multiplies by the model's per-token pricing. It's an estimate, not a
bill -- but it's in the right ballpark. The model catalog and pricing live in
`validator/lint_model.go` and `cost/pricing.go`, verified
against official provider documentation. Stale pricing is a lint error.

One practical consequence: longer prompts cost more, and the estimator sees
that directly from your `.dip` file. If a node's prompt block is
500 words, the estimator counts those tokens before you spend a cent.

## <span class="step-num">3</span> The max_turns Multiplier

Nodes inside retry loops get their cost multiplied by the retry count. Look at
the `Refactor` node: it shows $1.46 max because it's inside a retry
loop -- each attempt costs roughly the same, and the max reflects all of them
running. The expected cost assumes the node succeeds on the first try.

This is why max and expected can diverge significantly. A pipeline with a
well-behaved happy path might cost $0.65 expected, but if every retry fires
on every node, you're looking at $2.66. The cost report makes that spread
explicit so you can decide whether the retry budget is acceptable before
committing to a model choice.

## <span class="step-num">4</span> Free Nodes

Tool nodes and human nodes cost $0 -- they don't make LLM calls. You'll see
them in the cost breakdown with `$0.00` and no provider listed.
Don't worry about them. `RemeasureCognitive` runs a shell command
to re-check cyclomatic complexity; it shows up in the report for completeness,
but it contributes nothing to your API bill.

## <span class="step-num">5</span> dippin optimize

Once you know what a pipeline costs, `dippin optimize` tells you
where you could spend less:

<pre>
<span class="hl-dim">$</span> <span class="hl-shcmd">dippin optimize complexity_cleanup.dip</span>
&#x2550;&#x2550;&#x2550; Optimization Report &#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;&#x2550;
&#x2500;&#x2500;&#x2500; Cost Summary &#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;
  Current:   $0.65 (expected)
  Optimized: $0.02 (expected)
  Savings:   $0.63 (expected)

&#x2500;&#x2500;&#x2500; Suggestions &#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;
  [Refactor] node in retry loop &#x2014; consider a cheaper model for mechanical iterations
    claude-opus-4-6 &#x2192; claude-haiku-4-5  (saves ~$0.23)
  [Refactor] bookkeeping task (summary/cleanup/commit) can use a cheaper model
    claude-opus-4-6 &#x2192; claude-haiku-4-5  (saves ~$0.23)
  [QualityGate] bookkeeping task (summary/cleanup/commit) can use a cheaper model
    claude-opus-4-6 &#x2192; claude-haiku-4-5  (saves ~$0.12)
  [Done] bookkeeping task (summary/cleanup/commit) can use a cheaper model
    claude-sonnet-4-6 &#x2192; claude-haiku-4-5  (saves ~$0.04)
</pre>

The Cost Summary at the top shows where you'd land if you followed every
suggestion: $0.65 expected down to $0.02 -- a 97% reduction. That's an
aggressive projection, but it shows the ceiling on savings.

The Suggestions section lists each node with a specific swap. The optimizer
looks at each node's role in the pipeline -- whether it's in a retry loop,
whether it's doing bookkeeping work like summarization or cleanup -- and
suggests where a cheaper model would do the same job. For
`Refactor`, it flags two reasons to downgrade: it's in a retry
loop (mechanical repetition rarely needs the smartest model) and its task
is categorized as bookkeeping.

These are suggestions, not commands. A quality gate might genuinely need a
stronger model to make good decisions. Use your judgment -- the optimizer
doesn't know your quality bar, only your cost structure.

## What's Next

Between conditional edges and cost estimation, you can build pipelines that
route intelligently and stay within budget. Write the workflow, run
`dippin cost`, check `dippin optimize`, then make
deliberate choices about where quality justifies the price.
