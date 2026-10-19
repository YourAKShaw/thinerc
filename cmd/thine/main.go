// Command thine is the safe, animated verbs layer for your terminal.
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/YourAKShaw/thinerc/internal/cli"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

func main() {
	// Animations hide the cursor; always give it back, even on Ctrl-C.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		ui.RestoreTerminal()
		os.Stderr.WriteString("\r\x1b[2K")
		os.Exit(130)
	}()
	code := cli.Execute(os.Args[1:])
	ui.RestoreTerminal()
	os.Exit(code)
}
