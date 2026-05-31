package events

// EventsConfig configures the event system behavior.
type EventsConfig struct {
	// Enabled controls whether the event system is active.
	Enabled bool `yaml:"enabled"`

	// StorePath is the path to the JSONL event store file.
	// Default: .nekonomimo/events/run_events.jsonl
	StorePath string `yaml:"store_path"`

	// MaxMessageBytes caps the size of event Message fields.
	MaxMessageBytes int `yaml:"max_message_bytes"`

	// MaxMetadataValueBytes caps the size of each metadata value.
	MaxMetadataValueBytes int `yaml:"max_metadata_value_bytes"`

	// EmitToolEvents controls whether tool.started/finished events are emitted.
	EmitToolEvents bool `yaml:"emit_tool_events"`

	// EmitModelEvents controls whether model call events are emitted.
	EmitModelEvents bool `yaml:"emit_model_events"`

	// EmitPatchEvents controls whether patch.preview.started/finished events are emitted.
	EmitPatchEvents bool `yaml:"emit_patch_events"`

	// EmitValidationEvents controls whether validation.started/finished events are emitted.
	EmitValidationEvents bool `yaml:"emit_validation_events"`
}

// DefaultEventsConfig returns safe defaults for the event system.
func DefaultEventsConfig() EventsConfig {
	return EventsConfig{
		Enabled:               true,
		StorePath:             ".nekonomimo/events/run_events.jsonl",
		MaxMessageBytes:       2048,
		MaxMetadataValueBytes: 512,
		EmitToolEvents:        true,
		EmitModelEvents:       true,
		EmitPatchEvents:       true,
		EmitValidationEvents:  true,
	}
}
