package cluster

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.Name != "" {
		t.Errorf("Name want \"\", got %q", cfg.Name)
	}
	if cfg.Discovery == nil || cfg.Discovery.Type != "consul" {
		t.Errorf("Discovery want type consul, got %v", cfg.Discovery)
	}
	if cfg.MessageQueue == nil || cfg.MessageQueue.Type != "nats" {
		t.Errorf("MessageQueue want type nats, got %v", cfg.MessageQueue)
	}
}
