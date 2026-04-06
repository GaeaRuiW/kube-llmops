package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/kube-llmops/operator/internal/cli/cmd"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func main() {
	root := cmd.NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var cliErr *util.CLIError
		if errors.As(err, &cliErr) {
			os.Exit(cliErr.Code)
		}
		os.Exit(1)
	}
}
