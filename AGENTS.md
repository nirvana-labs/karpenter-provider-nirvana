# Agent instructions

`karpenter-provider-nirvana` is a [Karpenter](https://karpenter.sh) cloud
provider for Nirvana Labs. Go module
`github.com/nirvana-labs/karpenter-provider-nirvana`; controller entry point at
`./cmd/controller`.

## Build, lint, test

Day-to-day commands run through [Task](https://taskfile.dev) — see
`Taskfile.yml`:

- `task build` — compile the controller binary to `bin/`
- `task lint` — golangci-lint
- `task security-check` — gosec
- `task license-check` — go-licenses
- `task release:check` — validate `.goreleaser.yaml`
- `task release:snapshot` — goreleaser dry-run (no push)

Use `task install-tools` on a fresh checkout to install every required tool
binary.

## Commit and PR conventions

Commits and PR titles must use
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/). This is
enforced by the `pr-title` job in `.github/workflows/lint.yml` and is
load-bearing for the `release-please` automation. See `CONTRIBUTING.md` for the
version-bump mapping and the `Release-As:` override syntax.

PRs are squash-merged, so the PR title becomes the `main`-branch commit
message — that is what `release-please` parses.

## Releases

Tagging and changelog maintenance are automated. See `CONTRIBUTING.md`
§Releasing, plus the `release-please`, `release`, and `snapshot` workflows
under `.github/workflows/`. Do not edit `CHANGELOG.md` or create tags manually.
