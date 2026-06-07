package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BumpeiShimada/mdpoke/internal/app"
	md "github.com/BumpeiShimada/mdpoke/internal/markdown"
)

const usage = `Usage:
  mdpoke [--help] [--version] [--no-watch] [--max-size bytes] [--follow-symlinks] <markdown-file>

mdpoke is a terminal Markdown viewer.
`

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if hasHelpArg(args) {
		fmt.Fprint(stdout, usage)
		return 0
	}

	flags := flag.NewFlagSet("mdpoke", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usage)
	}
	showVersion := flags.Bool("version", false, "print version")
	noWatch := flags.Bool("no-watch", false, "disable automatic reload")
	maxSize := flags.Int64("max-size", md.DefaultMaxMarkdownSize, "maximum markdown file size in bytes")
	followSymlinks := flags.Bool("follow-symlinks", false, "allow opening symlinked markdown files")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *showVersion {
		fmt.Fprintf(stdout, "mdpoke %s\n", versionString())
		return 0
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
	if *maxSize <= 0 {
		fmt.Fprintln(stderr, "mdpoke: --max-size must be greater than zero")
		return 2
	}

	loadOptions := md.LoadOptions{
		Width:          app.DefaultRenderWidth,
		MaxSize:        *maxSize,
		FollowSymlinks: *followSymlinks,
	}
	doc, err := md.LoadWithOptions(flags.Arg(0), loadOptions)
	if err != nil {
		fmt.Fprintf(stderr, "mdpoke: %s\n", md.SanitizeMarkdownInput(err.Error()))
		if errors.Is(err, md.ErrInvalidInput) {
			return 2
		}
		return 1
	}

	program := tea.NewProgram(
		app.NewWithOptions(doc, app.Options{
			NoWatch:     *noWatch,
			LoadOptions: loadOptions,
		}),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(stderr, "mdpoke: %s\n", md.SanitizeMarkdownInput(err.Error()))
		return 1
	}

	return 0
}

func hasHelpArg(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "-help" {
			return true
		}
	}
	return false
}

func versionString() string {
	if version != "" && version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}
