package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/skpr/pinchy/cmd/pinchy/exec"
)

var cmd = &cobra.Command{
	Use:   "pinchy",
	Short: "CLI for the Pinchy platform.",
}

func main() {
	cmd.AddCommand(exec.NewCommand())

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
