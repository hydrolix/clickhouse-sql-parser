## Context

`func (l *Lexer) skipComments()` in `parser/lexer.go` is a small loop that, while at the head of the input stream, repeatedly: skips whitespace, peeks at the next byte, and dispatches on it. The arm for `'-'` checks whether the next byte is also `'-'` and, if so, calls `consumeSingleLineComment` and continues the loop; the arm for `'/'` does the same for `/*`. Everything else falls through and `skipComments` returns.

`consumeSingleLineComment` already implements exactly the "everything up to the next newline (or end-of-input)" behaviour. It is shape-agnostic — it does not care which character started the comment, because the caller has already consumed (or peeked past) the marker. That makes adding a `#` arm a one-case addition with no helper churn.

## Goals / Non-Goals

**Goals:**
- `#` at the head of the lookahead stream initiates a single-line comment that runs to the next newline (or EOF).
- The lexer surface is unchanged for every input that does not contain `#`. No new token kind, no public API change.

**Non-Goals:**
- Special handling of `#` inside string literals, identifiers, or comments — the existing tokenisation already isolates those contexts before `skipComments` sees them, so the `#` arm cannot mistakenly trigger inside them.
- Multi-line block comments started by `#`. ClickHouse only treats `#` as a single-line comment terminator.
- A "shell-style comment" capability covering `#!` shebangs or any other directive. Out of scope; only the plain `#` line comment is in scope here.

## Decisions

### Decision 1: Reuse `consumeSingleLineComment` rather than introduce a hash-specific helper

The `--` arm dispatches to `consumeSingleLineComment`; the `#` arm will do the same. The helper does not encode a marker length, so a single call with no arguments works for both.

**Why:** No reason to fork the helper. A `#` comment and a `--` comment have identical termination semantics (next newline or EOF), and divergent helpers would risk drift.

**Alternative considered:** A second helper `consumeHashComment` that hard-codes the leading character. Rejected — pure duplication without benefit.

### Decision 2: Place the `#` arm next to the `--` arm

The new `case '#':` arm goes in the same switch immediately adjacent to the `-` arm, before the `/` arm. The order does not affect correctness — switch arms are disjoint by leading byte — but adjacency to `--` keeps the comment-related arms grouped.

**Why:** Readability. Reviewers looking for "where are line comments handled" find both forms side by side.

### Decision 3: No grammar-level change

The lexer's `skipComments` is called before every token is emitted. Comments never become tokens. The parser therefore sees no change, the AST shape does not move, and the formatter — which renders from AST, not from token stream — is structurally incapable of being affected.

**Why this is worth noting:** It scopes review to a single function. There is no follow-up parser change, no AST node update, no formatter update needed for this work.

## Risks / Trade-offs

- **Risk: A future user has a database/table named with a leading `#` and that name appears unquoted.** ClickHouse does not allow unquoted identifiers to start with `#`, so this cannot happen in valid SQL. Quoted identifiers (backticks or double quotes) are handled by `consumeIdent`, which runs after `skipComments` returns — and quoted identifiers do not contain the leading quote until `consumeIdent` strips it. **Mitigation:** None needed; the existing tokenisation flow already isolates these contexts.
- **Risk: A `#` inside a string literal gets eaten.** Cannot happen. `skipComments` is called at the **head** of the input, between tokens, never while scanning a string. By the time the lexer dispatches into `consumeString` it has bypassed `skipComments`, and `consumeString` does not call back into it.
- **Trade-off: This change is permissive.** Anything between `#` and the next newline is silently dropped. If a user typo-writes `# instead of -- and means it as actual SQL, they get a silent skip rather than a parse error.** This matches the existing behaviour for `--` and is the standard SQL convention. Acceptable.

## Migration Plan

Single commit, no dependencies, no data or config changes. Rollback is `git revert`.
