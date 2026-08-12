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
	Use:   "convert [id-ou-url]",
	Short: "Gera a URL de download a partir do ID",
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
		".",
		"pasta onde os arquivos serão salvos",
	)
	rootCommand.AddCommand(convertCommand)
}
