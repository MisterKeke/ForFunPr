// Package policy defines the cross-interface timeout hierarchy. Each outer
// layer outlives the work it calls so cancellation is reported by the layer
// that owns it instead of leaving a mutation running after its client exits.
package policy

import "time"

const (
	ProviderRequestTimeout = 25 * time.Second
	APIRequestTimeout      = 55 * time.Second // allows two sequential provider stages
	APIWriteTimeout        = 60 * time.Second
	CLIRequestTimeout      = 65 * time.Second
	MCPRequestTimeout      = 70 * time.Second
	MCPWriteTimeout        = 75 * time.Second
	GracefulShutdownTimeout = 10 * time.Second
)
