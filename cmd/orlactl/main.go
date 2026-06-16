// Command orlactl is the control-plane CLI for the orla daemon. It is a
// thin HTTP client: it registers backends, maps stages, and inspects both,
// without linking the database driver or the server packages.
package main

import (
	"fmt"
	"os"

	"github.com/harvard-cns/orla/internal/orlaclient"
)

func main() {
	if err := orlaclient.NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "orlactl: %v\n", err)
		os.Exit(1)
	}
}
