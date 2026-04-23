# Contributing

This doc covers the conventions the release pipeline depends on. Code style and
review mechanics are handled by the existing `lint`, `build`, `security-check`,
and `license-check` workflows — read those for detail.

## Commit messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).
The `pr-title` job in `.github/workflows/lint.yml` blocks any PR whose title
does not parse as a Conventional Commit, because PRs are squash-merged and the
PR title becomes the commit message on `main`. `release-please` then parses
those commits to decide the next version.

### Version bumps

| Commit type | Bump (pre-1.0) | Bump (≥ 1.0) |
| --- | --- | --- |
| `feat:` | minor | minor |
| `fix:` | patch | patch |
| `feat!:` / `fix!:` / `BREAKING CHANGE:` footer | minor | major |
| `perf:`, `deps:`, `revert:` | shown in changelog, no bump | same |
| `chore:`, `docs:`, `refactor:`, `test:`, `build:`, `ci:` | hidden, no bump | same |

### Manual overrides

- **Force a specific version:** add a `Release-As: 1.2.3` footer to a commit on
  `main`. `release-please` will propose that exact version in its next release
  PR, regardless of the computed bump.
- **Skip a release:** don't merge the release PR. It stays open and updates as
  new commits land.
- **Force a major bump pre-1.0:** include a `BREAKING CHANGE:` footer, or use
  `Release-As: 1.0.0` when you're ready to declare the API stable.

## Releasing

1. Merge PRs to `main` with Conventional Commit titles.
2. `release-please.yml` opens (or updates) a `chore(main): release X.Y.Z` PR
   that rewrites `CHANGELOG.md` and bumps `.release-please-manifest.json`.
3. When ready, merge the release PR. `release-please` creates tag `vX.Y.Z` and
   a GitHub Release.
4. `release.yml` runs on the tag push. `goreleaser` builds `linux/amd64` and
   `linux/arm64` tarballs, uploads them plus `checksums.txt` to the Release,
   and pushes a multi-arch Docker manifest to
   `ghcr.io/nirvana-labs/karpenter-provider-nirvana:X.Y.Z` and `:latest`.

`CHANGELOG.md` is maintained by `release-please` — don't edit it by hand.

## Preview images for staging

To test an unmerged PR on the NKS staging cluster, a maintainer applies the
`preview` label. `snapshot.yml` then builds and pushes:

- `ghcr.io/nirvana-labs/karpenter-provider-nirvana:pr-{N}-{shortsha}` —
  immutable per commit
- `ghcr.io/nirvana-labs/karpenter-provider-nirvana:pr-{N}` — floating,
  overwritten on each push to the PR

Point staging manifests at the floating `pr-{N}` tag for easy refresh, or the
sha-suffixed tag for reproducibility. A sticky comment on the PR shows both.

Fork PRs are skipped intentionally — push the branch to this repo if you need a
preview image.

## Local development

```sh
task install-tools        # all tool binaries (lint, sec, licenses, goreleaser)
task build                # compile ./cmd/controller to bin/karpenter-provider-nirvana
task lint                 # golangci-lint
task security-check       # gosec
task license-check        # go-licenses

task release:check        # validate .goreleaser.yaml
task release:snapshot     # dry-run a release locally (no push)
```

After `task release:snapshot`, inspect `dist/` for tarballs, `checksums.txt`,
and per-arch local Docker images tagged
`ghcr.io/nirvana-labs/karpenter-provider-nirvana:<snapshot-version>-amd64|arm64`.
