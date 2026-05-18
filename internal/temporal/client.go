package temporal

import (
	"fmt"
	"os"

	"go.temporal.io/sdk/client"
)

const (
	DefaultNamespace = "default"

	// Task queues
	TaskQueueGateway  = "brevio-gateway"
	TaskQueueBrain    = "brevio-brain"
	TaskQueueControl  = "brevio-control"
	TaskQueueExecutor = "brevio-executor"
	TaskQueueCanvas   = "brevio-canvas"
	TaskQueueCore     = "brevio-core"
	TaskQueueAdmin    = "brevio-admin"
)

// NewClient creates a Temporal client from environment configuration.
func NewClient() (client.Client, error) {
	host := os.Getenv("TEMPORAL_HOST")
	if host == "" {
		host = "localhost:7233"
	}
	ns := os.Getenv("TEMPORAL_NAMESPACE")
	if ns == "" {
		ns = DefaultNamespace
	}
	opts := client.Options{
		HostPort:  host,
		Namespace: ns,
	}
	c, err := client.Dial(opts)
	if err != nil {
		return nil, fmt.Errorf("temporal client dial: %w", err)
	}
	return c, nil
}
