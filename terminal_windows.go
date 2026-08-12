//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const compactTerminalEnvironment = "REGIELEKI_COMPACT_TERMINAL"

func relaunchInCompactTerminal() bool {
	if os.Getenv("WT_SESSION") == "" || os.Getenv(compactTerminalEnvironment) == "1" || !isInteractiveInvocation() {
		return false
	}

	executablePath, err := os.Executable()
	if err != nil {
		return false
	}
	terminalPath, err := exec.LookPath("wt.exe")
	if err != nil {
		return false
	}

	arguments := []string{
		"-w", "new",
		"--size", fmt.Sprintf("%d,%d", consoleColumns, consoleRows),
		"new-tab",
		"--title", "Regieleki",
		"--suppressApplicationTitle",
		executablePath,
	}
	arguments = append(arguments, os.Args[1:]...)

	command := exec.Command(terminalPath, arguments...)
	command.Env = append(os.Environ(), compactTerminalEnvironment+"=1")
	if err := command.Start(); err != nil {
		return false
	}
	return true
}

func isInteractiveInvocation() bool {
	arguments := os.Args[1:]
	if len(arguments) == 0 {
		return true
	}

	if arguments[0] == "convert" {
		for index := 1; index < len(arguments); index++ {
			argument := arguments[index]
			switch argument {
			case "--help", "-h", "--version", "-v":
				return false
			case "--output-dir", "-o":
				index++
			default:
				if !strings.HasPrefix(argument, "-") {
					return false
				}
			}
		}
		return true
	}

	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch argument {
		case "--help", "-h", "--version", "-v":
			return false
		case "--output-dir", "-o":
			index++
		default:
			if !strings.HasPrefix(argument, "-") {
				return false
			}
		}
	}
	return true
}
