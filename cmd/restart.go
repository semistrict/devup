package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/semistrict/devup/internal/proxy"
)

var restartCmd = &cobra.Command{
	Use:   "restart <hostname|index>",
	Short: "Remove a route from the proxy (the devup TUI will detect the disconnect)",
	Args:  cobra.ExactArgs(1),
	RunE:  runRestart,
}

func runRestart(cmd *cobra.Command, args []string) error {
	client := proxy.NewClient()
	routes, err := client.ListRoutes()
	if err != nil {
		return fmt.Errorf("failed to query proxy: %w", err)
	}

	query := args[0]
	var target *proxy.Route

	// Try as 1-based index
	if idx, err := strconv.Atoi(query); err == nil && idx >= 1 && idx <= len(routes) {
		target = &routes[idx-1]
	}

	// Try as hostname
	if target == nil {
		for i, r := range routes {
			if r.Hostname == query || r.ID == query {
				target = &routes[i]
				break
			}
		}
	}

	if target == nil {
		return fmt.Errorf("no route matching %q found", query)
	}

	fmt.Printf("Removing route %s (port %d)...\n", target.Hostname, target.Port)
	if err := client.Deregister(target.Hostname); err != nil {
		return fmt.Errorf("deregister route: %w", err)
	}
	fmt.Printf("Removed. The devup instance will need to re-register when restarted.\n")
	return nil
}
