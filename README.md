File based catalog configurations of CLI Manager Operator

Prepare an initial catalog/index.yaml and after that execute with the correct bundle version:

```shell
$ opm render registry.redhat.io/cli-manager/cli-manager-operator-bundle@sha256:1f083f8a6235c4313c6cefa1d0c6ec62cab7b0fd7a58c4ce6f1c271165030112 --output=yaml >> catalog/index.yaml
```