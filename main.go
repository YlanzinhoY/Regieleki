package main

import (
	"fmt"
	"os"
)

func main() {
	if relaunchInCompactTerminal() {
		return
	}
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
