# Canonizer Implementation Plan

Rebuild the `ai-skill` ruleset-production tool as a climax-structured `canonizer`
CLI, with **every local path configurable** and **maximum offload to skillet** (and
the stdlib) so canonizer holds only its differentiated logic.

______________________________________________________________________

## 1. Goal & Fidelity Contract

Reproduce the observable behavior of `/Users/steve/Documents/agent-orange/go-advice/ai-skill`:

- **Distill** — walk a tree of Markdown sources and, for each, write a
  `*_prompt.md` that embeds links to the source and to the rules file it should
  produce. This is the *entire* behavior of ai-skill's binary.
- **Synthesise** — ai-skill ships a second prompt template for merging many
  distilled rulesets into one. The binary never touched it (it is a
  human/agent-run asset), but the templates are part of ai-skill's *functionality*,
  so canonizer bundles them and adds a thin command to assemble the synthesis input.

Fidelity target: `canonizer distill --source X --out Y` produces byte-identical
`*_prompt.md` output to `ai-skill`'s `go run . X Y` for the same template, because
both call the same `skillet/ruleset/distill.Generate`.

## 2. Current State

- **canonizer**: fresh `climax init` scaffold — `main.go`, `cmd/cmd.go` (dispatcher
  with climax markers), `cmd/root/root.go` (Config, ExitError), `cmd/version/`.
  Module `github.com/StevenACoffman/canonizer`, Go 1.26.3, `ff/v4`, and
  `github.com/StevenACoffman/skillet v0.3.0` already present (indirect).
- **ai-skill**: ~92-line `main.go`; single stage = `distill.Generate(tmpl, srcDir,
  outDir)`. Template resolved by a three-tier lookup (flag → **binary-relative** →
  **cwd**) against a hardcoded filename `distill_source_prompt.md`; driven by
  `make_distill.sh` with **hardcoded absolute paths**. These are the local-path
  references to eliminate.
- **skillet** (v0.3.0, module `github.com/StevenACoffman/skillet`) already provides
  every non-differentiated piece — see the offload map.

## 3. Local-Path References to Make Configurable (The Core Ask)

| ai-skill today                                                      | canonizer                                                                                    |
| ------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Hardcoded template filename + binary-relative + cwd fallback lookup | **Embedded default template** (`go:embed`, no path at all) with a `--template PATH` override |
| Source dir as positional arg; absolute path in `make_distill.sh`    | `--source DIR` flag (required; validated in `exec`)                                          |
| Output dir as positional arg; absolute path in `make_distill.sh`    | `--out DIR` flag (required; validated in `exec`)                                             |
| Synthesis template only reachable by editing files in place         | Embedded default + `--template PATH` override                                                |

Result: no absolute paths, no cwd/binary-relative magic, no hardcoded filenames in
code. The default is compiled in; every path is a flag.

## 4. Offload Map (Avoid Undifferentiated Heavy Lifting)

| Behavior                                                                                                       | Owner                                                                                                                                                           |
| -------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Tree walk, source filtering (`*_rules.md`/`*_prompt.md`/hidden skipped), per-source prompt fill, atomic writes | `skillet/ruleset/distill.Generate`                                                                                                                              |
| Template placeholder validation (fail-loud on missing `{{SOURCE_CONTENT}}`/`{{DESTINATION_CONTENT}}`)          | `skillet/ruleset/distill` (via `Generate`)                                                                                                                      |
| Source title + `*_rules.md`/`*_prompt.md` filename derivation                                                  | `skillet/naming` (inside `Generate`)                                                                                                                            |
| CLI dispatch, I/O injection, exit codes, signal handling, `-h`                                                 | climax scaffold over `ff/v4`                                                                                                                                    |
| Embedding default templates into the binary                                                                    | stdlib `embed`                                                                                                                                                  |
| Assemble N distilled rulesets into one synthesis prompt                                                        | `internal/synth` (pure) — **no skillet equivalent yet**; flagged in TODO as an upstream-centralization candidate, mirroring how distill was pushed into skillet |

canonizer's own code is therefore small: template assets, two thin command shells,
and one pure ~30-line fill function.

## 5. Architecture — Functional Core / Imperative Shell

Pure asset provider and pure core; thin shells that do the I/O.

```text
internal/prompt/          — the prompt-template source: embedded defaults + resolution
    prompt.go             —   //go:embed defaults; Resolve(path, fallback) (string, error)
    distill_source_prompt.md
    synthesize_rulesets_prompt.md
internal/synth/           — pure synthesis-input assembly (no I/O)
    synth.go              —   FillTemplate(tmpl string, blocks []Ruleset) (string, error)
cmd/distill/distill.go    — shell: resolve flags, load template, call distill.Generate, print
cmd/synthesize/synthesize.go — shell: read *_rules.md, call synth.FillTemplate, write one file
cmd/cmd.go                — register distill + synthesize (climax markers preserved)
```

