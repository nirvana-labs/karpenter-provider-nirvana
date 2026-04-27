# Karpenter Provider Nirvana

Karpenter provider implementation for Nirvana Labs.

## Introduction

Karpenter is an open-source node provisioning project built for Kubernetes. Karpenter improves the efficiency and cost of running workloads on Kubernetes clusters by:

- Watching for pods that the Kubernetes scheduler has marked as unschedulable
- Evaluating scheduling constraints (resource requests, nodeselectors, affinities, tolerations, and topology spread constraints) requested by the pods
- Provisioning nodes that meet the requirements of the pods
- Removing the nodes when the nodes are no longer needed
