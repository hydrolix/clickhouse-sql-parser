## Context

The lexer in `parser/lexer.go` already has two relevant entry points:

- `consumeToken` switches on the next byte and dispatches to a kind-specific consumer (`consumeIdent`, `consumeString`, `consumeNumber`, …).
- `consumeString` is currently the single-quote string scanner: it walks from `'…'` and produces a `TokenKindString` token whose `String` field contains the inner bytes.

For the dollar-quoted form, the byte after the leading `$$` and the byte before the trailing `$$` belong to the content. The semantics are deliberately permissive — anything between the markers is literal — so the scanner is simpler than the single-quote scanner (no escape handling, no doubling rule, just "find the next `$$`").

The trick is dispatch: a single `$` already routes to `consumeIdent` (where bare `$ident` and braced `${name}` placeholders are produced). The lexer needs to distinguish `$$` (start of text block) from `$ident` (variable / identifier) without breaking either path.

## Goals / Non-Goals

**Goals:**
- `$$ … $$` lexes as a single `TokenKindString` whose `String` field is the verbatim content between the markers.
- Single `$` continues to behave exactly as today — Grafana variables (`${name}`), format suffixes (`${name:format}`), and bare `$ident` are unaffected.
- An unterminated `$$…` produces the same `invalid string` error already used for unterminated `'…'`.

**Non-Goals:**
- Tagged dollar-quoting (`$tag$ … $tag$`). The fork-level reference and the existing test contract only cover the untagged form, and tagged quoting introduces additional grammar (the tag must match) that is not justified by any current use case.
- A separate token kind. Dollar-quoted strings collapse onto the existing `TokenKindString`; downstream code already accepts string tokens wherever they are valid.
- Any handling of `$$` inside another string. By definition `consumeString` runs once it has begun, and it does not re-enter the `consumeToken` dispatch.
- AST or formatter changes. The token is a string; the AST already represents string literals.

## Decisions

### Decision 1: Two-byte lookahead at dispatch time, not inside `consumeIdent`

`consumeToken`'s `case '$'` is the right place to disambiguate `$$` from `$ident`. The lookahead is two bytes: if `peekN(1) == '$'`, dispatch to `consumeString` (which now handles both `'…'` and `$$…$$`). Otherwise dispatch to `consumeIdent` as before.

**Why:** Keeps the two consumers cleanly separated. `consumeIdent` does not need to know about `$$`, and `consumeString` only needs to recognise that it might be entered for a text block. The branching is a single `if` at the top of `consumeString`, not a tangled conditional inside `consumeIdent`.

**Alternative considered:** Hand the `$$` recognition to `consumeIdent` and have it bail out if it sees a second `$`. Rejected — `consumeIdent`'s control flow is already nontrivial because of `${…}` and `:format` handling, and adding a "actually, this is a string" exit path would obscure both responsibilities.

### Decision 2: `consumeString` carries a `start` offset and an `isTextBlock` flag

`consumeString` currently uses `i := 1` (skip the opening `'`) and scans for the matching `'`. The change generalises this to:

- **Detect the opener.** If `peekN(0) == '$' && peekN(1) == '$'`, set `start := 2` and `isTextBlock := true`. Otherwise leave `start := 1` and `isTextBlock := false` (the existing single-quote path).
- **Initialise the scan index from `start`.** `i := start`. At this point, `i` is the index of the first content byte just past the opening marker.
- **Walk the content.** Inside the loop, `i` advances byte-by-byte. The terminator check at each step is:
  - When `isTextBlock`: break if `peekN(i) == '$' && peekN(i+1) == '$'`. On break, `i` is the **index of the first `$` of the closing marker**.
  - Otherwise: break if `peekN(i) == '\''`. On break, `i` is the **index of the closing `'`**.
  - If `peekOk(i)` becomes false before a terminator is seen, return `errors.New("invalid string")`.
- **Slice and advance using `start`.** Since the closing marker has the same length as the opening marker (`start` bytes) in both shapes — one byte for `'…'`, two bytes for `$$…$$` — the content slice is `l.slice(start, i)` and the advance amount is `l.skipN(i + start)`. The `i + start` arithmetic is **not** coincidental: it relies on opener-length equalling closer-length, which is the invariant Decision 2 is built around.

**Why:** Symmetry between opener and closer. Both `'…'` and `$$…$$` have an opener-length matching their closer-length, so a single offset variable lets the same arithmetic carry both shapes. The `i` cursor lands on the first byte of the closing marker (one byte before the past-the-end position for `'…'`, two bytes before for `$$…$$`), and `start` doubles as both the offset into content and the number of trailing-marker bytes to skip. No duplicate "exit" code.

### Decision 3: No escape handling inside text blocks

Inside `$$…$$`, every byte is literal. No `\n`, no `''` doubling, no `\\` collapsing. The only thing the scanner checks for is the closing `$$`.

**Why:** That is precisely the value proposition of dollar-quoted strings — they let authors embed text containing any of the characters that would otherwise need escaping. Adding escape handling would defeat the purpose.

**Implication for the test fixture `$$${variable:format} and 'string' $$`:** the inner `${variable:format}` is plain bytes in the string content; it does NOT trigger `consumeIdent`'s brace-variable logic. The token's `String` field literally contains the substring "`${variable:format} and 'string' `" (with the trailing space).

### Decision 4: Unterminated `$$` shares the existing `invalid string` error

When the scanner walks off the end of input without finding a closing `$$`, `consumeString` returns `errors.New("invalid string")` — the same wording already used for unterminated `'…'`.

**Why:** Callers that classify lexer errors should not need a separate code path for unterminated text blocks. The user-facing message is generic enough to cover both.

## Risks / Trade-offs

- **Risk: A query containing `$ $` (single `$`, space, single `$`) gets mis-lexed as a text block.** Cannot happen. The opener requires `peekN(0) == '$' && peekN(1) == '$'` — two adjacent `$`s with no intervening byte. A `$ $` sequence has a space between them and lexes as two separate tokens.
- **Risk: A `${var}` placeholder that contains a literal `$$` substring is misread.** Cannot happen either. The brace-variable path runs entirely inside `consumeIdent`, and `consumeIdent` is only entered when the dispatch already decided the input is not `$$…`. Once inside `consumeIdent`, any `$` characters within `${…}` are normal ident bytes.
- **Risk: A `$$` opener immediately followed by another `$$` (empty text block) returns an empty string.** That is the documented behaviour and matches every other empty literal (`''` produces an empty string too). No issue.
- **Trade-off: This change is permissive about content.** A typo where the author meant `$$ end-marker` but actually wrote a stray `$$` somewhere in the middle of a long block silently truncates the literal. This matches every other "until-marker" syntax in SQL (single quotes, double quotes, backticks, block comments). Acceptable.
- **Trade-off: Untagged-only.** A user who writes `$tag$ … $tag$` (Postgres-tagged form) still fails to lex. If that ever surfaces as a real need, the design here is straightforwardly extensible: replace the two-byte `$$` lookahead with a `\$[A-Za-z_]*\$` lookahead and remember the captured tag.

## Migration Plan

Single commit, no dependencies, no migration of data or config. Rollback is `git revert`.
