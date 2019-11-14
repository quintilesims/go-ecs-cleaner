package cmd

import (
	"github.com/quintilesims/go-ecs-cleaner/ecs_task"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(ecsTaskCmd)
}

var ecsTaskCmd = &cobra.Command{
	Use:   "ecs-task",
	Short: "Marks stale and unused ECS tasks as inactive",
	Run:   ecs_task.Run,
}
