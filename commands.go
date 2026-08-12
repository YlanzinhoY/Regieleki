package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var outputDirectory string

var rootCommand = &cobra.Command{
	Use:   "regieleki",
	Short: "Blazing fast PixelDrain TUI downloader",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runInteractive(outputDirectory)
	},
}

var convertCommand = &cobra.Command{
	Use:   "convert [id-or-url]",
	Short: "Generate the download URL from the file ID",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, arguments []string) error {
		if len(arguments) == 0 {
			return runInteractive(outputDirectory)
		}

		conversion, err := convertInput(arguments[0])
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(command.OutOrStdout(), conversion.DownloadURL)
		return err
	},
}

func init() {
	cobra.MousetrapHelpText = ""
	rootCommand.PersistentFlags().StringVarP(
		&outputDirectory,
		"output-dir",
		"o",
		defaultDownloadDirectory(),
		"directory where files will be saved",
	)
	rootCommand.PersistentFlags().Lookup("output-dir").Usage = "directory where files will be saved"
	rootCommand.AddCommand(convertCommand)
}
