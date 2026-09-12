// Command objv is the Vertex Objective-C compiler.
package main

import (
	"os"

	"github.com/vertex-language/objv/internal/cli"
)

func main() { os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr)) }
