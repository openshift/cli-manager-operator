File based catalog configurations of CLI Manager Operator

Prepare an initial catalog/index.yaml and after that execute with the correct bundle version:

```shell
$ opm render registry.redhat.io/cli-manager-operator/cli-manager-operator-bundle@sha256:51c90b8d9f243e3ada2aec161441e1c641ba2ee69096afc8d2832fc6981bad2a --output=yaml >> catalog/index.yaml
```