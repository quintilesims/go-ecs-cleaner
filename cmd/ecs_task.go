package cmd

import (
	"fmt"
	"os"

	"github.com/quintilesims/go-ecs-cleaner/ecsclient"
	"github.com/spf13/cobra"
)

var applyFlag bool
var cutoffFlag int
var debugFlag bool
var quietFlag bool
var verboseFlag bool

func init() {
	ecsTaskCmd.Flags().BoolVarP(&applyFlag, "apply", "a", false, "actually perform task definition deregistration")
	ecsTaskCmd.Flags().IntVarP(&cutoffFlag, "cutoff", "c", 5, "how many most-recent task definitions to keep around")
	ecsTaskCmd.Flags().BoolVarP(&debugFlag, "debug", "d", false, "enable for all the output")
	ecsTaskCmd.Flags().BoolVarP(&quietFlag, "quiet", "q", false, "minimize output")
	ecsTaskCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "enable for chattier output")
	rootCmd.AddCommand(ecsTaskCmd)
}

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
		if debugFlag {
			verboseFlag = true
		}

		if quietFlag && verboseFlag {
			fmt.Println("Can't set quiet flag alongside verbose or debug flags.")
			os.Exit(1)
		}

		ecsClient := ecsclient.NewECSClient()

		if err := ecsClient.ConfigureSession(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		ecsClient.Flags.Apply = applyFlag
		ecsClient.Flags.Cutoff = cutoffFlag
		ecsClient.Flags.Debug = debugFlag
		ecsClient.Flags.Quiet = quietFlag
		ecsClient.Flags.Verbose = verboseFlag

		if err := ecsClient.CleanupTaskDefinitions(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}
