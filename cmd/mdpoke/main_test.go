package main

import (
	"bytes"
	"strings"
	"testing"
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
