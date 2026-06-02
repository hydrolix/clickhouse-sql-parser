## Context

`func (l *Lexer) consumeToken()` in `parser/lexer.go` dispatches on the lookahead byte. Three of the existing arms already use a save/restore pattern to peek at potential multi-character operators without committing — see the `*`/`/` arm at line ~352 that does:

```go
savedState := l.saveState()
_ = l.consumeToken()
token := l.lastToken
l.restoreState(savedState)
```

So the snapshot primitives already exist as public-on-the-package methods on `*Lexer`: `saveState() lexerState` and `restoreState(state lexerState)`. The `lexerState` type is embedded in `*Lexer` and carries the byte cursor (`current`) plus the last-token slot — exactly what we need to rewind around a failed number scan.

The digit case is the only call site where we need this rewind. Numbers that scan successfully should NOT pay any cost; only the failure path takes the second tokenisation pass.

## Goals / Non-Goals

**Goals:**
- A digit-prefixed input that fails to scan as a number SHALL re-scan as an identifier and produce a `TokenKindIdent` token whose `String` field is the verbatim input.
- A digit-prefixed input that scans successfully as a number SHALL produce the same `TokenKindInt` or `TokenKindFloat` token it produces today. No behaviour change for valid numbers.
- The lexer's public API stays the same. No new methods, no signature changes.

**Non-Goals:**
- Resilience for non-digit-prefixed inputs. Other lexer arms continue to return their existing errors. Only the `'0'…'9'` arm changes.
- A new `TokenKindUnknown` or `TokenKindGarbage`. The fallback rescans as an identifier and produces a normal `TokenKindIdent`.
- Resilience to inputs that even `consumeIdent` rejects (e.g. pure punctuation that starts with a digit but has no ident bytes at all — there are no such inputs in practice, since the digit itself is an ident-part).
- A symmetrical change for the `'+'`/`'-'` arm which dispatches to `consumeNumber` when followed by a digit. That arm is operator-context-dependent and is out of scope.

## Decisions

### Decision 1: Reuse the existing `saveState` / `restoreState` primitives

`*Lexer` already carries an embedded `lexerState` value with two snapshot helpers (`saveState`, `restoreState`). The digit-case wrap uses them directly:

```go
case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
    savedState := l.saveState()
    if err := l.consumeNumber(); err != nil {
        l.restoreState(savedState)
        return l.consumeIdent(Pos(l.current))
    }
    return nil
```

**Why:** No new state-shape decisions, no risk of forgetting a field during snapshot. The helpers are already exercised by another dispatch arm in the same function, so we know they correctly capture and restore everything that downstream code reads.

**Alternative considered:** A dedicated `*Lexer.tryConsumeNumber` helper that returns `(token, ok)` and never errors. Rejected — it duplicates `consumeNumber`'s knowledge of valid number shapes, and the save/restore approach keeps `consumeNumber` as the sole authority on what is and isn't a number.

### Decision 2: Restore-then-redispatch, not restore-then-mutate-token

After `consumeNumber()` errors, the snapshot fully rolls back the cursor (and any partial `lastToken` state). The call to `consumeIdent(Pos(l.current))` then re-scans from the original position. `consumeIdent` already handles arbitrary identifier shapes including leading digits (its loop is just "while next byte is an ident-part, advance"), so no special-casing inside `consumeIdent` is needed.

**Why:** Keeps responsibilities clean. `consumeNumber` is the number authority; `consumeIdent` is the identifier authority. The fallback is one if-statement away — no shared state, no implicit coupling.

### Decision 3: Two `TestConsumeNumber` sub-tests are removed, not rewritten

The sub-tests `Invalid number` and `Invalid float number` (currently passing) assert that `consumeToken` returns an error on inputs like `"123e"`, `"0xg"`, `"123.456b"`. After this change, those inputs no longer error — they tokenise as identifiers. The sub-tests cannot be kept as-is.

We have two options. Option A: rewrite them to assert that the result is an identifier (i.e., `Kind == TokenKindIdent`). Option B: remove them entirely, since three already-present `Fork: …` sub-tests under the same `TestConsumeNumber` already exercise the SAME inputs with the new expectations.

**Decision:** Option B — remove. Two sub-tests with two opposite assertions on the same inputs would be confusing forever after.

**Why not keep them as deprecated documentation?** The Fork sub-tests are the documentation. There is no value to retaining the obsolete sub-tests — they describe behaviour that no longer exists.

### Decision 4: The `"00e1d"` entry in `TestParser_InvalidSyntax` is removed

That entry was added to guard against a parser panic when the lexer returned an error with `lastToken == nil`. After this change the lexer never returns a nil-lastToken error for `"00e1d"` — it produces a valid `TokenKindIdent` token instead, and the parser accepts the input as a bare identifier statement (verified during the propose phase by probing the parser with `"foo"` and `"abc123"`, both of which parse without error).

The original panic-guard concern is no longer applicable: the panic was reachable only via the nil-lastToken-error path, which this change closes off entirely.

**Why not replace with a new "invalid syntax" probe?** Any new probe would have to construct an input the lexer can't recover from, but the lexer's other error paths (`unclosed quoted identifier`, `invalid string`, etc.) are already exercised by their own targeted tests. Adding another entry would be redundant.

## Risks / Trade-offs

- **Risk: A user types a typo'd number (`123e`) and gets a confusing downstream error.** Before this change: lexer error `exponent part should contain at least one digit`. After this change: the token tokenises as an identifier and the parser may produce something like `unexpected token: 123e`. **Mitigation:** The downstream error path is the same one users hit when they type any unrecognised identifier — it's not worse, just less specific. Lenient tokenisation is the typical industry trade-off, and the bare-typo case is rare compared to legitimate identifier-shaped inputs that today don't lex.
- **Risk: A subtly invalid hex literal (`0xg`) was previously rejected at lex time; now it's accepted as the identifier `0xg`.** Same mitigation as above — the parser will reject `0xg` as a bare token in any meaningful context, just later in the pipeline.
- **Risk: The save/restore primitives don't capture some piece of state we don't know about.** Mitigated by the fact that they are already in production use in the same function for the `*`/`/` arm. If they were leaky we would already have bugs there.
- **Trade-off: Two existing sub-tests are removed.** This is documented above. Anyone reading the diff sees the rationale in the commit message and the spec.
- **Trade-off: One existing parser-test entry is removed.** Same.

## Migration Plan

Single commit, no dependencies, no data or config involvement. Rollback is `git revert`.
