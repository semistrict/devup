package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/semistrict/devup/internal/proxy"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List running dev servers",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
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
