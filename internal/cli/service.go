package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/app"
	"github.com/spf13/cobra"
)

func (c *commands) serviceCommand() *cobra.Command {
	var concurrency int
	var misfireGrace time.Duration
	var detach, detachedChild bool
	command := &cobra.Command{
		Use:   "service",
		Short: "Run the RunBinder scheduling service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if concurrency < 1 {
				return errors.New("concurrency must be at least 1")
			}
			if detach && detachedChild {
				return errors.New("detach and detached-child cannot be used together")
			}
			options := app.ServiceOptions{Concurrency: concurrency, MisfireGrace: misfireGrace}
			if detach {
				pid, err := c.app.Service.StartDetached(cmd.Context(), options)
				if err != nil {
					return err
				}
				fmt.Fprintf(c.out, "[RUNBINDER] Service started in the background (PID %d).\n", pid)
				return nil
			}
			serviceCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if !detachedChild {
				fmt.Fprintln(c.out, "RunBinder service started. Press Ctrl+C to stop.")
			}
			return c.app.Service.Run(serviceCtx, options)
		},
	}
	command.Flags().IntVarP(&concurrency, "concurrency", "j", 4, "maximum number of tasks to run concurrently")
	command.Flags().DurationVar(&misfireGrace, "misfire-grace", time.Minute, "maximum age of a delayed occurrence to run")
	command.Flags().BoolVarP(&detach, "detach", "d", false, "run the service in the background")
	command.Flags().BoolVar(&detachedChild, "detached-child", false, "internal detached service mode")
	_ = command.Flags().MarkHidden("detached-child")
	command.AddCommand(c.stopServiceCommand())
	return command
}

func (c *commands) stopServiceCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stopped, err := c.app.Service.Stop(cmd.Context())
			if err != nil {
				return err
			}
			if stopped {
				fmt.Fprintln(c.out, "[RUNBINDER] Service stopped.")
			} else {
				fmt.Fprintln(c.out, "[RUNBINDER] Service is not running.")
			}
			return nil
		},
	}
}
