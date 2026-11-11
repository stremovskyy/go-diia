package recording

import "time"

type HTTPRecord struct {
	Method     string
	Path       string
	StatusCode int
	Duration   time.Duration
	Error      string
}
