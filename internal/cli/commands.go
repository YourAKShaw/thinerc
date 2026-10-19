package cli

import "github.com/spf13/cobra"

// commands lists every top-level verb in help order.
func (a *App) commands() []*cobra.Command {
	return []*cobra.Command{
		a.portCmd(),
		a.portsCmd(),
		a.killCmd(),
		a.freeupCmd(),
		a.cleanupCmd(),
		a.applyCmd(),
		a.undoCmd(),
		a.historyCmd(),
		a.trashCmd(),
		a.helloCmd(),
		a.initCmd(),
		a.setupCmd(),
		a.versionCmd(),
	}
}
