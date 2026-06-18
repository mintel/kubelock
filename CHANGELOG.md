# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-06-18

### Changed

- Leader election now uses native Kubernetes `Lease` objects
  (`coordination.k8s.io/v1`) via client-go's `LeaseLock` instead of
  Endpoints annotations.
- Upgraded Go toolchain to 1.23+ and `k8s.io/client-go` to v0.32.3.
- Updated the Docker build image to `golang:1.26-alpine`.
- Updated README, examples, RBAC manifests, and integration tests for
  lease-based locking.

### Removed

- Support for Endpoints-based leader election. This mechanism was removed
  upstream from client-go and is no longer supported by kubelock.

### Migration

- Replace Endpoints RBAC with Lease permissions:

  ```yaml
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "list", "create", "update"]
  ```

- CLI flags are unchanged (`--name`, `--namespace`, etc.), but `--name`
  now refers to the Lease resource name rather than an Endpoints object.
- A Service is no longer required for the lock itself; kubelock creates
  and updates the named Lease directly.

## [0.3.0] - 2021-06-02

### Changed

- Updated `k8s.io/client-go` and `k8s.io/apimachinery` to v0.22.0-alpha.2.

## [0.2.0] - 2021-05-28

### Changed

- Updated Go to 1.16 and `k8s.io/client-go` to v0.21.1.
- Added `gofmt` check to CI.

## [0.1.0] - 2019-10-16

### Added

- Initial public release of kubelock: a command wrapper that acquires a
  distributed lock using Kubernetes leader election before running a
  subprocess.

[0.4.0]: https://github.com/mintel/kubelock/compare/0.3.0...0.4.0
[0.3.0]: https://github.com/mintel/kubelock/compare/0.2.0...0.3.0
[0.2.0]: https://github.com/mintel/kubelock/compare/0.1.0...0.2.0
[0.1.0]: https://github.com/mintel/kubelock/releases/tag/0.1.0
