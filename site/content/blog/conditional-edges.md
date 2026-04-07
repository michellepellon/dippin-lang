---
title: "Conditional Edges: Routing Pipelines with when"
date: "2026-03-31"
description: "Build branching AI pipelines that route based on LLM output. Learn Dippin's condition syntax, operators, and exhaustive detection."
tagStyle: "tutorial"
tagLabel: "TUTORIAL"
category: "Authoring"
readTime: "10 min read"
related:
  - url: "cost-estimation.html"
    title: "Cost Estimation"
    summary: "Estimate per-run pipeline costs before spending real money on LLM calls."
  - url: "multi-line-prompts.html"
    title: "Multi-line Prompts"
    summary: "The #1 reason Dippin exists -- write prompts without escaping."
---

In the [last post](multi-line-prompts.html) we showed how Dippin handles
prompts. Now let's make pipelines that think. Linear workflows are fine for simple
tasks, but real AI pipelines need to branch: retry on failure, escalate when something
looks wrong, skip steps that don't apply. Dippin's `when` keyword is how
you express all of that.

## <span class="step-num">1</span> The Linear Baseline

Let's start with a document review pipeline. It drafts, reviews, and publishes -- in
that order, every time, with no branching. Here's the whole thing:

<pre>
<span class="hl-kw">workflow</span> <span class="hl-node">ReviewPipeline</span>
  <span class="hl-field">goal</span>: <span class="hl-str">"Draft, review, and publish a document"</span>
  <span class="hl-field">start</span>: Start
  <span class="hl-field">exit</span>: Exit

  <span class="hl-kw">defaults</span>
    <span class="hl-field">provider</span>: anthropic
    <span class="hl-field">model</span>: claude-sonnet-4-6

  <span class="hl-kw">agent</span> <span class="hl-node">Start</span>
    <span class="hl-field">label</span>: Start

  <span class="hl-kw">agent</span> <span class="hl-node">Exit</span>
    <span class="hl-field">label</span>: Exit

  <span class="hl-kw">agent</span> <span class="hl-node">Draft</span>
    <span class="hl-field">label</span>: <span class="hl-str">"Write Draft"</span>
    <span class="hl-field">prompt</span>:
      Write a clear, concise technical document based on the
      provided requirements. Focus on accuracy and readability.

  <span class="hl-kw">agent</span> <span class="hl-node">Review</span>
    <span class="hl-field">label</span>: <span class="hl-str">"Review Draft"</span>
    <span class="hl-field">auto_status</span>: true
    <span class="hl-field">prompt</span>:
      Review the draft for technical accuracy, clarity, and
      completeness. Return success if the draft meets standards,
      or fail with specific feedback.

  <span class="hl-kw">agent</span> <span class="hl-node">Publish</span>
    <span class="hl-field">label</span>: Publish
    <span class="hl-field">prompt</span>:
      Format the approved draft for publication.

  <span class="hl-kw">edges</span>
    Start -> Draft
    Draft -> Review
    Review -> Publish
    Publish -> Exit
</pre>

This works, but every draft gets published regardless of the review. The
`Review` node dutifully critiques the draft, but we ignore its verdict
and publish anyway. We need branching.

## <span class="step-num">2</span> Basic Conditions

Add `when` clauses to the edges out of
`Review`. The node definitions stay exactly the same -- only the
`edges` block changes:

<pre>
  <span class="hl-kw">edges</span>
    Start -> Draft
    Draft -> Review
    Review -> Publish  <span class="hl-cond">when</span> ctx.outcome = success
    Review -> Draft    <span class="hl-cond">when</span> ctx.outcome = fail
    Publish -> Exit
</pre>

Lint it:

<pre>
<span class="hl-dim">$</span> <span class="hl-shcmd">dippin lint pipeline.dip</span>
<span class="hl-pass">PASS</span>  pipeline.dip  <span class="hl-dim">(0 errors, 0 warnings)</span>
</pre>

Two things are happening here. First, `auto_status: true` on the
`Review` node tells Dippin that the LLM will set
`ctx.outcome` based on the content of its response -- it parses the
output for `success` or `fail` and writes it into the
pipeline context automatically. Second, the edges now form a retry loop: a
failed review sends execution back to `Draft`, which writes a new
version, which goes back to `Review`, until the review passes and
the pipeline proceeds to `Publish`.

## <span class="step-num">3</span> Operators

`success`/`fail` is the most common pattern, but conditions
support a full set of string operators:

