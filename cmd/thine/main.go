// Command thine is the safe, animated verbs layer for your terminal.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/YourAKShaw/thinerc/internal/cli"
	"github.com/YourAKShaw/thinerc/internal/interrupt"
	"github.com/YourAKShaw/thinerc/internal/ui"
)

func main() {
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	ctx := interrupt.Watch(context.Background(), sig, func() {
		// Animations hide the cursor; always give it back, even on Ctrl-C.
		ui.RestoreTerminal()
		os.Stderr.WriteString("\r\x1b[2K")
		os.Exit(cli.ExitInterrupted)
	})
	ui.CancelPromptsOn(ctx.Done())
	code := cli.Execute(ctx, os.Args[1:])
	ui.RestoreTerminal()
	os.Exit(code)
}
