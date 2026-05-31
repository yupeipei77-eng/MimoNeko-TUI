package prefix

import "context"

type SourceKind string

const (
	SourceKindStaticFile      SourceKind = "static_file"
	SourceKindGeneratedSchema SourceKind = "generated_schema"
)

type Source struct {
	Name     string
	Kind     SourceKind
	Path     string
	Required bool
	SHA256   string
}

type ToolSchema struct {
	Name  string
	Bytes []byte
}

type BuildRequest struct {
	Version      int
	SystemPrompt []byte
	CodingRules  []byte
	ToolSchemas  []ToolSchema
	Sources      []Source
}

type Document struct {
	Version       int
	Bytes         []byte
	SHA256        string
	Sources       []Source
	TokenEstimate int
}

type Fingerprint struct {
	SHA256  string
	Version int
}

type PrefixBuilder interface {
	Build(ctx context.Context, req BuildRequest) (Document, error)
	Fingerprint(ctx context.Context, req BuildRequest) (Fingerprint, error)
}
