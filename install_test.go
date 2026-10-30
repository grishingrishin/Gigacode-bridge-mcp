package main

import (
	"strings"
	"testing"
)

func TestCodexTimeoutPreservesOtherSettings(t *testing.T) {
	original := "model = \"example\"\n# custom setting\n[mcp_servers.other]\ntool_timeout_sec = 12\n[mcp_servers.gigacode-bridge-mcp]\ncommand = \"/path with spaces/server\"\ntool_timeout_sec = 30\n[mcp_servers.gigacode-bridge-mcp.env]\nKEY = \"value\"\n"
	got, err := codexTimeout(original)
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.Replace(original, "command = \"/path with spaces/server\"\ntool_timeout_sec = 30", "tool_timeout_sec = 180\ncommand = \"/path with spaces/server\"", 1)
	if got != expected {
		t.Fatalf("unexpected configuration: %s", got)
	}
	second, err := codexTimeout(got)
	if err != nil || second != got {
		t.Fatal("configuration is not idempotent")
	}
}

func TestCodexTimeoutRejectsMissingSection(t *testing.T) {
	if _, err := codexTimeout("model = \"example\"\n"); err == nil {
		t.Fatal("missing section accepted")
	}
}
