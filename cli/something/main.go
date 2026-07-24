package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"currency-wails/cli/something/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cmd.Execute(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
