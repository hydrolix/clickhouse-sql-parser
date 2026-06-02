## Context

`clickhouse-sql-parser` parses standard ClickHouse SQL. It does not currently recognise Grafana template variables, which appear in queries authored against Grafana dashboards before the templating layer substitutes them. The most common shapes are:

- `${name}` — a placeholder for a value (e.g. `${tbl}` as a table name, `${ts}` inside a function argument).
- `${name:format}` — a placeholder with a Grafana format suffix (e.g. `${y:sqlstring}` requests SQL-quoted substitution).
- Bare `$ident` (no braces) — already parses today as a regular identifier and must continue to.
- `${VAR}` between operands — used by Grafana's `$__conditionalAll` and similar patterns to template entire operators (e.g. `statusCode ${AND_statusCode} (1, 2)`).

These placeholders appear at every position where ClickHouse SQL accepts an identifier or an expression: select-list items, `FROM` targets, function-call arguments, `WHERE` operands, and `SETTINGS` values. The parser must accept each of these without modifying the AST shape, so the existing formatter, visitor interface, JSON marshalling, and golden-file fixtures continue to work unchanged.

A set of bare per-feature tests already exists in `parser/parser_test.go` defining the exact contract: one test per parser entry point that needs to accept variables, plus regression guards. The implementation's job is to make those tests pass without disturbing anything else.

## Goals / Non-Goals

**Goals:**
- Lex `${name}` and `${name:format}` as a single `TokenKindIdent` whose `String` field is the verbatim source text (including the braces and the format suffix).
- Allow that token to flow through every parser entry point where a regular identifier is valid.
- Allow `${VAR}` to act as an infix operator placeholder between two operands.
- Keep AST, formatter, visitor, and golden-file behaviour unchanged for every input that does not contain a variable.
- Keep `EXTRACT(unit FROM expr)` working — it is an existing special-form expression that this change must not regress.

**Non-Goals:**
- `$$ … $$` text blocks (a related but distinct lexer feature, separate change).
- Variable resolution or substitution. This is a parser change; consuming the parsed variable is the caller's problem.
- A dedicated AST node type for variables. Variables surface as ordinary identifier-shaped expressions.
- Variables on the **key** side of a `SETTINGS` entry. Only the value side is in scope.

## Decisions

### Decision 1: Variables surface as ordinary identifier tokens

The lexer extends `consumeIdent` so that a `$` followed by `{` enters "variable" mode, reads identifier characters and at most one `:format` suffix, and requires a closing `}`. The resulting token is `TokenKindIdent`, and its `String` field carries the literal `${name}` or `${name:format}` source text. No new `TokenKind` is introduced.

**Why:** This is the smallest viable change. The parser already accepts any `TokenKindIdent` in every position we care about, so once the lexer produces the token, almost every parse path already routes it correctly. No new switch arms in the formatter, no visitor interface change, no AST node, no breaking change to golden files. Downstream consumers that care which identifiers are variables can do a `strings.HasPrefix(name, "$")` check.

**Alternative considered:** A dedicated `TokenKindVariable` plus a `VariableExpr` AST node. Rejected — it would force changes in every site that handles identifiers (every `FormatSQL`, the visitor interface, every test golden) for no caller-visible win. A structured node could be added later as an additive change if a real consumer needs it.

### Decision 2: A single `matchVariable()` helper centralises the predicate

A small helper on `*Parser`:

```go
func (p *Parser) matchVariable() bool {
    return p.matchTokenKind(TokenKindIdent) && strings.HasPrefix(p.last().String, "$")
}
```

Three parser callsites use it: `getNextPrecedence`, `parseInfix`, and `parseSettingsExprList`.

**Why:** Three callsites is enough to justify a helper, and it documents intent at each callsite ("this slot accepts a variable"). Inline duplication of the check would be a minor maintenance hazard.

### Decision 3: Variables as infix operators get a dedicated precedence slot

`parser_column.go` defines a precedence ladder (`PrecedenceUnknown`, `PrecedenceOr`, `PrecedenceAnd`, comparison, `+`/`-`, `*`/`/`/`%`, …). A new `PrecedenceIndent` slot is inserted between `PrecedenceUnknown` and `PrecedenceOr` — one step above unknown, one below `OR`. When the parser sees a variable between two operands, it consumes that variable as a binary operator at this precedence.

