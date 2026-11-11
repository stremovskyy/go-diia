package go_diia

// RunOption controls a single mutating API request.
type RunOption func(*runOptions)

// DryRunHandler receives the endpoint and typed payload for a skipped request.
// The SDK does not invoke a default payload logger because these payloads may
// contain identifiers. Applications must explicitly opt in to inspecting it.
type DryRunHandler func(endpoint string, payload any)

type runOptions struct {
	dryRun  bool
	handler DryRunHandler
}

// DryRun validates and builds the request without sending it.
func DryRun(handler ...DryRunHandler) RunOption {
	return func(options *runOptions) {
		options.dryRun = true

		if len(handler) != 0 {
			options.handler = handler[0]
		}
	}
}

func collectRunOptions(options []RunOption) runOptions {
	var result runOptions

	for _, option := range options {
		if option != nil {
			option(&result)
		}
	}

	return result
}

func (o runOptions) handle(endpoint string, payload any) bool {
	if !o.dryRun {
		return false
	}

	if o.handler != nil {
		o.handler(endpoint, payload)
	}

	return true
}