- The commands hold **no business logic**: each `exec` is a flat sequence —
  read flags → `prompt.Resolve` → call one core/skillet function → write result. Per
  §5 (functional core / imperative shell) and §8 (a CLI is a composition root).
- `internal/prompt` is a **deep module**: one call, `Resolve(path, fallback)`, hides
  both the `embed` machinery and the file read behind a trivial interface — the
  caller never handles a path or an `os.ReadFile`. It is the single home for the
  "override by path, else use the baked-in default" rule, so neither command repeats
  it (§4, Repetition red flag). Its concept is coherent: *where a template comes
  from*, not a general file reader — so it is not a special/general mixture (§4).
- `internal/synth.FillTemplate` is **pure** (values in, string out, no I/O, no
  clock), so it is table-testable without fixtures (§5).

## 6. Command Surface (Every Knob Is a Flag; No `os.Getenv`, No Hardcoded Path)

**`canonizer distill`** — faithful duplicate of ai-skill's binary.

| Flag         | Type   | Default                          | Meaning                               |
| ------------ | ------ | -------------------------------- | ------------------------------------- |
| `--template` | string | `""` → embedded distill template | Path to a distill prompt template     |
| `--source`   | string | *(required)*                     | Directory tree of source `.md` files  |
| `--out`      | string | *(required)*                     | Directory to write `*_prompt.md` into |

`exec`: `tmpl, err := prompt.Resolve(cfg.Template, prompt.Distill)` →
`distill.Generate(tmpl, cfg.Source, cfg.Out)` → print `wrote <path>` per returned
path to `cfg.Stdout`. Missing `--source`/`--out` returns a plain wrapped error
`distill: --source is required` (lowercase, no trailing punctuation, §18).

**`canonizer synthesize`** — assemble the synthesis prompt from distilled rulesets.

| Flag         | Type   | Default                             | Meaning                                                  |
| ------------ | ------ | ----------------------------------- | -------------------------------------------------------- |
| `--template` | string | `""` → embedded synthesize template | Path to a synthesis prompt template                      |
| `--rulesets` | string | *(required)*                        | Directory of `*_rules.md` files to merge                 |
| `--out`      | string | `""` → stdout                       | File to write the assembled prompt into (empty = stdout) |

The embedded synthesize template carries a single clean `{{RULESETS}}` marker (the
one adaptation from ai-skill's variable-arity `<ruleset id="N">` example block, so
assembly is a deterministic single replacement). `synth.FillTemplate` validates the
marker is present (fail-loud, like distill) and injects one
`<ruleset id="i" source="<title>">…</ruleset>` block per input, sorted by filename.

## 7. Implementation Phases

Each phase ends with a self-review against `summary_rules.md` **and**
`golangci-lint run --fix ./...` (rules unchanged); proceed only when both are clean.

1. **Templates** — create `internal/prompt`; copy ai-skill's `distill_source_prompt.md`
   verbatim (skillet validates its placeholders); adapt the synthesize template to a
   single `{{RULESETS}}` marker; `//go:embed` both. Lint.
2. **Distill command** — `cmd/distill`; wire `skillet/ruleset/distill.Generate`;
   register in `cmd/cmd.go`. `go mod tidy` (skillet → direct). Smoke-test against a
   temp fixture tree; compare against ai-skill output. Lint.
3. **Distill tests** — end-to-end via `cmd.Run(ctx, args, stdin, stdout, stderr)`
   with injected buffers and `t.TempDir()` source/out; assert written files + stdout.
   `internal/prompt` test asserts defaults are non-empty and contain the required
   placeholders. Lint.
4. **Synthesize** — `internal/synth.FillTemplate` (pure) + table tests
   (marker present / missing / zero rulesets); `cmd/synthesize` shell; register. Lint.
5. **Finalize** — `go build ./...`, `go test ./...`, `climax lint .` (structural
   drift), final `golangci-lint run ./...`. Confirm no absolute paths anywhere
   (`grep` for `/Users/`), no `os.Getenv`, no `os.Stdout`/`os.Stderr` in commands.

## 8. Testing (Stdlib Only; §9–10)

- **distill** (`cmd/distill/distill_test.go`): build a `t.TempDir()` with two source
  `.md` files; run the command through `cmd.Run` with a `bytes.Buffer`; assert both
  `*_prompt.md` exist and stdout lists them. A second case: `--template` pointing at
  a temp template missing a placeholder surfaces skillet's fail-loud error.
