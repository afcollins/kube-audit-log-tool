package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/afcollins/kube-audit-log-tool/internal/tui"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [file1.log file2.log.gz ...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Interactive TUI for exploring Kubernetes audit log events.\n")
		fmt.Fprintf(os.Stderr, "If no files are provided, a file picker will be shown.\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	files := flag.Args()

	if err := tui.Run(files); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