**Why this precedence:** When Grafana later substitutes the variable, the substituted operator could be anything — `AND`, `IN`, `=`, `LIKE`, etc. Putting the variable-operator just above `PrecedenceUnknown` means the variable binds **looser** than every real operator that could replace it, so the parse tree groups `(left) variable (right)` as a single binary expression regardless of what real operator gets substituted in. Putting it any higher would mis-group templates whose runtime operator is low-precedence (e.g. `AND`).

**Trade-off:** A SQL author writing `a ${X} b AND c ${Y} d` will get specific grouping that depends on how the parser walks. Acceptable — these are templates, not arithmetic, and the actual operator structure is decided at substitution time anyway.

### Decision 4: `$var` allowed as a `SETTINGS` value only, not as a key

`parseSettingsExprList` walks `name = value` pairs. The value side gets a new `case p.matchVariable():` arm. The name side is unchanged.

**Why:** All observed Grafana queries put the variable on the value side (`max_threads = ${threads}`). Allowing it on the key side would create grammar ambiguity with the existing settings syntax for no known use case.

### Decision 5: Explicit error wording for unclosed variables

When the lexer encounters `${` and reaches end-of-input before a closing `}`, it returns `fmt.Errorf("unclosed variable: %s", l.slice(0, i))`. The `unclosed variable:` prefix is part of the public error contract.

**Why:** Caller code that classifies parse errors by message prefix benefits from a stable, descriptive wording.

### Decision 6: Bare-feature tests are the contract

The contract for this change is a set of bare per-feature tests already present in `parser/parser_test.go`, each exercising exactly one parser entry point that needs variable support:

- `TestParser_Var_TopLevel` — `${var}` as an expression operand.
- `TestParser_Var_InFromClause` — `${var}` as a table name.
- `TestParser_Var_FormatSuffix` — `${var:format}`.
- `TestParser_Var_InFunctionArg` — `${var}` inside a function-call argument list.
- `TestParser_Var_AsInfixOperator` — `${VAR}` between two operands.
- `TestParser_With_VariableInSettings` — `${var}` as a `SETTINGS` value.

Plus two regression guards that must remain green:

- `TestParser_Var_BareDollarIdent` — bare `$ident` already parses today, must keep parsing.
- `TestParser_ExtractStillParses` — `EXTRACT(unit FROM expr)` already parses today, must keep parsing.

All six feature tests FAIL on the unchanged codebase; all assertions in the regression tests pass on the unchanged codebase. The implementation is correct when the six FAILs flip to PASS without disturbing either guard.

**Why this contract over a separate verification doc:** Bake the requirements into runnable tests so verification is mechanical and CI-friendly. If a future regression breaks one of these behaviours, the failing test name points at the exact feature involved.

## Risks / Trade-offs

- **Risk: Existing golden files shift.** The formatter should emit `${name}` literally, so no golden update is expected. If a golden does change, that indicates an unintended rendering change and must be investigated rather than accepted with `-update`. **Mitigation:** Tasks call out running `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify` as gates and forbid `-update`.
- **Risk: Allowing `:` inside `${…}` collides with ClickHouse's `::` cast operator.** **Mitigation:** The `:` is allowed only **inside** the braces. Once the closing `}` is consumed, the lexer returns to normal mode where `::` continues to lex as `TokenKindDash` per the existing constant. No collision is possible because the brace-mode and out-of-brace mode are mutually exclusive.
- **Risk: Variable-as-operator precedence picks the wrong slot for some unanticipated Grafana template, causing mis-grouping.** **Mitigation:** Decision 3's rationale (looser-than-any-real-operator) covers every operator Grafana could substitute. If a real-world template surfaces a mis-grouping, the fix is a one-constant edit in a follow-up change.
- **Trade-off: No dedicated `VariableExpr` AST node.** Consumers wanting to recognise variables must do a `strings.HasPrefix(name, "$")` check. Acceptable for now; a structured node can be added later as an additive change.
- **Trade-off: `EXTRACT(unit FROM expr)` continues to be parsed by a dedicated arm in `parseColumnExpr` rather than being delegated to generic function-call parsing.** This is preserved deliberately — refactoring `EXTRACT` is out of scope and would risk regressing existing golden tests.

## Migration Plan

This is an additive parser change with no data, schema, or config involvement. The implementation should land in a single commit (or a tight series of commits) and be reviewable file-by-file. Rollback is `git revert`.
