package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (c *commands) addCommand() *cobra.Command {
	var enabled bool
	command := &cobra.Command{
		Use:   "add <task-file>",
		Short: "Register a task definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task, err := c.app.Tasks.Add(cmd.Context(), args[0], enabled)
			if err != nil {
				return err
			}
			fmt.Fprintf(c.out, "[RUNBINDER] Registered task %q.\n", task.Namespace)
			return nil
		},
	}
	command.Flags().BoolVarP(&enabled, "enable", "e", false, "enable the task immediately")
	return command
}

func (c *commands) updateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update <task-file>",
		Short: "Update a registered task from its definition file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task, err := c.app.Tasks.Update(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(c.out, "[RUNBINDER] Updated task %q.\n", task.Namespace)
			return nil
		},
	}
}

func (c *commands) stateCommand(name string, active bool) *cobra.Command {
	verb := "Enable"
	if !active {
		verb = "Disable"
	}
	return &cobra.Command{
		Use:   name + " <namespace-or-task-file>",
		Short: verb + " a registered task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task, changed, err := c.app.Tasks.SetActive(cmd.Context(), args[0], active)
			if err != nil {
				return err
			}
			if !changed {
				fmt.Fprintf(c.out, "[RUNBINDER] Task %q is already %s.\n", task.Namespace, stateName(active))
				return nil
			}
			fmt.Fprintf(c.out, "[RUNBINDER] Task %q is now %s.\n", task.Namespace, stateName(active))
			return nil
		},
	}
}

func (c *commands) removeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <namespace-or-task-file>",
		Short: "Remove a task registration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task, err := c.app.Tasks.Remove(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(c.out, "[RUNBINDER] Removed task %q.\n", task.Namespace)
			return nil
		},
	}
}

func (c *commands) runCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run <namespace-or-task-file>",
		Short: "Run a task immediately",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task, err := c.app.Tasks.Run(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(c.out, "[RUNBINDER] Task %q completed successfully.\n", task.Namespace)
			return nil
		},
	}
}

func stateName(active bool) string {
	if active {
		return "enabled"
	}
	return "disabled"
}
