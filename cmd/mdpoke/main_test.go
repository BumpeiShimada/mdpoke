package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	md "github.com/BumpeiShimada/mdpoke/internal/markdown"
)

func TestRunUsageMentionsSafetyFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"--no-watch", "--max-size", "--follow-symlinks"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("usage missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestReadmeMentionsIssueBackedFeatures(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	for _, want := range []string{
		"heading outline",
		"Drag to select text and release to copy",
		"Toggle checkboxes",
		"--no-watch",
		"--max-size",
		"--follow-symlinks",
		"strips terminal control characters",
		fmt.Sprintf("%d MiB", md.DefaultMaxMarkdownSize/(1024*1024)),
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README missing %q", want)
		}
	}
	if strings.Index(readme, "## Safety And Limits") < strings.Index(readme, "## Keys") {
		t.Fatalf("README safety section should stay after primary usage sections")
	}
}

func TestReadmeRunExamplesUseSupportedFlags(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("usage exit code = %d, want 0", code)
	}
	usage := stdout.String()

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "mdpoke ") {
			continue
		}
		for _, field := range strings.Fields(line) {
			if !strings.HasPrefix(field, "--") {
				continue
			}
			flagName := strings.SplitN(field, "=", 2)[0]
			if !strings.Contains(usage, flagName) {
				t.Fatalf("README example uses unsupported flag %q in line %q", flagName, line)
			}
		}
	}
}

func TestReadmeInstallAndImagesAreWellFormed(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	if !strings.Contains(readme, "brew install BumpeiShimada/tap/mdpoke") {
		t.Fatal("README missing Homebrew install command")
	}

	imageCount := 0
	for _, line := range strings.Split(readme, "\n") {
		if !strings.Contains(line, "<img ") {
			continue
		}
		imageCount++
		if !strings.Contains(line, "src=\"https://github.com/user-attachments/assets/") {
			t.Fatalf("README image should use a GitHub attachment URL: %s", line)
		}
		if !strings.Contains(line, "alt=\"") {
			t.Fatalf("README image should include alt text: %s", line)
		}
	}
	if imageCount == 0 {
		t.Fatal("README should include screenshots")
	}
}

func TestRunHelpExitsSuccessfully(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "-help"} {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run([]string{arg}, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Fatalf("stdout = %q, want usage", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(stdout.String(), "mdpoke ") {
		t.Fatalf("stdout = %q, want version", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRejectsNonPositiveMaxSize(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--max-size=0", "README.md"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--max-size must be greater than zero") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsMultipleMarkdownFiles(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"one.md", "two.md"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "expected exactly one markdown file") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunSanitizesLoadErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"missing\x1b[2J.md"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if strings.Contains(stderr.String(), "\x1b") {
		t.Fatalf("stderr contains ESC: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "[2J") {
		t.Fatalf("stderr = %q, want harmless escaped text payload", stderr.String())
	}
}
