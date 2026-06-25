# CLI Manager Operator

An OpenShift operator that manages the [CLI Manager](https://github.com/openshift/cli-manager) operand, which distributes CLI tools and kubectl plugins to cluster users via [krew](https://krew.sigs.k8s.io/). Built on the [library-go](https://github.com/openshift/library-go) controller framework, it watches a singleton `CliManager` custom resource and reconciles a Deployment, Route, Service, RBAC, and ServiceMonitor for the operand. Installed by the [Cluster Version Operator](https://github.com/openshift/cluster-version-operator) (CVO).

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design and data flow.

## Build and Test

```bash
make build          # Build all binaries (operator + OTE test runner)
make test-unit      # Unit tests (./pkg/... ./cmd/...) — alias: make test
make verify         # Formatting, vetting, golang version checks
make test-e2e       # E2E operator tests (3h timeout, requires cluster)
```

Go version: see `go.mod`.

## Project Structure

| Directory | Purpose |
|-----------|---------|
| `cmd/cli-manager-operator/` | Operator binary entry point (`operator` subcommand) |
| `cmd/cli-manager-operator-testing/` | Testing variant that enables insecure HTTP artifact serving |
| `cmd/cli-manager-operator-tests-ext/` | OTE test runner entry point |
| `pkg/operator/starter.go` | Operator initialization — creates clients, informers, and starts controllers |
| `pkg/operator/target_config_reconciler.go` | Main reconciliation loop — manages all operand resources |
| `pkg/operator/operatorclient/` | Namespace constants and operator client interfaces |
| `pkg/apis/climanager/v1/` | `CliManager` CRD type definitions |
| `pkg/generated/` | Generated clientsets, informers, listers for the CliManager CRD |
| `pkg/cmd/operator/` | Cobra command factory for the `operator` subcommand |
| `pkg/version/` | Build version info and Prometheus build_info metric |
| `bindata/assets/cli-manager/` | Embedded operand manifests (Deployment, Service, Route, RBAC, ServiceMonitor) |
| `manifests/` | CVO deployment manifests (CRD, CSV) |
| `deploy/` | Quick-start manifests for direct deployment (CRD, namespace, RBAC, operator Deployment, CR) |
| `test/e2e/` | E2E test suite (deploys operator, creates Plugin CR, validates krew install) |

## Controller Pattern

The operator has two controllers wired in `pkg/operator/starter.go` via `RunOperator()`:

1. **`TargetConfigReconciler`** — the main reconciliation loop. Watches the singleton `CliManager` CR (name `cluster`, namespace `openshift-cli-manager-operator`) and reconciles all operand resources on every add/update/delete event.
2. **`ClusterOperatorLoggingController`** — a library-go controller that watches the CR's `logLevel`/`operatorLogLevel` fields and dynamically adjusts klog verbosity at runtime.

The reconciler uses `resourceapply` from library-go for idempotent resource management: it reads embedded YAML assets from `bindata/`, substitutes the operand image placeholder (`${IMAGE}`), applies log-level flags, and applies each resource.

## Key Conventions

- **Namespace:** everything runs in `openshift-cli-manager-operator`. `OperandName = "openshift-cli-manager"` is a resource name prefix, not a separate namespace. Constants in `pkg/operator/operatorclient/interfaces.go`.
- **Singleton CR:** The operator watches a single `CliManager` CR named `cluster`.
- **Operand image:** Injected via the `RELATED_IMAGE_OPERAND_IMAGE` environment variable on the operator Deployment.
- **Logging:** `k8s.io/klog/v2` with verbosity levels. Log level from CR spec: Normal→`-v=2`, Debug→`-v=4`, Trace→`-v=6`, TraceAll→`-v=8`.
- **Error handling:** wrap with `fmt.Errorf("context: %w", err)`.
- **Build flags:** `-tags strictfipsruntime` (FIPS compliance).
- **Upstream changes:** controller framework fixes should go to [library-go](https://github.com/openshift/library-go), not here. CRD type changes go to `pkg/apis/` in this repo, then regenerate clients with `hack/update-codegen.sh` (uses `k8s.io/code-generator`).

## Non-Obvious Internals

- **`controllercmd` intermediary:** The entry point chain (`cmd/` → `pkg/cmd/operator/cmd.go` → `pkg/operator/starter.go`) passes through library-go's `controllercmd.ControllerCommandConfig`, which handles leader election, signal handling, health checks, and serving info. None of this is visible in the operator code itself.
- **Adding a new operand resource:** Requires two steps: (1) add the YAML manifest under `bindata/assets/cli-manager/`, and (2) add a corresponding `manage*` method + call in the `sync()` method of `target_config_reconciler.go`. The `resourceapply` helpers do read-modify-write with last-applied annotation tracking.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for full guidelines. Key rules:

- Do not modify files under `vendor/`. Use `go mod tidy && go mod vendor`.
- `bindata/assets.go` is generated — update the YAML files under `bindata/assets/`, not this file.
- Write unit tests for every change. E2E tests for significant features.

## Testing

- **Unit tests:** co-located `*_test.go` files, table-driven, `make test` or `go test ./pkg/... ./cmd/...`
- **E2E tests:** `test/e2e/` — deploys the operator to a real cluster, creates a Plugin CR, installs it via krew, and verifies execution.
- **OTE framework:** `cli-manager-operator-tests-ext` binary. See [CONTRIBUTING.md](CONTRIBUTING.md#openshift-tests-extension-ote) for usage.
