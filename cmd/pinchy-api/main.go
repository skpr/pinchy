package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/skpr/pinchy/cmd/pinchy-api/listen"
)

var cmd = &cobra.Command{
	Use:   "pinchy-api",
	Short: "API server for the Pinchy platform.",
}

func main() {
	cmd.AddCommand(listen.NewCommand())

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
