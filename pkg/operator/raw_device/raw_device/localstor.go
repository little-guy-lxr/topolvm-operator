package main

import (
	controller "github.com/alauda/topolvm-operator/pkg/operator/raw_device/controller/cmd"
	node "github.com/alauda/topolvm-operator/pkg/operator/raw_device/node/cmd"
	"io"
	"os"
	"path/filepath"
)

func usage() {
	io.WriteString(os.Stderr, `Usage: localstor COMMAND [ARGS ...]

COMMAND:
    raw-device-controller:  raw-device CSI controller service.
    raw-device-node:        raw-device CSI node service.
`)
}

func main() {
	name := filepath.Base(os.Args[0])
	if name == "localstor" {
		if len(os.Args) == 1 {
			usage()
			os.Exit(1)
		}
		name = os.Args[1]
		os.Args = os.Args[1:]
	}

	switch name {
	case "raw-device-node":
		node.Execute()
	case "raw-device-controller":
		controller.Execute()
	default:
		usage()
		os.Exit(1)
	}
}
