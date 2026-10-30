package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func installCodex(ctx context.Context) error {
	directory := os.Getenv("CODEX_HOME")
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		directory = filepath.Join(home, ".codex")
	}
	path := filepath.Join(directory, "config.toml")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	original, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		backup, err := os.OpenFile(path+".backup-"+time.Now().Format("20060102-150405.000000000"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := backup.Write(original)
		if err = errors.Join(writeErr, backup.Close()); err != nil {
			return err
		}
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "codex", "mcp", "add", "gigacode-bridge-mcp", "--", binary)
	if err = command.Run(); err != nil {
		return fmt.Errorf("Codex registration failed: %w", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, err := codexTimeout(string(content))
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".gigacode-config-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	_, writeErr := temporary.WriteString(updated)
	if err = errors.Join(writeErr, temporary.Close()); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	fmt.Println("Codex configured. Restart active sessions to load the MCP server.")
	return nil
}

func codexTimeout(content string) (string, error) {
	header := "[mcp_servers.gigacode-bridge-mcp]"
	lines := strings.SplitAfter(content, "\n")
	found := false
	inside := false
	var result strings.Builder
	timeout := regexp.MustCompile(`^tool_timeout_sec\s*=`)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == header {
			if found {
				return "", errors.New("duplicate MCP configuration section")
			}
			found = true
			inside = true
			result.WriteString(header + "\ntool_timeout_sec = 180\n")
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inside = false
		}
		if inside && timeout.MatchString(trimmed) {
			continue
		}
		result.WriteString(line)
	}
	if !found {
		return "", errors.New("Codex did not write the expected MCP section; configure tool_timeout_sec manually")
	}
	return result.String(), nil
}
