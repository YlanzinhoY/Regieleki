//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	consoleColumns int16 = 96
	consoleRows    int16 = 30
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	setConsoleBufferSizeProc = kernel32.NewProc("SetConsoleScreenBufferSize")
	setConsoleWindowInfoProc = kernel32.NewProc("SetConsoleWindowInfo")
)

func configureConsoleWindow() {
	consoleHandle, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || consoleHandle == windows.InvalidHandle {
		return
	}

	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(consoleHandle, &info); err != nil {
		return
	}

	columns := consoleColumns
	rows := consoleRows
	if info.MaximumWindowSize.X > 0 && columns > info.MaximumWindowSize.X {
		columns = info.MaximumWindowSize.X
	}
	if info.MaximumWindowSize.Y > 0 && rows > info.MaximumWindowSize.Y {
		rows = info.MaximumWindowSize.Y
	}

	targetWindow := windows.SmallRect{
		Left:   0,
		Top:    0,
		Right:  columns - 1,
		Bottom: rows - 1,
	}
	currentWindowWidth := info.Window.Right - info.Window.Left + 1
	currentWindowHeight := info.Window.Bottom - info.Window.Top + 1
	if currentWindowWidth > columns || currentWindowHeight > rows {
		_ = setConsoleWindowInfo(consoleHandle, &targetWindow)
	}

	targetBuffer := info.Size
	if targetBuffer.X < columns {
		targetBuffer.X = columns
	}
	if targetBuffer.Y < rows {
		targetBuffer.Y = rows
	}
	_ = setConsoleBufferSize(consoleHandle, &targetBuffer)
	_ = setConsoleWindowInfo(consoleHandle, &targetWindow)
}

func setConsoleBufferSize(consoleHandle windows.Handle, size *windows.Coord) error {
	result, _, err := setConsoleBufferSizeProc.Call(
		uintptr(consoleHandle),
		uintptr(unsafe.Pointer(size)),
	)
	if result == 0 {
		return err
	}
	return nil
}

func setConsoleWindowInfo(consoleHandle windows.Handle, window *windows.SmallRect) error {
	result, _, err := setConsoleWindowInfoProc.Call(
		uintptr(consoleHandle),
		1,
		uintptr(unsafe.Pointer(window)),
	)
	if result == 0 {
		return err
	}
	return nil
}
