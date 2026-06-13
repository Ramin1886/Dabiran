# git-dep-worker — Semantic AST / Dependency Parser Worker

A small Rust CLI that parses cross-repository dependency manifests and
auto-generates **cross-repository dependency links**, closing the
manual-dependency-mapping gap with automation. It can print the links and/or
POST them to the backend for storage and visualization.

## What it does

Given a root directory that contains **one subdirectory per repository**
(the subdirectory **name is the repo id**, e.g. `1/`, `2/`), the worker:

1. Reads each repo's manifests (missing ones are skipped, never an error):
   - `go.mod` — the `module` line is the module this repo **provides**; the
     `require` entries (block or single-line form) are what it **depends on**.
   - `package.json` — `.name` is the npm package this repo **provides**;
     `.dependencies` + `.devDependencies` keys are what it **depends on**.
2. Builds provider maps (`provided module/package -> repo id`).
3. For each repo's dependencies, matches against other repos' provided modules:
   - **Go**: exact match, or a sub-package prefix match (`dep` equals the
     provider module or starts with `<provider>/`). The longest (most specific)
     matching provider wins.
   - **npm**: exact package-name match.
4. Emits a `DependencyLink` per match — no self-links, no duplicates.

## The contract

Each link is the snake_case JSON object the backend ingests and the frontend
renders:

```json
{ "from_repo": "2", "to_repo": "1", "via": "github.com/acme/lib", "kind": "go" }
```

`from_repo` depends on `to_repo` through module/package `via`; `kind` is
`"go"` or `"npm"`. The worker prints `{"links":[...]}` to stdout.

## Run

```sh
# Print links only:
git-dep-worker --root /path/to/repos

# Print AND POST to the backend (both flags required together):
git-dep-worker --root /path/to/repos \
  --api http://localhost:8080 \
  --token "$JWT"
```

With `--api`/`--token` the worker POSTs to
`<base_url>/api/v1/dependency-links` with `Authorization: Bearer <jwt>` and
logs the response status. The caller's team must own every link's `from_repo`
(same team authorization the topology endpoint uses), otherwise the backend
returns 403 and the worker exits non-zero.

The process exits non-zero on hard errors (bad args, unreadable root,
malformed `package.json`, non-2xx POST). Missing manifests in a repo are not
errors.

## Build & test

```sh
cargo build
cargo test     # unit tests for the parsers and the matching algorithm (no network)
```

## Container

A multi-stage `Dockerfile` is provided; see `infra/docker-compose.yml` (the
`worker` service, profile `tools`) for the on-demand invocation.
