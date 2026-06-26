# Architecture

## Overview

The CLI Manager Operator is an OpenShift operator that manages the [CLI Manager](https://github.com/openshift/cli-manager) operand — a service that distributes CLI tools and kubectl plugins to cluster users via [krew](https://krew.sigs.k8s.io/). It is deployed by the Cluster Version Operator (CVO) and uses the [library-go](https://github.com/openshift/library-go) controller framework.

The operator's primary responsibilities:
- Watch the singleton `CliManager` custom resource for desired state
- Reconcile operand resources: Deployment, Route, Service, RBAC, ServiceMonitor
- Inject the operand container image and log-level configuration
- Report status via operator conditions and generation tracking

## Data Flow

```text
  CliManager CR (operator.openshift.io/v1)
  (name: "cluster", ns: openshift-cli-manager-operator)
              │
              ▼
  ┌──────────────────────────────────────────────────┐
  │         TargetConfigReconciler                    │
  │  (watches CR, reads embedded YAML assets,         │
  │   substitutes image + log level, applies all      │
  │   operand resources via resourceapply)             │
  └──────────────────────┬───────────────────────────┘
                         │
      ┌──────────────────┼──────────────────┐
      ▼                  ▼                  ▼
  Deployment        Route + Service      RBAC
  (cli-manager      (TLS edge,           (ClusterRole,
   2 replicas)       /cli-manager)        ServiceAccount)
      │
      ▼
  CLI Manager pods
  (serve plugins via krew index)
```

Users interact with the operand through the `oc krew` command, which fetches plugin metadata from the Route and installs CLI tools.

## Operator Startup

Entry point: `cmd/cli-manager-operator/main.go` → `pkg/cmd/operator/cmd.go` → `pkg/operator/starter.go:RunOperator()`.

Startup sequence:
1. Create clients (Kubernetes, dynamic, CliManager, Route)
2. Create informer factory for the CliManager CRD (10-minute resync)
3. Create and start `TargetConfigReconciler` and `ClusterOperatorLoggingController`
4. Block until context cancellation

## Namespace

Everything runs in a single namespace: `openshift-cli-manager-operator` (constant `OperatorNamespace` in `pkg/operator/operatorclient/interfaces.go`). Both the operator Deployment and all operand resources (Deployment, Service, Route, RBAC, ServiceMonitor) live here. The constant `OperandName = "openshift-cli-manager"` is a resource name prefix, not a separate namespace.

## TargetConfigReconciler

`pkg/operator/target_config_reconciler.go` is the single controller. On each sync it:

1. Fetches the `CliManager` CR (`cluster`)
2. Applies RBAC resources (ClusterRole, ClusterRoleBinding, Role, RoleBinding)
3. Applies the operand Deployment:
   - Substitutes `${IMAGE}` with the value of `RELATED_IMAGE_OPERAND_IMAGE`
   - Sets klog verbosity based on CR `logLevel` (Normal→`-v=2`, Debug→`-v=4`, Trace→`-v=6`, TraceAll→`-v=8`)
   - Optionally enables `--serve-artifacts-in-http` for testing
4. Applies networking resources (Route, Service, metrics Service)
5. Applies the ServiceAccount
6. Applies the ServiceMonitor for Prometheus scraping
7. Updates CR status with deployment generation

The reconciler uses a rate-limited work queue. A worker goroutine blocks on the queue and processes items immediately as they arrive; `wait.Until` restarts the worker with a 1-second delay if it exits. Events (add/update/delete) on the `CliManager` CR enqueue a sync.

## Operand Resources

All operand manifests are embedded in `bindata/assets/cli-manager/`:

| Asset | Purpose |
|-------|---------|
| `deployment.yaml` | 2-replica Deployment running `cli-manager start`, ports 9449 (service) and 60000 (metrics) |
| `route.yaml` | TLS edge-terminated Route at path `/cli-manager` with 5-minute HAProxy timeout |
| `service.yaml` | ClusterIP Service on port 9449 |
| `servicemetrics.yaml` | Metrics Service on port 60000 |
| `servicemonitor.yaml` | Prometheus ServiceMonitor |
| `serviceaccount.yaml` | ServiceAccount for the operand |
| `clusterrole.yaml` | ClusterRole for CLI Manager |
| `clusterrolebinding.yaml` | ClusterRoleBinding |
| `role.yaml` | Namespaced Role |
| `rolebinding.yaml` | RoleBinding |

The Deployment runs with a restricted-v2 security context and mounts three emptyDir volumes (`krew-plugins`, `krew-git`, `tmp`) plus a cert secret (`certs-dir`).

## Custom Resource

The `CliManager` CRD (`operator.openshift.io/v1`) embeds the standard `OperatorSpec` and `OperatorStatus` from the OpenShift API:

- **Spec fields** (inherited): `managementState`, `logLevel`, `operatorLogLevel`, `observedConfig`, `unsupportedConfigOverrides`
- **Status fields** (inherited): `conditions`, `generations`, `observedGeneration`, `readyReplicas`, `version`

The CRD is a singleton — the operator expects exactly one instance named `cluster` in the `openshift-cli-manager-operator` namespace.

## Testing Variant

`cmd/cli-manager-operator-testing/` builds an identical operator binary but with `--serve-artifacts-in-http` support (hidden flag). When enabled, the Route uses `InsecureEdgeTerminationPolicy: Allow` instead of `Redirect`, and the operand serves artifacts over HTTP. This exists so e2e tests can fetch artifacts over plain HTTP without dealing with TLS certificate setup in CI.