- **synth** (`internal/synth/synth_test.go`): table-driven `FillTemplate` — marker
  present injects N blocks in order; marker missing returns an error; empty input is
  a defined outcome.
- **prompt** (`internal/prompt/prompt_test.go`): defaults non-empty; distill default
  contains `{{SOURCE_CONTENT}}` and `{{DESTINATION_CONTENT}}`; synth default contains
  `{{RULESETS}}`.
- No third-party assert libs; helpers use `t.Helper()`; `t.Parallel()` where safe;
  no `time.Sleep`; no `t.Setenv`.

## 9. Non-Goals

- **No LLM invocation** — ai-skill doesn't call a model; canonizer fills templates
  for a downstream agent. The rigor loop (cold critic, findings gate, provenance)
  lives in `ai_skill_todo.md` and is carried into `TODO.md`, not built here.
- **No new CLI framework** — `ff/v4` only, via the climax scaffold.
- **No ruleset parsing/verification yet** — `ruleset.Parse`/`finding`/`judge` are
  available for later rigor work; this pass reproduces ai-skill, which does none.

## 10. Risks & Decisions

- **skillet `Generate` always writes** (no dry-run) — matches ai-skill exactly
  (ai-skill dropped its `-dry-run` for this reason, per skillet TODO). Documented, not
  worked around.
- **Synthesize template adaptation** — the single `{{RULESETS}}` marker diverges from
  ai-skill's illustrative multi-block example. This is deliberate: it makes assembly a
  pure, deterministic replacement instead of ad-hoc block editing. Noted in TODO as a
  candidate to standardize upstream in skillet.
- **Required flags in ff/v4** — `ff` has no built-in "required"; `exec` validates and
  returns a lowercase, punctuation-free `<command>: <reason>` error (§18).

## 11. Design Review Against `summary_rules.md`

| Rule                                                                                   | How the plan satisfies it                                                                                                                                                                                                                                 |
| -------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| §1 Project layout — pure CLI keeps `main.go` at root, `cmd/` holds command subpackages | Inherited from the climax scaffold; new logic lives in `internal/`, not `main`                                                                                                                                                                            |
| §1 One major concept per file; no cross-imports among sibling subpackages              | `prompt`, `synth`, and each command are single-concept leaves; commands import the `internal/*` leaves, never each other                                                                                                                                  |
| §4 Deep modules; no pass-through methods; no special/general mixture                   | `prompt.Resolve` and `synth.FillTemplate` hide real work behind small interfaces; no method merely forwards an identical signature                                                                                                                        |
| §4 Interface comment before body                                                       | Each exported func's godoc/contract (`Requires`/`Ensures`) is written before its implementation                                                                                                                                                           |
| §4 Model constraints in types                                                          | `synth.Ruleset{Title, Body}` is a named struct, not a `(string, string)` pair, so element meaning is in the type (§13)                                                                                                                                    |
| §5 Functional core / imperative shell                                                  | `synth.FillTemplate` is pure; the two `exec`s are the only I/O and stay branch-light                                                                                                                                                                      |
| §5 Two-commit evolution                                                                | Phase 4 lands `internal/synth` + tests *before* wiring `cmd/synthesize`                                                                                                                                                                                   |
| §13 Consistent naming                                                                  | `Resolve`, `FillTemplate` mirror skillet's `distill.FillTemplate`; no synonyms invented                                                                                                                                                                   |
| §15 Don't add complexity that doesn't pay                                              | **No domain `Error` type / error-code taxonomy** — that pattern serves a domain package with many callers; a two-command path-filler uses plain `fmt.Errorf("<cmd>: <reason>")`. Adding the full `Error` machinery here would be complexity with no payer |
| §18 Pattern B (ff/v4)                                                                  | Flags bound in `New`, not `exec`; `SetParent` on every flag set; I/O via `cfg.Stdout`/`cfg.Stderr`; controlled exits via `root.ExitError`; climax markers preserved                                                                                       |
| §9 Investment calibrated                                                               | Tended, low-risk dev tool → deliberate design + minimal formal tests; the pure core and one e2e test earn their keep as design pressure, not coverage theater                                                                                             |

**Two designs considered (§15).** (a) Read templates from a configurable directory
at runtime — rejected: it *keeps* a local-path dependency, the exact smell we are
removing. (b) Embed defaults + `--template` override — chosen: zero required paths,
still fully overridable. This is the ergonomic inversion of ai-skill's fragile
binary-relative/cwd lookup.

**What this plan deliberately does not build** (carried to `TODO.md`): the
`ai_skill_todo.md` rigor loop (cold critic, structured-findings gate, provenance),
ruleset parse/verify via `ruleset`/`finding`/`judge`, and pushing `synth.FillTemplate`
upstream into skillet.
