package cmd

import (
	"github.com/quintilesims/go-ecs-cleaner/ecstask"
	"github.com/spf13/cobra"
)

var ecsTaskCmd = &cobra.Command{
	Use:   "ecs-task",
	Short: "Deregister unused task definitions (dry run by default).",
	Long: `Deregister unused task definitions (dry run by default).

BEFORE RUNNING: Make sure that you've properly configured your environment with
the AWS CLI for the AWS account you want to clean up.`,
	Run: func(cmd *cobra.Command, args []string) {
		flags := map[string]interface{}{
			"apply":  applyFlag,
			"cutoff": cutoffFlag,
		}

		ecstask.Run(cmd, args, flags)
	},
}

var applyFlag bool
var cutoffFlag int

func init() {
	ecsTaskCmd.Flags().BoolVarP(&applyFlag, "apply", "a", false, "actually perform task definition deregistration")
	ecsTaskCmd.Flags().IntVarP(&cutoffFlag, "cutoff", "c", 5, "how many most-recent task definitions to keep around")
	rootCmd.AddCommand(ecsTaskCmd)
}