<table>
  <thead>
    <tr><th>Operator</th><th>Example</th></tr>
  </thead>
  <tbody>
    <tr>
      <td><code>=</code> / <code>==</code></td>
      <td><code><span class="hl-cond">when</span> ctx.outcome = success</code></td>
    </tr>
    <tr>
      <td><code>!=</code></td>
      <td><code><span class="hl-cond">when</span> ctx.outcome != success</code></td>
    </tr>
    <tr>
      <td><code>contains</code></td>
      <td><code><span class="hl-cond">when</span> ctx.feedback contains "security"</code></td>
    </tr>
    <tr>
      <td><code>not contains</code></td>
      <td><code><span class="hl-cond">when</span> ctx.feedback not contains "security"</code></td>
    </tr>
    <tr>
      <td><code>startswith</code></td>
      <td><code><span class="hl-cond">when</span> ctx.category startswith "urgent"</code></td>
    </tr>
    <tr>
      <td><code>endswith</code></td>
      <td><code><span class="hl-cond">when</span> ctx.filename endswith ".go"</code></td>
    </tr>
    <tr>
      <td><code>in</code></td>
      <td><code><span class="hl-cond">when</span> ctx.priority in high,critical</code></td>
    </tr>
  </tbody>
</table>

All operators work on string values. The left side is always a context variable,
the right side is a literal. Comparisons are case-sensitive. The `in`
operator matches against a comma-separated list of values with no spaces around
the commas.

## <span class="step-num">4</span> Compound Conditions

Sometimes a single condition isn't enough. Suppose failed reviews that mention
security issues need to go to a dedicated escalation node, while ordinary failures
just loop back to drafting:

<pre>
    Review -> Escalate  <span class="hl-cond">when</span> ctx.outcome = fail and ctx.feedback contains "security"
    Review -> Draft     <span class="hl-cond">when</span> ctx.outcome = fail and ctx.feedback not contains "security"
</pre>

`and` and `or` compose conditions in the expected way. Use
parentheses to control precedence when mixing them -- without parens,
`and` binds tighter than `or`:

<pre>
    <span class="hl-cond">when</span> (ctx.outcome = fail and ctx.severity = high) or ctx.override = true
</pre>

This routes to the target if the outcome is a high-severity failure, or if an
override flag was explicitly set -- whichever comes first.

## <span class="step-num">5</span> Exhaustive Detection

Dippin tracks whether a node's outgoing edges cover all possible outcomes. If they
don't, `dippin lint` will tell you. This is one of the most useful
things the linter does -- a missing branch is a silent runtime failure waiting
to happen.

There are three patterns to know. The first is an unconditional fallback -- one
edge has a condition, the other doesn't. The unconditional edge fires when no
condition matches:

<pre>
    Review -> Publish  <span class="hl-cond">when</span> ctx.outcome = success
    Review -> Exit
</pre>

<pre>
<span class="hl-dim">$</span> <span class="hl-shcmd">dippin lint pipeline.dip</span>
<span class="hl-pass">PASS</span>  pipeline.dip  <span class="hl-dim">(0 errors, 0 warnings)</span>
</pre>

The second pattern is a single conditional edge with no fallback. If
`ctx.outcome` is never `success`, execution has nowhere
to go:

<pre>
    Review -> Publish  <span class="hl-cond">when</span> ctx.outcome = success
</pre>

<pre>
<span class="hl-dim">$</span> <span class="hl-shcmd">dippin lint pipeline.dip</span>
<span class="hl-warn">WARN</span>  pipeline.dip
  <span class="hl-warn">DIP101</span>: node "Review" has conditional edges but no unconditional fallback
</pre>

The third pattern is a complementary pair. When both branches of a
`success`/`fail` split are present, Dippin recognizes
them as exhaustive and the warning is suppressed -- this is exactly the pattern
from Section 2:

<pre>
    Review -> Publish  <span class="hl-cond">when</span> ctx.outcome = success
    Review -> Draft    <span class="hl-cond">when</span> ctx.outcome = fail
</pre>

<pre>
<span class="hl-dim">$</span> <span class="hl-shcmd">dippin lint pipeline.dip</span>
<span class="hl-pass">PASS</span>  pipeline.dip  <span class="hl-dim">(0 errors, 0 warnings)</span>
</pre>

Dippin recognizes two kinds of exhaustive pairs: `success`/`fail`
and `contains X`/`not contains X` for the same variable and
value. Any other combination still needs an unconditional fallback to satisfy the linter.

## <span class="step-num">6</span> The `ctx.` Prefix

Context variables must be namespace-prefixed. If you write a condition without one,
the linter catches it:

<pre>
    Review -> Publish  <span class="hl-cond">when</span> outcome = success
</pre>

<pre>
<span class="hl-dim">$</span> <span class="hl-shcmd">dippin lint pipeline.dip</span>
<span class="hl-warn">WARN</span>  pipeline.dip
  <span class="hl-warn">DIP120</span>: condition references "outcome" without namespace prefix — did you mean "ctx.outcome"?
</pre>

The fix is one prefix away: change `outcome` to `ctx.outcome`.
Dippin supports three namespaces in conditions: `ctx` for runtime context
set by node execution, `graph` for pipeline-level metadata, and
`params` for values passed in at invocation time. Most conditions you write
day-to-day will use `ctx`.

## What's Next

You've got pipelines that branch. But what do those branches cost? Each path through
a conditional pipeline has a different token count and a different price tag. The next
post shows how to estimate it before spending real money on LLM calls.
