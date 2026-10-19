// Command thine is the safe, animated verbs layer for your terminal.
package main

import (
	"fmt"
	"os"

	"github.com/YourAKShaw/thinerc/internal/buildinfo"
)

func main() {
	fmt.Fprintf(os.Stdout, "thine %s\n", buildinfo.Get().Version)
}
