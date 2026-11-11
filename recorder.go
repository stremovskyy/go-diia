package go_diia

import (
	"context"

	"github.com/stremovskyy/go-diia/internal/models/recording"
)

// HTTPRecord is deliberately body-free: recording SDK traffic must never
// persist tokens, metadata, PDFs, callback bodies, or personal data.
type HTTPRecord = recording.HTTPRecord

// Recorder receives sanitized request outcome metadata.
type Recorder interface {
	Record(ctx context.Context, record HTTPRecord) error
}

// RecorderFunc adapts a function to Recorder.
type RecorderFunc func(context.Context, HTTPRecord) error

func (f RecorderFunc) Record(ctx context.Context, record HTTPRecord) error {
	return f(ctx, record)
}
