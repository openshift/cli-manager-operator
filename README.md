File based catalog configurations of CLI Manager Operator

Prepare an initial catalog/index.yaml and after that execute with the correct bundle version:

```shell
$ opm render registry.redhat.io/cli-manager-operator/cli-manager-operator-bundle:0.0.1 --output=yaml >> catalog/index.yaml
```