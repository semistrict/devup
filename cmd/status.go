package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/semistrict/devup/internal/proxy"
)

func runStatus() error {
	client := proxy.NewClient()
	routes, err := client.ListRoutes()
	if err != nil {
		return fmt.Errorf("failed to query proxy (is caddy running?): %w", err)
	}

	if len(routes) == 0 {
		fmt.Println("No running dev servers.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tHOSTNAME\tPORT")
	for i, r := range routes {
		fmt.Fprintf(w, "%d\t%s\t%d\n", i+1, r.Hostname, r.Port)
	}
	return w.Flush()
}
