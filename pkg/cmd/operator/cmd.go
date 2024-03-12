package operator

import (
	"context"
	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/openshift/cli-manager-operator/pkg/version"
)

func NewOperator() *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig("openshift-cli-manager-operator", version.Get(), nil).
		NewCommandWithContext(context.TODO())
	cmd.Use = "operator"
	cmd.Short = "Start the Cluster CLI Manager Operator"

	return cmd
}
