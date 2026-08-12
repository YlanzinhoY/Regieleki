package main

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(64)
	successStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42")).
			Foreground(lipgloss.Color("42")).
			Padding(1, 2).
			Width(64)
	errorStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("204")).
			Foreground(lipgloss.Color("204")).
			Padding(1, 2).
			Width(64)
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)

type screenState uint8

const (
	stateInput screenState = iota
	stateDownloading
	stateCompleted
	stateError
)

type downloadProgressMsg struct {
	Downloaded int64
	Total      int64
	Speed      float64
}

type downloadCompletedMsg struct {
	Result downloadResult
}

type downloadFailedMsg struct {
	Err error
}

type model struct {
	input         string
	conversion    *Conversion
	state         screenState
	downloaded    int64
	total         int64
	speed         float64
	outputPath    string
	downloadError error
	outputDir     string
	downloadCtx   context.Context
	cancel        context.CancelFunc
	send          func(tea.Msg)
}

func newModel(outputDir string, downloadContext context.Context, cancel context.CancelFunc) *model {
	return &model{
		state:       stateInput,
		outputDir:   outputDir,
		downloadCtx: downloadContext,
		cancel:      cancel,
	}
}

func (model *model) Init() tea.Cmd {
	return nil
}

func (model *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.KeyMsg:
		return model.updateKey(message)
	case downloadProgressMsg:
		if model.state == stateDownloading {
			model.downloaded = message.Downloaded
			model.total = message.Total
			model.speed = message.Speed
		}
	case downloadCompletedMsg:
		model.state = stateCompleted
		model.downloaded = message.Result.Downloaded
		model.total = message.Result.Total
		model.speed = message.Result.AverageSpeed
		model.outputPath = message.Result.Path
	case downloadFailedMsg:
		model.state = stateError
		model.downloadError = message.Err
	}

	return model, nil
}

func (model *model) updateKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		if model.cancel != nil {
			model.cancel()
		}
		return model, tea.Quit
	case tea.KeyEnter:
		if model.state == stateDownloading {
			return model, nil
		}
		if model.state == stateCompleted {
			model.reset()
			return model, nil
		}
		if model.state == stateError && model.conversion != nil {
			return model, model.beginDownload(*model.conversion)
		}
		if model.state == stateError {
			model.state = stateInput
			model.downloadError = nil
			return model, nil
		}

		conversion, err := convertInput(model.input)
		if err != nil {
			model.downloadError = err
			model.state = stateError
			return model, nil
		}

		return model, model.beginDownload(conversion)
	case tea.KeyBackspace, tea.KeyDelete:
		if model.state == stateInput {
			inputRunes := []rune(model.input)
			if len(inputRunes) > 0 {
				model.input = string(inputRunes[:len(inputRunes)-1])
			}
		}
	case tea.KeyRunes:
		if model.state == stateInput {
			model.input += string(message.Runes)
		}
	}

	return model, nil
}

func (model *model) beginDownload(conversion Conversion) tea.Cmd {
	model.conversion = &conversion
	model.state = stateDownloading
	model.downloaded = 0
	model.total = 0
	model.speed = 0
	model.downloadError = nil
	return startDownload(model.downloadCtx, conversion, model.outputDir, model.send)
}

func (model *model) reset() {
	model.input = ""
	model.conversion = nil
	model.state = stateInput
	model.downloaded = 0
	model.total = 0
	model.speed = 0
	model.outputPath = ""
	model.downloadError = nil
}

func (model *model) View() string {
	content := []string{
		titleStyle.Render("Regieleki"),
		"Blazing fast PixelDrain TUI downloader",
		"Digite o ID do arquivo:",
		inputStyle.Render(model.input + ">"),
	}

	switch model.state {
	case stateDownloading:
		content = append(content, "", model.downloadView())
	case stateCompleted:
		content = append(content, "", model.completedView())
	case stateError:
		errorMessage := fmt.Sprintf("Falha no download:\n%s", model.downloadError)
		content = append(content, "", errorStyle.Render(errorMessage))
		content = append(content, helpStyle.Render("Pressione Enter para tentar novamente ou Esc para sair."))
	default:
		content = append(content, helpStyle.Render("Enter baixa o arquivo  |  Esc/Ctrl+C sai"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, content...)
}

func (model *model) downloadView() string {
	progress := progressBar(model.downloaded, model.total, 42)
	size := formatBytes(model.downloaded)
	if model.total > 0 {
		percentage := float64(model.downloaded) / float64(model.total) * 100
		size = fmt.Sprintf("%s / %s (%.1f%%)", size, formatBytes(model.total), percentage)
	} else {
		size += " / tamanho desconhecido"
	}

	return panelStyle.Render(fmt.Sprintf(
		"Baixando: %s\n\n%s\n%s\nVelocidade: %s\nDestino: %s",
		model.conversion.FileID,
		progress,
		size,
		formatSpeed(model.speed),
		model.outputDir,
	))
}

func (model *model) completedView() string {
	return successStyle.Render(fmt.Sprintf(
		"Download concluido!\n\nArquivo: %s\nTamanho: %s\nVelocidade media: %s\nSalvo em: %s",
		model.conversion.FileID,
		formatBytes(model.downloaded),
		formatSpeed(model.speed),
		model.outputPath,
	))
}

func progressBar(downloaded int64, total int64, width int) string {
	if total <= 0 {
		return "[ streaming | tamanho desconhecido ]"
	}

	ratio := float64(downloaded) / float64(total)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func formatBytes(bytes int64) string {
	if bytes < 0 {
		return "desconhecido"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(bytes)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", bytes, units[unit])
	}
	return fmt.Sprintf("%.2f %s", value, units[unit])
}

func formatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 {
		return "calculando..."
	}
	return formatBytes(int64(bytesPerSecond)) + "/s"
}

func startDownload(
	downloadContext context.Context,
	conversion Conversion,
	outputDir string,
	send func(tea.Msg),
) tea.Cmd {
	return func() tea.Msg {
		go func() {
			result, err := downloadFile(downloadContext, conversion, outputDir, func(progress downloadProgress) {
				if send != nil {
					send(downloadProgressMsg{
						Downloaded: progress.Downloaded,
						Total:      progress.Total,
						Speed:      progress.Speed,
					})
				}
			})
			if send == nil {
				return
			}
			if err != nil {
				send(downloadFailedMsg{Err: err})
				return
			}
			send(downloadCompletedMsg{Result: result})
		}()
		return nil
	}
}

func runInteractive(outputDir string) error {
	downloadContext, cancel := context.WithCancel(context.Background())
	appModel := newModel(outputDir, downloadContext, cancel)
	program := tea.NewProgram(appModel, tea.WithAltScreen())
	appModel.send = program.Send
	_, err := program.Run()
	cancel()
	return err
}
