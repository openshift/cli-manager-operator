package operator

import (
	"context"
	"k8s.io/utils/clock"

	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/openshift/cli-manager-operator/pkg/operator"
	"github.com/openshift/cli-manager-operator/pkg/version"
)

func NewOperator(supportHttp bool) *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig("openshift-cli-manager-operator", version.Get(), operator.RunOperator, clock.RealClock{}).
		NewCommandWithContext(context.TODO())
	cmd.Use = "operator"
	cmd.Short = "Start the Cluster CLI Manager Operator"

	if supportHttp {
		cmd.Flags().BoolVar(&operator.ServeArtifactAsHttp, "serve-artifacts-in-http", false, "serving artifact in HTTP instead of HTTPS. That is used for testing purposes and not recommended for production")
		cmd.Flags().MarkHidden("serve-artifacts-in-http")
	}

	return cmd
}
