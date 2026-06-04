# AGENTS.md

Guidance for AI agents working on the orla codebase. The CLAUDE.md
symlink resolves to this file. Read top-to-bottom on first session.

## Project context

Orla is a Go daemon that sits between agent code and the LLM or tool
backends that serve it. It is an OpenAI-compatible proxy with two
side channels: a registry the platform engineer writes to, and a
telemetry stream the mapper reads from. See [`docs/concepts.md`](docs/concepts.md)
for the model, [`docs/quickstart.md`](docs/quickstart.md) for the
hands-on tour, and [`docs/proxy.md`](docs/proxy.md) plus
[`docs/storage.md`](docs/storage.md) for the wire and schema details.

The codebase is small and uniformly Go. There is no Python SDK in
this repo. Frontend tooling, CI helpers, and demos live in separate
repositories.

## Quality gate

Before declaring any change complete, run:

```bash
just check
```

That runs `go build ./...`, the test suite with the race detector, a
coverage profile, `golangci-lint`, and an offline markdown link
check. The pipeline is the same one CI runs. If it does not pass
locally, the work is not done.

Individual recipes:

```bash
just build        # compile every package
just test         # tests only (Docker required for testcontainers)
just test-cover   # tests with coverage.out (used by CI)
just lint         # golangci-lint v2
just links        # offline markdown link check
just fmt          # go fmt + go mod tidy
just sqlc         # regenerate internal/storage/db
just binary       # build bin/orla
just modernize    # gopls modernize report (no changes)
just              # list recipes
```

