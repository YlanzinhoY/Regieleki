package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	fileIDFromURLPattern = regexp.MustCompile("(?i)(?:pixeldrain\\.com/u/|pixeldrain\\.eu\\.cc/api/file/)([A-Za-z0-9_]+)")
	fileIDPattern        = regexp.MustCompile("^[A-Za-z0-9_]+$")
)

type Conversion struct {
	FileID      string
	DownloadURL string
}

func convertInput(input string) (Conversion, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Conversion{}, errors.New("informe o ID do arquivo")
	}

	fileID := input
	if match := fileIDFromURLPattern.FindStringSubmatch(input); len(match) == 2 {
		fileID = match[1]
	}
	if !fileIDPattern.MatchString(fileID) {
		return Conversion{}, errors.New("ID de arquivo invalido")
	}

	return Conversion{
		FileID:      fileID,
		DownloadURL: fmt.Sprintf("https://cdn18.pixeldrain.eu.cc/api/file/%s", fileID),
	}, nil
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1).
			Width(64)
	resultStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42")).
			Foreground(lipgloss.Color("42")).
			Padding(1, 2).
			Width(64)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("204")).
			MarginTop(1)
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)

type model struct {
	input        string
	result       *Conversion
	errorMessage string
}

func newModel(initialInput string) model {
	return model{input: initialInput}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	keyMessage, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMessage.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyEnter:
		if m.result != nil {
			m.input = ""
			m.result = nil
			m.errorMessage = ""
			return m, nil
		}
		conversion, err := convertInput(m.input)
		if err != nil {
			m.errorMessage = err.Error()
			return m, nil
		}
		m.result = &conversion
		m.errorMessage = ""
		return m, nil
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		m.errorMessage = ""
		return m, nil
	case tea.KeyRunes:
		m.input += string(keyMessage.Runes)
		m.errorMessage = ""
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	content := []string{
		titleStyle.Render("PixelDrain Worker"),
		"Digite o ID do arquivo:",
		inputStyle.Render(m.input + ">"),
	}

	if m.result != nil {
		content = append(content,
			"",
			resultStyle.Render(fmt.Sprintf(
				"ID: %s\n\nURL de download:\n%s",
				m.result.FileID,
				m.result.DownloadURL,
			)),
			helpStyle.Render("Pressione Enter para converter outro arquivo ou Esc para sair."),
		)
	} else {
		if m.errorMessage != "" {
			content = append(content, errorStyle.Render("Erro: "+m.errorMessage))
		}
		content = append(content, helpStyle.Render("Enter gera a URL  |  Esc/Ctrl+C sai"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, content...)
}

func runInteractive(initialInput string) error {
	program := tea.NewProgram(newModel(initialInput), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

var rootCommand = &cobra.Command{
	Use:   "pixeldrain-worker",
	Short: "Gera URLs de download do PixelDrain",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runInteractive("")
	},
}

var convertCommand = &cobra.Command{
	Use:   "convert [id-ou-url]",
	Short: "Gera uma URL de download a partir do ID",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, arguments []string) error {
		if len(arguments) == 0 {
			return runInteractive("")
		}

		conversion, err := convertInput(arguments[0])
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(command.OutOrStdout(), conversion.DownloadURL)
		return err
	},
}

func main() {
	rootCommand.AddCommand(convertCommand)
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
