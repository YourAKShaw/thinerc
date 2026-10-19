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
		a.whatsupCmd(),
		a.nmCmd(),
		a.runCmd(),
		a.recipesCmd(),
		a.importCmd(),
		a.helloCmd(),
		a.themeCmd(),
		a.fontsCmd(),
		a.fxCmd(),
		a.initCmd(),
		a.setupCmd(),
		a.trustCmd(),
		a.untrustCmd(),
		a.configCmd(),
		a.doctorCmd(),
		a.benchCmd(),
		a.aiCmd(),
		a.notifyCmd(),
		a.versionCmd(),
	}
}
