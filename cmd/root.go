package cmd

import (
	"github.com/spf13/cobra"
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:   "go-ecs-cleaner",
	Short: "Clean up your ECS",
	Long:  "A Go tool to clean up your ECS account, based upon https://github.com/FernandoMiguel/ecs-cleaner",
}
