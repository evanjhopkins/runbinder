package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/evanjhopkins/RunBinder/internal/taskconfig"
	"github.com/spf13/cobra"
)

func (c *commands) initCommand() *cobra.Command {
	var register, enable, force bool
	command := &cobra.Command{
		Use:   "init",
		Short: "Create a task definition in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			name := normalizeName(filepath.Base(cwd))
			path := filepath.Join(cwd, name+".runbinder.yaml")
			if _, err := os.Stat(path); err == nil && !force {
				overwrite, err := c.confirm(fmt.Sprintf("File %s exists. Overwrite?", filepath.Base(path)), false)
				if err != nil || !overwrite {
					return err
				}
			}
			namespace, err := c.prompt("Namespace", "myproject."+name+".task1")
			if err != nil {
				return err
			}
			cfg := taskconfig.Config{
				Namespace: namespace,
				Command:   taskconfig.Command{`echo "Running RunBinder task"`, `echo "init task"`},
				Schedule: &taskconfig.Schedule{WindowInterval: &taskconfig.WindowInterval{
					Start: "00:00:00", Stop: "23:59:59", IntervalSec: 60,
				}},
				WorkingDir: cwd,
			}
			if err := c.app.Definitions.Write(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(c.out, "[RUNBINDER] Created %s.\n", path)

			shouldRegister := register
			if !cmd.Flags().Changed("register") {
				shouldRegister, err = c.confirm("Register this task?", true)
				if err != nil {
					return err
				}
			}
			if !shouldRegister {
				return nil
			}
			task, err := c.app.Tasks.Add(cmd.Context(), path, enable)
			if err != nil {
				return err
			}
			fmt.Fprintf(c.out, "[RUNBINDER] Registered task %q.\n", task.Namespace)
			return nil
		},
	}
	command.Flags().BoolVar(&register, "register", false, "register the generated task without prompting")
	command.Flags().BoolVarP(&enable, "enable", "e", false, "enable the task when registering")
	command.Flags().BoolVarP(&force, "force", "f", false, "overwrite an existing definition")
	return command
}
