package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCompletion(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			var output bytes.Buffer
			if err := runCompletion([]string{shell}, &output); err != nil {
				t.Fatal(err)
			}
			if output.Len() == 0 || !strings.Contains(output.String(), "ascdir") || !strings.Contains(output.String(), "allow-local-asset-deletions") || !strings.Contains(output.String(), "submit") || !strings.Contains(output.String(), "distribute") || !strings.Contains(output.String(), "confirm") {
				t.Fatalf("completion output = %q", output.String())
			}
		})
	}
}

func TestRunCompletionRejectsInvalidShell(t *testing.T) {
	if err := runCompletion([]string{"invalid"}, &bytes.Buffer{}); err == nil {
		t.Fatal("invalid shell succeeded")
	}
}

func TestZshCompletionRegistersFunction(t *testing.T) {
	if strings.Contains(zshCompletion, `_ascdir "$@"`) {
		t.Fatal("zsh completion executes the function while sourcing")
	}
	if !strings.Contains(zshCompletion, "compdef _ascdir ascdir") {
		t.Fatal("zsh completion does not register _ascdir for ascdir")
	}
}
