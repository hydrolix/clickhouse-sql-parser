# Tasks: SETTINGS After FORMAT, and Two Upstream Expression Fixes

## 1. Baseline

- [x] 1.1 Record the current `go test ./... -count=1` result on `main`.
- [x] 1.2 Using `clickhouse-local` (see `AGENTS.md`), check that ClickHouse accepts
      `SELECT 1 FORMAT JSON SETTINGS max_threads = 1`. Also check what it does with
      `SELECT 1 SETTINGS a = 1 FORMAT JSON SETTINGS b = 2`: accept or reject, and which
      value wins. Record the result in the `FormatSettings` field comment and, if it
      differs, in the "Both positions are kept" scenario.

## 2. Cherry-picks (D4)

- [x] 2.1 `git cherry-pick -x 0d9ef25 dd867a1` from `upstream/master`.
- [x] 2.2 Resolve `parser/precedence_test.go`, which doesn't exist in the fork: keep only
      the cases that exercise these two fixes.
- [x] 2.3 Regenerate the picked goldens with `make update_test`. Check that only the new
      fixtures changed.

## 3. FormatSettings (D1–D3)

- [x] 3.1 `ast.go`: add `FormatSettings *SettingsClause` after `Format`, with a doc
      comment. Visit it in `SelectQuery.Accept` after `Format`.
- [x] 3.2 `parser_query.go` `parseSelectStmt`: after a non-nil `tryParseFormat`, call
      `tryParseSettingsClause`, and set `FormatSettings` and `StatementEnd` from it.
- [x] 3.3 `format.go`: emit ` SETTINGS …` after the `FORMAT` clause, in compact and
      beautify modes.
- [x] 3.4 `walk.go`: walk `FormatSettings`. Keep the ASTVisitor/Walk drift test green.
- [x] 3.5 Fixtures under `parser/testdata/query/`: `select_format_then_settings.sql`
      (both positions, set operation trailing leg, quoted, `$$` and brace-map values),
      then `make update_test`. Review the JSON-golden diff: it should add one
      `FormatSettings` line per `SelectQuery` and change nothing else.
- [x] 3.6 Inline test `TestParser_With_FormatSettings`, covering every
      `settings-after-format` scenario, including position assertions.

## 4. Verification

- [ ] 4.1 `go vet ./...`, `golangci-lint run` and `go test ./... -count=1` all pass.
- [x] 4.2 Run the chproxy corpus (`chproxy/cache/testdata/nondeterministic/*.sql`)
      through `ParseStmts`. Expected failures: 22 on `main`, 13 after the cherry-picks,
      and 9 (the truncated records only) after `FormatSettings`.

## 5. Release

- [ ] 5.1 Pick a tag name that doesn't collide with upstream's tags (upstream has
      `v0.5.0`–`v0.5.6`). Proposal: `v0.6.0-hdx.1`, or a fork-specific series agreed
      with the team. Update `CHANGELOG.md` and tag after merge.
- [ ] 5.2 Optional: open an upstream PR for `SETTINGS` after `FORMAT`.
