package wake_on_wan_relay_config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetContext_NotNil(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	configJSON := `{
		"macAddress": "AA:BB:CC:DD:EE:FF",
		"internalAddress": "192.168.1.100"
	}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)

	ctx := GetContext()
	if ctx == nil {
		t.Fatal("GetContext() returned nil")
	}
	if ctx.Version == "" {
		t.Errorf("Expected ctx.Version to be populated, got empty string")
	}
}
