package formatting

import "context"

// FormattingProvider defines the interface for document formatting providers
type FormattingProvider interface {
	// Id returns the unique identifier of the formatting provider
	Id() string

	// Name returns the human-readable name of the formatting provider
	Name() string

	// Format applies formatting to the given file content and returns the formatted content
	Format(ctx context.Context, filePath string, content string) (string, error)
}
