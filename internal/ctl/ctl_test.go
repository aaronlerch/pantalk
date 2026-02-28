package ctl

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pantalk/pantalk/internal/config"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pantalk.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return path
}

func TestRunConfigAddBot_Matrix(t *testing.T) {
	configPath := writeTestConfig(t, `
bots:
  - name: existing
    type: discord
    bot_token: discord-token
`)

	err := runConfigAddBot([]string{
		"--config", configPath,
		"--name", "matrix-bot",
		"--type", "matrix",
		"--endpoint", "https://matrix.example.com",
		"--access-token", "matrix-access-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.Bots) != 2 {
		t.Fatalf("expected 2 bots, got %d", len(cfg.Bots))
	}

	var matrix config.BotConfig
	for _, bot := range cfg.Bots {
		if bot.Name == "matrix-bot" {
			matrix = bot
			break
		}
	}

	if matrix.Type != "matrix" {
		t.Fatalf("expected matrix type, got %q", matrix.Type)
	}
	if matrix.Endpoint != "https://matrix.example.com" {
		t.Fatalf("unexpected matrix endpoint: %q", matrix.Endpoint)
	}
	if matrix.AccessToken != "matrix-access-token" {
		t.Fatalf("unexpected matrix access token: %q", matrix.AccessToken)
	}
}

func TestChooseProvider_MatrixByNumber(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("7\n"))
	provider, err := chooseProvider(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != "matrix" {
		t.Fatalf("expected matrix provider, got %q", provider)
	}
}

