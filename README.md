# Regieleki

**Blazing-fast PixelDrain downloader with a 6 GB limit bypass**

Regieleki is a terminal-based downloader written in Go, using Cobra, Bubble Tea, and Lip Gloss. It takes a file ID, builds the configured endpoint, and downloads the file to your computer using streaming.

## Requirements

* Go 1.26 or later
* Internet connection

## Running in Development

From the project root:

```powershell
go run .
```

Enter the file ID in the TUI and press `Enter`. The download will start automatically.

To choose the destination directory:

```powershell
go run . --output-dir downloads
```

You can also use the short option:

```powershell
go run . -o downloads
```

## Building the Binary

The Windows script generates `bin/regieleki.exe` using flags to reduce the binary size:

```powershell
.\build.ps1
```

The build uses:

* `-trimpath` to remove local file paths from the binary;
* `-buildvcs=false` to avoid including Git metadata;
* `-ldflags="-s -w"` to remove symbols and debug information.

The equivalent command on any system with Go installed is:

```bash
go build -trimpath -buildvcs=false -ldflags="-s -w" -o bin/regieleki .
```

Then run:

```powershell
.\bin\regieleki.exe
```

The generated executable is ignored by Git; only `bin/.gitkeep` is used to keep the directory in the repository.

## Commands

Open the TUI:

```powershell
regieleki
```

Generate the download URL without opening the TUI:

```powershell
regieleki convert e75isJ7y
regieleki convert https://pixeldrain.com/u/e75isJ7y
```

The `convert` command prints the URL. Automatic downloading happens through the interactive TUI flow.

## TUI Controls

* Type: enter the file ID;
* `Enter`: start the download;
* `Enter` after an error: try again;
* `Enter` after completion: start another download;
* `Esc` or `Ctrl+C`: exit.

During the download, the interface displays the progress, amount of data received, percentage when the server provides the total file size, current download speed, and destination path.

## File Behavior

* The filename is obtained from the `Content-Disposition` header when available;
* If no filename is provided, the file is named `file_<id>`;
* Existing files are not overwritten: the program creates names such as `file (1).zip`;
* Incomplete downloads are removed if an error occurs.

## Tests

```bash
go test ./...
```

The endpoint used by the project is configured in `workerBaseURL`, located in the `main.go` file.
