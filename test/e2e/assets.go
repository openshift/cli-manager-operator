package e2e

import (
	"github.com/openshift/cli-manager-operator/deploy"
)

func mustDeployAsset(name string) []byte {
	data, err := deploy.Assets.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return data
}
