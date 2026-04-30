package plugin

import (
	"testing"

	"github.com/hashicorp/go-plugin"
)

func TestGoPluginImports(t *testing.T) {
	// Minimal validation that go-plugin imports cleanly with Go 1.25
	var _ plugin.HandshakeConfig
	var _ plugin.PluginSet
}
