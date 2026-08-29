package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (c *commands) nukeCommand() *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "nuke",
		Short: "Delete RunBinder's database and all registrations",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !yes {
				confirmed, err := c.confirm("Delete all task registrations and run history?", false)
				if err != nil || !confirmed {
					return err
				}
			}
			removed, err := c.app.Service.Reset()
			if err != nil {
				return err
			}
			if removed {
				fmt.Fprintln(c.out, "[RUNBINDER] Deleted the RunBinder database.")
			} else {
				fmt.Fprintln(c.out, "[RUNBINDER] No database to delete.")
			}
			return nil
		},
	}
	command.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return command
}
