package main

import (
	"os"

	"github.com/spf13/cobra"
	"k8s.io/component-base/cli"

	"github.com/openshift/cli-manager-operator/pkg/cmd/operator"
)

func main() {
	command := NewCLIManagerOperatorCommand()
	code := cli.Run(command)
	os.Exit(code)
}

func NewCLIManagerOperatorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cli-manager-operator",
		Short: "OpenShift CLI Manager operator",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	cmd.AddCommand(operator.NewOperator(true))
	return cmd
}
