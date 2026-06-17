package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/skpr/pinchy/cmd/pinchy-proxy/listen"
)

var cmd = &cobra.Command{
	Use:   "pinchy-proxy",
	Short: "Reverse proxy that routes NAME.pinchy.localhost to Pinchy environments.",
}

func main() {
	cmd.AddCommand(listen.NewCommand())

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
