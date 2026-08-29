package cli

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/evanjhopkins/RunBinder/internal/app"
	"github.com/spf13/cobra"
)

func (c *commands) listCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			summaries, err := c.app.Tasks.List(cmd.Context())
			if err != nil {
				return err
			}
			if len(summaries) == 0 {
				fmt.Fprintln(c.out, "[RUNBINDER] No tasks have been registered.")
				return nil
			}
			var output bytes.Buffer
			writer := tabwriter.NewWriter(&output, 0, 4, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tNAMESPACE\tACTIVE\tYAML\tDIRECTORY\tLAST RUN")
			rows := make([]taskListRow, 0, len(summaries))
			for _, summary := range summaries {
				lastRun := "(none)"
				lastRunSuccess := false
				hasLastRun := summary.LastRun != nil
				if summary.LastRun != nil {
					status := "FAIL"
					if summary.LastRun.Success {
						status = "SUCC"
						lastRunSuccess = true
					}
					lastRun = summary.LastRun.StartedAt.Local().Format("2006-01-02 15:04:05") + " (" + status + ")"
				}
				task := summary.Task
				active := strconv.FormatBool(task.Active)
				fmt.Fprintf(writer, "%d\t%s\t%s\t%s\t%s\t%s\n", task.ID, task.Namespace, active, summary.Definition, task.WorkingDir, lastRun)
				rows = append(rows, taskListRow{
					namespace:  task.Namespace,
					active:     active,
					definition: summary.Definition,
					workingDir: task.WorkingDir,
					lastRun:    lastRun,
					enabled:    task.Active,
					success:    lastRunSuccess,
					hasRun:     hasLastRun,
				})
			}
			if err := writer.Flush(); err != nil {
				return err
			}
			lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
			fmt.Fprintln(c.out, lines[0])
			for index, row := range rows {
				fmt.Fprintln(c.out, c.taskListLine(lines[index+1], row))
			}
			return nil
		},
	}
}

type taskListRow struct {
	namespace  string
	active     string
	definition app.DefinitionState
	workingDir string
	lastRun    string
	enabled    bool
	success    bool
	hasRun     bool
}

func (c *commands) statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show service health and recent internal logs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			status, err := c.app.Service.Status(cmd.Context())
			if err != nil {
				return err
			}
			last, pid := "(none)", "(none)"
			if status.Heartbeat != nil {
				last = status.Heartbeat.Last.Local().Format("2006-01-02 15:04:05")
			}
			if status.PID > 0 {
				pid = strconv.Itoa(status.PID)
			}
			running := strings.ToUpper(strconv.FormatBool(status.Running))
			fmt.Fprintln(c.out, c.serviceStatusLine(status.Running, "Is Service Running: "+running))
			fmt.Fprintf(c.out, "Service PID: %s\n", pid)
			fmt.Fprintf(c.out, "Last Heartbeat: %s\n", last)
			fmt.Fprintf(c.out, "Internal Storage: %s\n", status.StorageDir)
			fmt.Fprintln(c.out, "Recent Logs:")
			if len(status.RecentLogs) == 0 {
				fmt.Fprintln(c.out, "(none)")
			}
			for _, line := range status.RecentLogs {
				fmt.Fprintln(c.out, c.serviceLogLine("-> "+line))
			}
			return nil
		},
	}
}

func (c *commands) logCommand() *cobra.Command {
	var lines int
	command := &cobra.Command{
		Use:   "log <namespace-or-task-file>",
		Short: "Show recent output from a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := c.app.Tasks.Log(cmd.Context(), args[0], lines)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintln(c.out, "(none)")
			}
			for _, entry := range entries {
				fmt.Fprintln(c.out, entry)
			}
			return nil
		},
	}
	command.Flags().IntVarP(&lines, "lines", "n", 20, "number of lines to show")
	return command
}
