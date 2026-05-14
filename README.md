# Karpenter Provider Nirvana

Karpenter provider implementation for Nirvana Labs.

## Introduction

Karpenter is an open-source node provisioning project built for Kubernetes. Karpenter improves the efficiency and cost of running workloads on Kubernetes clusters by:

- Watching for pods that the Kubernetes scheduler has marked as unschedulable
- Evaluating scheduling constraints (resource requests, nodeselectors, affinities, tolerations, and topology spread constraints) requested by the pods
- Provisioning nodes that meet the requirements of the pods
- Removing the nodes when the nodes are no longer needed

## Getting Started

### Prerequisites

- A Nirvana NKS cluster and its cluster UUID
- `kubectl` configured against that cluster
- [Helm](https://helm.sh) v3.8+ (required for OCI registry support)
- A Nirvana API key, made available to the controller via a Kubernetes `Secret`
  (provisioned automatically by `nirvana-api` at cluster-create time as
  `nirvana-api-key` with key `api-key`)

### Install with Helm

The Helm chart is published as an OCI artifact to GitHub Container Registry on
each release.

```sh
helm upgrade --install karpenter \
  oci://ghcr.io/nirvana-labs/charts/karpenter-provider-nirvana \
  --version <X.Y.Z> \
  --namespace karpenter --create-namespace \
  --set nirvana.clusterID=<NKS_CLUSTER_UUID>
```

Replace `<X.Y.Z>` with the desired chart version (matches the controller
release tag — see [Releases](https://github.com/nirvana-labs/karpenter-provider-nirvana/releases))
and `<NKS_CLUSTER_UUID>` with your cluster's full UUID.

The chart ships the `karpenter.sh` core CRDs (`NodePool`, `NodeClaim`) plus the
provider's own `NirvanaNodeClass`. See
[`charts/karpenter-provider-nirvana/values.yaml`](charts/karpenter-provider-nirvana/values.yaml)
for the full set of configurable values (image, resources, logging, affinity,
probes, etc.).

### Container image

Multi-arch (`linux/amd64`, `linux/arm64`) images are published alongside each
release:

```
ghcr.io/nirvana-labs/karpenter-provider-nirvana:<X.Y.Z>
ghcr.io/nirvana-labs/karpenter-provider-nirvana:latest
```

## Building from source

This project uses [Task](https://taskfile.dev) to drive day-to-day commands —
see [`Taskfile.yml`](Taskfile.yml).

```sh
# One-time: install all tool binaries (golangci-lint, gosec, go-licenses, goreleaser, gotestsum)
task install-tools

# Compile the controller binary to bin/karpenter-provider-nirvana
task build

# Lint, security, and license checks
task lint
task security-check
task license-check

# Validate the goreleaser config and build a local snapshot (no push)
task release:check
task release:snapshot
```

After `task release:snapshot`, inspect `dist/` for the per-arch tarballs,
`checksums.txt`, and local Docker images tagged
`ghcr.io/nirvana-labs/karpenter-provider-nirvana:<snapshot-version>-{amd64,arm64}`.

### Helm chart development

```sh
task helm:lint        # helm lint
task helm:template    # render the chart with a placeholder clusterID
```

### Vendored CRDs

The upstream `karpenter.sh` core CRDs are vendored into the chart and pinned to
the `sigs.k8s.io/karpenter` version in `go.mod`. To refresh them after a
library bump:

```sh
task vendor-crds
```

## Contributing

Commits and PR titles must follow
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) — they
drive `release-please` and the automated release pipeline. See
[`CONTRIBUTING.md`](CONTRIBUTING.md) for the version-bump mapping, release
process, and PR preview-image workflow.
