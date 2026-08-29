package cli

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"github.com/evanjhopkins/RunBinder/internal/app"
	"github.com/evanjhopkins/RunBinder/internal/platform"
	"github.com/spf13/cobra"
)

const version = "0.2.0"

type commands struct {
	app    *app.Application
	in     io.Reader
	out    io.Writer
	reader *bufio.Reader
}

func Execute() error {
	paths, err := platform.ResolvePaths()
	if err != nil {
		return err
	}
	commands := &commands{app: app.New(paths), in: os.Stdin, out: os.Stdout}
	return commands.rootCommand().Execute()
}

func (c *commands) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           filepath.Base(os.Args[0]),
		Short:         "File-based scheduled process execution",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetIn(c.in)
	root.SetOut(c.out)
	root.AddCommand(
		c.addCommand(),
		c.updateCommand(),
		c.stateCommand("enable", true),
		c.stateCommand("disable", false),
		c.removeCommand(),
		c.listCommand(),
		c.statusCommand(),
		c.logCommand(),
		c.runCommand(),
		c.serviceCommand(),
		c.initCommand(),
		c.nukeCommand(),
	)
	return root
}
