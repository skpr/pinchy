package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/skpr/pinchy/cmd/pinchy-board/listen"
)

var cmd = &cobra.Command{
	Use:   "pinchy-board",
	Short: "Kanban board of opencode sessions across all projects.",
}

func main() {
	cmd.AddCommand(listen.NewCommand())

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
