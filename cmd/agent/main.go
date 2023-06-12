// Package main implements the root level command for the disruptor agent CLI
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

func main() {
	env := runtime.DefaultEnvironment()
	ctx := context.Background()
	if err := commands.BuildRootCmd(env).Do(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
