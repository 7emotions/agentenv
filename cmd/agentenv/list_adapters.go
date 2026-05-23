package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/7emotions/agentenv/pkg/adapter"
	"github.com/spf13/cobra"
)

type adapterEntry struct {
	Name    string `json:"name"`
	Display string `json:"display"`
}

var listAdaptersCmd = &cobra.Command{
	Use:   "list-adapters",
	Short: "List available agent adapters",
	Long: `List all registered agent adapters with their display names.

Use --json for machine-readable output.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names := adapter.List()
		sort.Strings(names)

		entries := make([]adapterEntry, 0, len(names))
		for _, name := range names {
			a, err := adapter.Get(name)
			if err != nil {
				continue
			}
			entries = append(entries, adapterEntry{
				Name:    name,
				Display: a.DisplayName(),
			})
		}

		if jsonOutput {
			json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
			return nil
		}

		out := cmd.OutOrStdout()
		for _, e := range entries {
			fmt.Fprintf(out, "%-12s  │  %s\n", e.Name, e.Display)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listAdaptersCmd)
}
