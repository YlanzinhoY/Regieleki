package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const uiWidth = 72

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("220")).
			Padding(0, 1).
			MarginBottom(1)
	versionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1).
			Width(uiWidth)
	cursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("220"))
	pasteButtonStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Foreground(lipgloss.Color("252")).
				Padding(0, 1)
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(uiWidth)
	successStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42")).
			Foreground(lipgloss.Color("42")).
			Padding(1, 2).
			Width(uiWidth)
	errorStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("204")).
			Foreground(lipgloss.Color("204")).
			Padding(1, 2).
			Width(uiWidth)
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
	Mirror     int
}

type cursorBlinkMsg struct{}

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
	mirror        int
	cursorVisible bool
	outputPath    string
	downloadError error
	cdnBlocked    bool
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
	return blinkCursor()
}

func (model *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.KeyMsg:
		return model.updateKey(message)
	case cursorBlinkMsg:
		model.cursorVisible = !model.cursorVisible
		return model, blinkCursor()
	case downloadProgressMsg:
		if model.state == stateDownloading {
			model.downloaded = message.Downloaded
			model.total = message.Total
			model.speed = message.Speed
			if message.Mirror > 0 {
				model.mirror = message.Mirror
			}
		}
	case downloadCompletedMsg:
		model.state = stateCompleted
		model.downloaded = message.Result.Downloaded
		model.total = message.Result.Total
		model.speed = message.Result.AverageSpeed
		model.mirror = message.Result.Mirror
		model.outputPath = message.Result.Path
	case downloadFailedMsg:
		model.state = stateError
		model.downloadError = message.Err
		var limitError *cdnLimitError
		model.cdnBlocked = errors.As(message.Err, &limitError)
		var mirrorError *cdnMirrorError
		if errors.As(message.Err, &mirrorError) {
			model.mirror = mirrorError.Mirror
		} else if model.cdnBlocked && limitError.Mirror > 0 {
			model.mirror = limitError.Mirror
		}
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
			if model.cdnBlocked {
				model.state = stateInput
				model.downloadError = nil
				model.conversion = nil
				model.cdnBlocked = false
				return model, nil
			}
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
			pastedInput := string(message.Runes)
			if message.Paste {
				model.input = normalizeInput(pastedInput)
			} else {
				model.input += pastedInput
			}
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
	model.mirror = 0
	model.downloadError = nil
	model.cdnBlocked = false
	return startDownload(model.downloadCtx, conversion, model.outputDir, model.send)
}

func (model *model) reset() {
	model.input = ""
	model.conversion = nil
	model.state = stateInput
	model.downloaded = 0
	model.total = 0
	model.speed = 0
	model.mirror = 0
	model.outputPath = ""
	model.downloadError = nil
	model.cdnBlocked = false
}

func (model *model) View() string {
	content := []string{
		titleStyle.Render("⚡ Regieleki ⚡"),
		"Blazing fast PixelDrain TUI downloader",
		versionStyle.Render(fmt.Sprintf("Version %s", version)),
		"Enter the file ID:",
		model.inputView(),
		pasteButtonStyle.Render("Paste ID: Ctrl+Shift+V / Ctrl+V"),
	}

	switch model.state {
	case stateDownloading:
		content = append(content, "", model.downloadView())
	case stateCompleted:
		content = append(content, "", model.completedView())
	case stateError:
		errorTitle := "Download failed"
		if model.mirror > 0 {
			errorTitle = fmt.Sprintf("Download failed on Mirror %d", model.mirror)
		}
		errorMessage := fmt.Sprintf("%s:\n%s", errorTitle, model.downloadError)
		content = append(content, "", errorStyle.Render(errorMessage))
		if model.cdnBlocked {
			content = append(content, helpStyle.Render("The CDN limit was reached. Press Enter to enter another ID or Esc to exit."))
		} else {
			content = append(content, helpStyle.Render("Press Enter to try again or Esc to exit."))
		}
	default:
		content = append(content, helpStyle.Render("Enter downloads the file  |  Esc/Ctrl+C exits"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, content...)
}

func (model *model) inputView() string {
	cursor := " "
	if model.state == stateInput && model.cursorVisible {
		cursor = cursorStyle.Render("▌")
	}
	return inputStyle.Render(model.input + cursor)
}

func blinkCursor() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return cursorBlinkMsg{}
	})
}

func (model *model) downloadView() string {
	progress := progressBar(model.downloaded, model.total, 42)
	size := formatBytes(model.downloaded)
	if model.total > 0 {
		percentage := float64(model.downloaded) / float64(model.total) * 100
		size = fmt.Sprintf("%s / %s (%.1f%%)", size, formatBytes(model.total), percentage)
	} else {
		size += " / unknown size"
	}

	return panelStyle.Render(fmt.Sprintf(
		"Downloading: %s\nMirror: %d\n\n%s\n%s\nSpeed: %s\nDestination: %s",
		model.conversion.FileID,
		model.mirror,
		progress,
		size,
		formatSpeed(model.speed),
		model.outputDir,
	))
}

func (model *model) completedView() string {
	return successStyle.Render(fmt.Sprintf(
		"Download completed on Mirror %d!\n\nFile: %s\nSize: %s\nAverage speed: %s\nSaved to: %s",
		model.mirror,
		model.conversion.FileID,
		formatBytes(model.downloaded),
		formatSpeed(model.speed),
		model.outputPath,
	))
}

func progressBar(downloaded int64, total int64, width int) string {
	if total <= 0 {
		return "[ streaming | unknown size ]"
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
		return "unknown"
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
		return "calculating..."
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
						Mirror:     progress.Mirror,
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
	configureConsoleWindow()
	downloadContext, cancel := context.WithCancel(context.Background())
	appModel := newModel(outputDir, downloadContext, cancel)
	program := tea.NewProgram(appModel, tea.WithAltScreen())
	appModel.send = program.Send
	_, err := program.Run()
	cancel()
	return err
}
