package policy

import "testing"

func TestTimeoutHierarchy(t *testing.T) {
	if !(ProviderRequestTimeout < APIRequestTimeout &&
		APIRequestTimeout < APIWriteTimeout &&
		APIWriteTimeout < CLIRequestTimeout &&
		CLIRequestTimeout < MCPRequestTimeout &&
		MCPRequestTimeout < MCPWriteTimeout) {
		t.Fatal("timeout hierarchy must increase from provider through MCP write")
	}
}