Storage tests use [testcontainers](https://testcontainers.com/) and
need Docker running.

## Repository layout

```
cmd/orla/                CLI entry point and cobra subcommands
internal/
  api/                   HTTP server, middleware, route handlers, proxy
  backends/              Backend registry (PostgresRegistry + FakeRegistry)
  config/                envconfig-based daemon configuration
  metrics/               Prometheus collectors and registration
  provider/              LLMProvider + ToolProvider interfaces
  provider/structurepred/ Tool provider for protein structure prediction
  scheduler/             Per-backend FCFS executor with concurrency caps
  stages/                Stage registry (PostgresRegistry + FakeRegistry)
  storage/               pgx pool, goose migrations, BatchWriter
  storage/db/            sqlc-generated query code (regenerate with `just sqlc`)
  storage/queries/       sqlc query files
  storage/migrations/    goose .sql files
  telemetry/             Completion and feedback writers + readers
docs/                    User-facing documentation
share/                   README banner assets
```

There is no `pkg/`. Public clients consume orla over HTTP, not by
importing Go packages.

## Writing prose

The following style rules apply to all prose in the repo: README,
docs, design notes, commit message bodies, code comments. They were
established by the maintainer's stated preference and must be
honored.

### Hard rules

- **No em-dashes.** The character `—` does not appear in prose. If
  you would normally use an em-dash, split into two sentences or use
  a comma. The same goes for en-dashes (`–`) in prose.
- **No semicolons in prose.** Use a period and start a new sentence.
- **No unnecessary parentheses.** Parenthetical asides that pause the
  reader for a thought you could have put in its own sentence should
  go in its own sentence. Parens are fine for genuine clarifications
  (e.g., abbreviations on first use) but not as a substitute for a
  comma or period.
- **No ASCII diagrams.** Describe relationships in prose. ASCII boxes
  and arrows are hard to maintain and rarely earn their space.
- **No emoji** unless the user explicitly asks for them.

### Soft rules

- Write short, direct sentences. If a sentence has more than one
  comma, consider whether it should be two sentences.
- Lead with the noun, not the qualifier. "The proxy looks up the
  stage" beats "When a request arrives, the proxy looks up the
  stage."
- Define jargon on first use, even if you think the reader knows it.
- Don't write multi-paragraph code docstrings. One short paragraph
  per exported identifier is the ceiling.

### Examples

Wrong:

> The proxy auto-creates a default stage record (empty backend, empty
> everything) on first sighting; the request then falls back to
> `req.Model` for that one call.

Right:

> The proxy auto-creates a default stage record on first sighting.
> The request falls back to `req.Model` for that one call.

Wrong:

> Cost comes from the generic Rates map, with the tool reporting
> matching usage at dispatch time. ToolKind is required, ModelID is
> unused.

Right:

> Cost comes from the generic Rates map, with the tool reporting
> matching usage at dispatch time. ToolKind is required. ModelID is
> unused.

## Writing comments

The rules under "Writing prose" apply, plus:

- **Default to writing no comment.** A well-named identifier and a
  short function explain themselves. Only comment when the WHY is
  non-obvious: a hidden constraint, a subtle invariant, a workaround
  for a specific upstream bug, behavior that would surprise a reader.
- **Don't describe what the code does.** The code does that.
  "Increments the counter" above `c++` is noise.
- **Don't reference the past.** "Renamed from X", "formerly Y",
  "was previously fooBar" all rot. Comments describe the present
  state. If the reader needs migration history they can `git log`.
- **Don't reference callers or PRs.** "Called by X", "added for
  the Y flow", "see issue #123" all rot as the codebase evolves.
  Caller context belongs in the PR description.
- **Don't write multi-line comment banners.** Use one short comment
  per declaration, not a docblock with `@param`/`@returns`/`@example`
  decoration. Go doc tooling reads the comment line directly above
  the symbol.
- Package docs go in one file per package and explain the package's
  purpose in a paragraph or two. They are an exception to the
  "default to no comment" rule.

## Writing tests

### Structure

Use table-driven tests with subtests when there are multiple cases of
the same shape. Use standalone `Test<X>_<Scenario>` functions when
there is one case or each case is shaped differently.

```go
tests := []struct {
    name string
    in   T
    want U
}{
    {name: "descriptive case", in: ..., want: ...},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // ...
    })
}
```

### Naming

`Test<Type>_<Scenario>` or `Test<FunctionName>_<Scenario>`. The
scenario reads as a clause: `TestProxy_ComputesLLMCostUSD`,
`TestFakeRegistry_GetReturnsIndependentRatesCopy`,
`TestBackendHandlers_CreateRejectsRatesOnLLM`.

### Assertions

`github.com/stretchr/testify` is the assertion library.

- `require` for preconditions and setup that must succeed before the
  rest of the test makes sense. Failure halts the test.
- `assert` for the actual checks under test. Failure records the
  failure and continues so the test surfaces every problem in one
  run.

### Mocks and fakes

Use hand-written fakes with the suffix `Fake*` (e.g. `FakeRegistry`).
Do not introduce a mocking code generator.

For HTTP-shaped dependencies, prefer `httptest.NewServer` or a
hand-written fake server function that returns a *httptest.Server.
Both are easier to debug than generated mocks.

Every Fake should pass the same contract test as the real
implementation (see `internal/backends/fake_registry.go` and the
contract test that exercises both `PostgresRegistry` and
`FakeRegistry`).

### What to test

- The happy path for every exported function.
- Every validation branch that returns an error.
- Boundary cases for numeric inputs (zero, max, NaN, Inf, negative).
- One end-to-end integration test per HTTP route that wires storage,
  scheduler, and the handler together.
- The fakes themselves. A silently broken fake hides regressions.

## Commit messages

[Conventional commits](https://www.conventionalcommits.org/en/v1.0.0/),
one sentence each, no body unless absolutely necessary.

```
feat: compute cost_usd for LLM dispatches from per-million-token rates
fix: reject non-finite tool costs and log misconfig
docs: refresh storage.md schema and cost semantics for v2
refactor: squash migrations into a single fresh init.sql
chore: drop demo-* justfile recipes since demos live in a separate repo
style: replace em-dashes with commas in Go comments
test: add coverage for batch writer drop policy
```

Rules:

- One sentence subject. No imperative-vs-past-tense pedantry, just
  pick one and be consistent. The examples above use present-tense
  imperative.
- Lowercase the type and the first word after the colon, unless that
  first word is a proper noun or acronym.
- No commit body unless the change is non-obvious and the reason
  cannot fit in the subject. Don't pad with a "Test plan" or
  "Summary" boilerplate.
- **No `Co-Authored-By: Claude` trailer.** Ever. Even when the user
  has authorized commits in advance.
- Do not amend or rewrite published commits without explicit user
  consent. Force-push only with `--force-with-lease`, only on a
  feature branch, and only after confirming with the user.

## Git practices

- Use whatever git identity the user has configured. Never pass
  `-c user.email` or `-c user.name`.
- Don't push without explicit authorization. The user often wants to
  review the local commit chain before it leaves the machine.
- For PR merges, prefer `gh pr merge <num> --squash --delete-branch`.
- Before any destructive operation (`git reset --hard`, force-push,
  `git rm -r .`, branch delete), confirm with the user. Use
  `--force-with-lease` not `--force` when force-pushing is
  authorized.

## Go style

### Naming

- Exported: PascalCase. Unexported: camelCase. Acronyms stay
  uppercase: `LLMBackend`, `APIKeyEnvVar`, `CostUSD`.
- One main type per file. Mocks/fakes in `fake_*.go` or `mock_*.go`.
  Package-shared types in `types.go` if there is no obvious home.
- Receivers are one-letter abbreviations for the type name (`r` for
  `*PostgresRegistry`, `s` for `*Scheduler`).

### Errors

- Wrap with `fmt.Errorf("operation: %w", err)`. Include enough
  context that the caller can tell which operation failed without
  reading the trace.
- Return sentinel errors (`var ErrNotFound = errors.New(...)`) for
  conditions callers branch on. Compare with `errors.Is`.
- Validation errors should describe both the constraint and the bad
  input: `"max_concurrency must be >= 1, got 0"`.
- Don't swallow errors. If an operation can fail and you choose to
  proceed anyway, log at warn level with enough context that an
  operator can investigate.

### Logging

`log/slog` everywhere. Use structured key-value attrs, not
formatted strings.

```go
slog.Default().Warn("tool: dropping non-finite reported cost",
    "backend", backendName,
    "completion_id", completionID,
    "cost_usd", c,
)
```

Logs go to stderr so stdout stays clean for tool wrappers and child
processes.

### Concurrency

- Protect shared state with `sync.RWMutex`. Use `RLock`/`RUnlock` for
  read-only access.
- `defer mu.Unlock()` immediately after `mu.Lock()`. Keep the locked
  region small.
- Never write to a map under `RLock`. If you find yourself wanting
  to, restructure or upgrade to `Lock`.
- Channels for ownership transfer, mutexes for protecting fields.
  Don't use channels as locks.
- When passing maps across goroutine boundaries, copy them at the
  boundary unless the producer documents the no-mutate contract.
  See `copyUsage` in `internal/api/tool.go` for the pattern.

### Optional fields

Use pointer types for optional values: `*int`, `*float64`,
`*string`. nil means "not set". Helpers like `core.IntPtr` and
`new(string)` are both fine.

For maps in patch requests, use `*map[K]V` so the caller can
distinguish three states:

- absent field in JSON: the pointer is nil. Don't modify.
- JSON null or pointer to nil map: clear.
- pointer to populated map: overwrite.

See `backends.PatchRequest.Rates` for the canonical example.

### Validation

Validate at boundaries: HTTP handlers, database reads from
operator-managed columns, anything coming in from outside the
process. Once a value is past the boundary, trust it.

Helpers like `isFiniteNonNegative(float64) bool` and
`validateRates(map) string` keep the boundary checks readable.

## Database and storage

### Migrations

Goose with embedded `.sql` files under `internal/storage/migrations`.
Files are `NNNN_description.sql` with `-- +goose Up` and
`-- +goose Down` sections.

The v2 init lives in `0001_init.sql`. Add new changes as new files
(`0002_*.sql`, `0003_*.sql`, …). Do not edit prior migrations once
they have been deployed.

### sqlc

`sqlc.yaml` configures generation. Query files live in
`internal/storage/queries/`. Generated code lives in
`internal/storage/db/`.

Workflow:

1. Edit the migration file with the schema change.
2. Edit the query file in `internal/storage/queries/`.
3. Run `just sqlc` to regenerate.
4. Update Go callers to use the new generated types.

Never edit `internal/storage/db/*.go` by hand. They are regenerated
from the queries and will lose your changes.

### Write strategy

- **Control plane** (stage records, backend records): synchronous,
  return after the row is durable.
- **Data plane** (completion records, feedback): async via
  `storage.BatchWriter[T]`. Buffer drops are counted in a Prometheus
  metric. The producer must not block.

### JSONB columns

Use the helpers in `internal/telemetry/completion.go`:

- `encodeJSONBObject` returns valid `{}` bytes for nil or empty maps
  and logs marshal failures. The default for JSONB columns must be
  `'{}'::jsonb`, not `null`, even if Go would naturally encode a nil
  map as JSON `null`.
- On the read path, surface unmarshal errors. Don't silently set
  fields to nil when the JSONB column is malformed: an operator hand
  edit or schema drift will silently zero downstream calculations.

## Cost reporting

Two channels exist on backends:

- LLM backends price through `input_cost_per_mtoken` and
  `output_cost_per_mtoken` (per million tokens). The proxy computes
  `cost_usd = (prompt_tokens × input + completion_tokens × output) /
  1_000_000` and records it on every completion.
- Tool backends price through the `rates` JSONB map. Each key is a
  resource name (`gpu_seconds`, `cpu_seconds`, `calls`, …) and the
  value is USD per unit. Tool wrappers report a parallel `usage` map
  on their response. The proxy computes the dot product.

Rules:

- `rates` is rejected at registration time for LLM backends. The API
  returns 400.
- All numeric rate and cost values are validated as finite and
  non-negative. NaN, Inf, and negative values are rejected with 400
  at the API boundary and dropped with a log line at the proxy if a
  tool returns one in a response.
- A tool can short-circuit cost computation by setting
  `cost_usd` on its response. The proxy uses that value verbatim
  after the same finite-non-negative sanity check.
- A tool that reports usage keys that do not match any backend rate
  emits a warning log naming both key sets. Cost is recorded as null,
  not zero, so an operator can tell misconfiguration from a free
  tool.

## Working with the user

### Risk and reversibility

Carefully consider the blast radius of every action. Local, reversible
actions (edit a file, run a test) need no preamble. Hard-to-reverse
actions (force-push, drop database, delete a branch, modify a shared
configuration) need explicit user confirmation each time.

Authorization for a single action does not extend to similar actions.
A `git push` approved once is not blanket approval for every future
push.

### Confirmation patterns

- Lay out a plan before doing destructive multi-step work. Get a
  green light, then execute.
- After every destructive step, summarize the state. The user often
  wants to verify before authorizing the next step.
- When you spot a side-effect the user didn't ask for (cleanup,
  refactor, lint fix), name it and ask before doing it. Do not slip
  it into a commit silently.

### Communication style

- Default to terse. The user reads diffs and can see what changed.
- Lead with the result, then the details if asked. Don't bury the
  headline under a recap of the process.
- One-sentence end-of-turn summary: what shipped and what is next.
  Never longer than two sentences.
- Don't restate the user's request back to them. Don't say "Great
  question" or "Let me help with that". Just answer.
- When you've already done a task, don't describe it in the past
  tense; the diff already documents it.

### Scope

Match the scope of your changes to what the user asked. A bug fix
does not get a free refactor of the surrounding code. A one-shot
script does not need a helper module.

If a side-improvement is genuinely small and obvious (one line,
zero behavior change), do it without ceremony. If it is more than
that, surface it as a separate option for the user to opt into.

## Adding a new feature

A rough order:

1. **Schema first** if the feature touches storage. Add the
   migration. Update `internal/storage/queries/*.sql`. Run `just sqlc`.
2. **Types and interfaces** next. Define the wire types, the patch
   request, the response shape. Keep them in the package that owns
   the domain (`backends`, `telemetry`, `stages`).
3. **Implementation** with both `PostgresRegistry` and `FakeRegistry`
   paths if applicable. They must pass the same contract test.
4. **HTTP handler** wiring with validation at the boundary.
5. **Tests** for each: happy path, every validation branch, one
   integration test that ties storage and handler together.
6. **Documentation** if the feature changes the wire contract or the
   schema. Update `docs/concepts.md`, `docs/proxy.md`, and
   `docs/storage.md` as relevant.
7. **`just check`** until green. Fix everything it surfaces.

## When in doubt

Re-read this document, then the most recent code changes that touched
the same area. The patterns are intentionally consistent across the
codebase. Match them rather than introducing a new variation.
