package cmd

import (
	"fmt"
	"os"

	"github.com/quintilesims/go-ecs-cleaner/ecstask"
	"github.com/spf13/cobra"
)

var ecsTaskCmd = &cobra.Command{
	Use:   "ecs-task",
	Short: "Deregister unused task definitions (dry run by default).",
	Long: `Deregister unused task definitions (dry run by default).

BEFORE RUNNING: Make sure that you've properly configured your environment with
the following environment variables:

AWS_ACCESS_KEY
AWS_SECRET_ACCESS_KEY
AWS_REGION`,
	Run: func(cmd *cobra.Command, args []string) {
		if parallelFlag < 1 {
			fmt.Println("minimum parallel is 1")
			os.Exit(1)
		}

		if debugFlag {
			verboseFlag = true
		}

		flags := map[string]interface{}{
			"apply":    applyFlag,
			"cutoff":   cutoffFlag,
			"debug":    debugFlag,
			"parallel": parallelFlag,
			"verbose":  verboseFlag,
		}

		ecstask.Run(cmd, args, flags)
	},
}

var applyFlag bool
var cutoffFlag int
var debugFlag bool
var parallelFlag int

var verboseFlag bool

func init() {
	ecsTaskCmd.Flags().BoolVarP(&applyFlag, "apply", "a", false, "actually perform task definition deregistration")
	ecsTaskCmd.Flags().IntVarP(&cutoffFlag, "cutoff", "c", 5, "how many most-recent task definitions to keep around")
	ecsTaskCmd.Flags().BoolVarP(&debugFlag, "debug", "d", false, "enable for all the output")
	ecsTaskCmd.Flags().IntVarP(&parallelFlag, "parallel", "p", 2, "how many concurrent deregistration requests to make")
	ecsTaskCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "enable for chattier output")
	rootCmd.AddCommand(ecsTaskCmd)
}
