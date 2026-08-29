package cli

import (
	"io"

	"github.com/spf13/cobra"
)

func NewRootCommand(version string, stdin io.Reader, stdout io.Writer, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "hostero-devkit",
		Short:         "Hostero API developer toolkit",
		Args:          cobra.NoArgs,
		RunE:          showHelp,
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version,
	}

	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetIn(stdin)
	root.AddCommand(
		newInitCommand(),
		newGenerateCommand(),
		newUpdateCommand(),
		newDiffCommand(),
		newValidateCommand(),
	)
	root.InitDefaultCompletionCmd()

	return root
}

func showHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
