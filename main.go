package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const workerBaseURL = "https://cdn18.pixeldrain.eu.cc/api/file"

var (
	fileIDFromURLPattern = regexp.MustCompile("(?i)(?:pixeldrain\\.com/u/|pixeldrain\\.eu\\.cc/api/file/)([A-Za-z0-9_]+)")
	fileIDPattern        = regexp.MustCompile("^[A-Za-z0-9_]+$")
	outputDirectory      string
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
		DownloadURL: fmt.Sprintf("%s/%s", workerBaseURL, fileID),
	}, nil
}

type downloadProgress struct {
	Downloaded int64
	Total      int64
	Speed      float64
}

type downloadResult struct {
	Path         string
	Downloaded   int64
	Total        int64
	Elapsed      time.Duration
	AverageSpeed float64
}

type progressWriter struct {
	writer     io.Writer
	downloaded int64
	total      int64
	startedAt  time.Time
	lastReport time.Time
	report     func(downloadProgress)
}

func (writer *progressWriter) Write(chunk []byte) (int, error) {
	bytesWritten, err := writer.writer.Write(chunk)
	writer.downloaded += int64(bytesWritten)

	now := time.Now()
	if writer.lastReport.IsZero() || now.Sub(writer.lastReport) >= 100*time.Millisecond || err != nil {
		writer.emit(now)
	}

	return bytesWritten, err
}

func (writer *progressWriter) emit(now time.Time) {
	elapsed := now.Sub(writer.startedAt)
	speed := float64(writer.downloaded)
	if elapsed > 0 {
		speed /= elapsed.Seconds()
	}
	writer.report(downloadProgress{
		Downloaded: writer.downloaded,
		Total:      writer.total,
		Speed:      speed,
	})
	writer.lastReport = now
}

func downloadFile(
	ctx context.Context,
	conversion Conversion,
	directory string,
	report func(downloadProgress),
) (downloadResult, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, conversion.DownloadURL, nil)
	if err != nil {
		return downloadResult{}, fmt.Errorf("criando requisicao: %w", err)
	}
	request.Header.Set("User-Agent", "pixeldrain-worker/1.0")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return downloadResult{}, fmt.Errorf("conectando ao worker: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 2048))
		message := compactMessage(string(body))
		if readErr != nil || message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return downloadResult{}, fmt.Errorf("worker respondeu HTTP %d: %s", response.StatusCode, message)
	}

	filename := responseFilename(response, conversion.FileID)
	file, path, err := createOutputFile(directory, filename)
	if err != nil {
		return downloadResult{}, err
	}

	keepFile := false
	defer func() {
		if !keepFile {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()

	startedAt := time.Now()
	writer := &progressWriter{
		writer:    file,
		total:     response.ContentLength,
		startedAt: startedAt,
		report:    report,
	}
	_, err = io.CopyBuffer(writer, response.Body, make([]byte, 64*1024))
	if err != nil {
		return downloadResult{}, fmt.Errorf("baixando arquivo: %w", err)
	}
	writer.emit(time.Now())

	if err := file.Close(); err != nil {
		return downloadResult{}, fmt.Errorf("fechando arquivo: %w", err)
	}
	keepFile = true

	elapsed := time.Since(startedAt)
	if elapsed <= 0 {
		elapsed = time.Nanosecond
	}
	averageSpeed := float64(writer.downloaded) / elapsed.Seconds()

	return downloadResult{
		Path:         path,
		Downloaded:   writer.downloaded,
		Total:        response.ContentLength,
		Elapsed:      elapsed,
		AverageSpeed: averageSpeed,
	}, nil
}

func responseFilename(response *http.Response, fileID string) string {
	disposition := response.Header.Get("Content-Disposition")
	if disposition != "" {
		_, parameters, err := mime.ParseMediaType(disposition)
		if err == nil {
			for _, key := range []string{"filename", "filename*"} {
				if name := safeFilename(parameters[key], ""); name != "" {
					return name
				}
			}
		}
	}
	return "file_" + fileID
}

func compactMessage(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 240 {
		return message[:240] + "..."
	}
	return message
}

func safeFilename(name string, fallback string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\x00", "")
	name = filepath.Base(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return fallback
	}
	return name
}

func createOutputFile(directory string, filename string) (*os.File, string, error) {
	if directory == "" {
		directory = "."
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, "", fmt.Errorf("criando pasta de destino: %w", err)
	}

	filename = safeFilename(filename, "download")
	extension := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, extension)

	for attempt := 0; attempt < 10000; attempt++ {
		candidate := filename
		if attempt > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, attempt, extension)
		}
		path := filepath.Join(directory, candidate)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return file, path, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", fmt.Errorf("criando arquivo de destino: %w", err)
		}
	}

	return nil, "", errors.New("nao foi possivel encontrar um nome de arquivo livre")
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
	context       context.Context
	cancel        context.CancelFunc
	send          func(tea.Msg)
}

func newModel(outputDir string, downloadContext context.Context, cancel context.CancelFunc) *model {
	return &model{
		state:     stateInput,
		outputDir: outputDir,
		context:   downloadContext,
		cancel:    cancel,
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
	return startDownload(model.context, conversion, model.outputDir, model.send)
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
		titleStyle.Render("PixelDrain Downloader"),
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
		if model.conversion != nil {
			errorMessage += fmt.Sprintf("\n\nURL:\n%s", model.conversion.DownloadURL)
		}
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

var rootCommand = &cobra.Command{
	Use:   "pixeldrain-worker",
	Short: "Baixa arquivos do PixelDrain usando a TUI",
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
	rootCommand.PersistentFlags().StringVarP(
		&outputDirectory,
		"output-dir",
		"o",
		".",
		"pasta onde os arquivos serão salvos",
	)
	rootCommand.AddCommand(convertCommand)
}

func main() {
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
