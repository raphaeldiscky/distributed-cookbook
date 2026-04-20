package telemetry

import "time"

// Config holds telemetry provider settings.
type Config struct {
	ServiceName    string
	OTLPEndpoint   string        // host:port (HTTP), e.g. "localhost:4318"
	SampleRatio    float64       // 0.0 – 1.0
	ExportTimeout  time.Duration // per batch
	ShutdownWait   time.Duration // on graceful shutdown
	Insecure       bool          // send OTLP over plaintext (dev only)
	TracingEnabled bool
}

// DefaultConfig returns a dev-friendly config for the given service name.
func DefaultConfig(serviceName, otlpEndpoint string) Config {
	return Config{
		ServiceName:    serviceName,
		OTLPEndpoint:   otlpEndpoint,
		SampleRatio:    1.0,
		ExportTimeout:  5 * time.Second,
		ShutdownWait:   5 * time.Second,
		Insecure:       true,
		TracingEnabled: true,
	}
}
