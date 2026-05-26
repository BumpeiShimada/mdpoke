package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BumpeiShimada/mdpoke/internal/app"
	md "github.com/BumpeiShimada/mdpoke/internal/markdown"
)

const usage = `Usage:
  mdpoke <markdown-file>

mdpoke is a terminal Markdown viewer.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("mdpoke", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usage)
	}

	if err := flags.Parse(args); err != nil {
		return 2
	}

	if flags.NArg() == 0 {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "mdpoke: expected exactly one markdown file")
		fmt.Fprint(stderr, usage)
		return 2
	}

	doc, err := md.Load(flags.Arg(0), app.DefaultRenderWidth)
	if err != nil {
		fmt.Fprintf(stderr, "mdpoke: %v\n", err)
		if errors.Is(err, md.ErrInvalidInput) {
			return 2
		}
		return 1
	}

	program := tea.NewProgram(
		app.New(doc),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(stderr, "mdpoke: %v\n", err)
		return 1
	}

	return 0
}
