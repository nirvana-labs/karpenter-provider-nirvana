# karpenter-provider-nirvana — Helm chart

Karpenter cloud provider for Nirvana NKS clusters. Deploys the controller and the CRDs it depends on (`karpenter.sh/NodePool`, `karpenter.sh/NodeClaim`, `nirvanalabs.io/NirvanaNodeClass`).

## Prerequisites

- A running Nirvana NKS cluster.
- The `nirvana-api-key` Secret in the target namespace, containing key `api-key` with a project-scoped Nirvana API key. Provisioned automatically by nirvana-api at cluster-create time via cloud-init — operators don't manage it directly.
- The full NKS cluster UUID, supplied as `nirvana.clusterID`.

## Install

    helm install karpenter \
      oci://ghcr.io/nirvana-labs/charts/karpenter-provider-nirvana \
      --namespace karpenter \
      --create-namespace \
      --set nirvana.clusterID=<full-NKS-cluster-uuid>

Pin to a specific release with `--version X.Y.Z` (chart and controller versions track each other in lockstep — see the changelog for available versions). Omitting `--version` pulls the latest published tag.

The chart does **not** create the namespace itself — apply PSA labels (or any other policies) on the namespace separately. This keeps the chart focused on the controller and matches the convention used by other Karpenter provider charts.

## Required values

- `nirvana.clusterID` (string) — the full NKS cluster UUID. Chart fails to render without it (`required` template guard).

See [`values.yaml`](values.yaml) for the full set of configurable options (resources, replica count, node placement, logging, etc.).

## What this chart deploys

- ServiceAccount + ClusterRole + ClusterRoleBinding (Karpenter core + `nirvanalabs.io/nirvananodeclasses`)
- Single-replica controller Deployment with PSA-restricted-compatible securityContext (matches the distroless `:nonroot` base image, UID 65532)
- CRDs (`crds/`):
  - `karpenter.sh/v1/NodePool` — vendored from upstream `sigs.k8s.io/karpenter v1.9.0`
  - `karpenter.sh/v1/NodeClaim` — vendored from upstream `sigs.k8s.io/karpenter v1.9.0`
  - `nirvanalabs.io/v1alpha1/NirvanaNodeClass` — owned by this repo

The deployer is responsible for namespace creation and any namespace-level policies (PodSecurityStandard labels, network policies, resource quotas, etc.).

## CRDs and Helm

Helm only installs CRDs on first install (not on upgrade). To bump CRDs (e.g. when the karpenter library version changes), apply them out-of-band before `helm upgrade`:

    kubectl apply --server-side -f charts/karpenter-provider-nirvana/crds/

The upstream `karpenter.sh/*` CRDs are vendored at the version pinned in `go.mod`. Re-vendor with:

    task vendor-crds

## Pod placement

The default `affinity` excludes nodes carrying the `karpenter.sh/nodepool` label, so the controller can't schedule onto a node it later disrupts. Override `affinity` if your cluster topology requires a different rule.

## Versioning

`Chart.yaml` declares only `version` — `appVersion` is intentionally omitted. The chart version tracks the controller release tag in lockstep, bumped automatically by release-please on every release via the `# x-release-please-version` marker on the `version` line (configured through `extra-files` in `.release-please-config.json`). Helm consumers that read `.Chart.AppVersion` (e.g. for `app.kubernetes.io/version` labels) fall through to `.Chart.Version`, which is identical.
