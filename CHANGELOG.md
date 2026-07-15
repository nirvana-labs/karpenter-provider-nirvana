# Changelog

## [0.15.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.14.0...v0.15.0) (2026-07-15)


### Features

* implement pricing logic for instance types ([#56](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/56)) ([9aa6669](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/9aa6669f24d46448b84efce125c3d710daa041bc))
* make pool scheduling taint aware ([#57](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/57)) ([1204804](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/12048048a26cd3ff155d1ba0ecc91dc57b4feed0))

## [0.14.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.13.0...v0.14.0) (2026-05-13)


### Features

* remove testing limits for node pool capacity ([#52](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/52)) ([568230a](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/568230ac783714d7596706249297eb87eb1f6c73))

## [0.13.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.12.0...v0.13.0) (2026-05-07)


### Features

* implement Get and List methods for NodeClaims ([#49](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/49)) ([0effe09](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/0effe099800306b44e6579a72fd5ffd3d500fb01))

## [0.12.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.11.0...v0.12.0) (2026-05-06)


### Features

* remove cooldown period ([#43](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/43)) ([ed81224](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/ed812245056710ea3fbf6ac2cdb30a7b591a995c))

## [0.11.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.10.0...v0.11.0) (2026-05-05)


### Features

* **chart:** contribute initial helm chart ([#44](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/44)) ([ee94b60](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/ee94b6010195eca997faff3aaaf6e45cce7b17dd))


### Bug Fixes

* **chart:** drop appVersion in favor of single version field ([#46](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/46)) ([b66d147](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/b66d147ff8c63d7f6b099f99a9f3bf2d6f36972e))

## [0.10.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.9.0...v0.10.0) (2026-05-05)


### Features

* add architecture and OS labels to node claims ([#41](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/41)) ([a5f4d98](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/a5f4d980b41ce66559b73bbcbd6ef411e6a2118d))

## [0.9.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.8.1...v0.9.0) (2026-05-05)


### Features

* add requirements for instance types to include architecture and OS labels ([#39](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/39)) ([0db26be](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/0db26be6320ee015ae4a4453ab5dd2b7f6a8c83f))

## [0.8.1](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.8.0...v0.8.1) (2026-05-05)


### Bug Fixes

* update capacity type label from OnDemand to Reserved ([#36](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/36)) ([5641233](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/5641233b4ed1112f88874d0ab7cab99dbe71ea22))

## [0.8.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.7.3...v0.8.0) (2026-05-05)


### Features

* introduce surgical delete for scale-down ([#34](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/34)) ([8fcecd9](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/8fcecd95549a583b647d6babf3928163568e3df2))

## [0.7.3](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.7.2...v0.7.3) (2026-05-04)


### Bug Fixes

* new node labeling ([#30](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/30)) ([e501093](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/e5010931e1dcbb1b12289ab72f5f9c05033e91d3))

## [0.7.2](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.7.1...v0.7.2) (2026-05-04)


### Bug Fixes

* enforce maximum cooldown duration in RecordScaleComplete ([#26](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/26)) ([d0bb6fb](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/d0bb6fb4c3833f2c42b9f49feef54775ab1975a3))

## [0.7.1](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.7.0...v0.7.1) (2026-05-04)


### Bug Fixes

* id mapping for new nodes ([#24](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/24)) ([aba4cad](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/aba4cad4058bc28c39889d8f82093ff3a5f90419))

## [0.7.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.6.0...v0.7.0) (2026-05-04)


### Features

* enable full functionality ([#22](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/22)) ([487d6aa](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/487d6aa8a6306ac9fd371525790cc705010f9c33))

## [0.6.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.5.0...v0.6.0) (2026-04-30)


### Features

* wire to the nirvana api ([#20](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/20)) ([2bfb2bd](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/2bfb2bd14178bd9ba9f5af2d427c2e819a005dc7))

## [0.5.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.4.0...v0.5.0) (2026-04-29)


### Features

* **nodeclass:** implement controller for NirvanaNodeClass with regis… ([#18](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/18)) ([eea5fbc](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/eea5fbca8f173bbb785b50074b92a592f55c3b80))

## [0.4.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.3.0...v0.4.0) (2026-04-29)


### Features

* **controller:** add status controller for NirvanaNodeClass ([#16](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/16)) ([0d0ea43](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/0d0ea43b9c447f6e834b4a4388ce614271381e00))

## [0.3.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.2.0...v0.3.0) (2026-04-29)


### Features

* **init:** basic structure with logging ([#14](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/14)) ([ecdcc62](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/ecdcc62eb60fe864e46d93368edc8039fc0c3086))

## [0.2.0](https://github.com/nirvana-labs/karpenter-provider-nirvana/compare/v0.1.0...v0.2.0) (2026-04-23)


### Features

* add structured logging with zerolog ([#8](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/8)) ([4c9464e](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/4c9464ec06eeffcfaae249b5b9de8b0f671521dc))

## 0.1.0 (2026-04-23)


### Features

* bootstrap release pipeline ([#7](https://github.com/nirvana-labs/karpenter-provider-nirvana/issues/7)) ([716fd6b](https://github.com/nirvana-labs/karpenter-provider-nirvana/commit/716fd6bd71d4ba7680e5368114c41770646a831b))
