package go_diia

import (
	"context"
	"net/http"
)

var (
	_ CallbackParseBuilder        = (*callbackParseBuilder)(nil)
	_ CallbackProcessBuilder      = (*callbackProcessBuilder)(nil)
	_ CallbackWriteSuccessBuilder = (*callbackWriteSuccessBuilder)(nil)
	_ CallbackWriteFailureBuilder = (*callbackWriteFailureBuilder)(nil)
)

type callbackParseBuilder struct {
	service *callbackService
	request *http.Request
}

func (s *callbackService) Parse() CallbackParseBuilder { return &callbackParseBuilder{service: s} }

func (b *callbackParseBuilder) WithRequest(request *http.Request) CallbackParseBuilder {
	b.request = request

	return b
}

func (b *callbackParseBuilder) Do(ctx context.Context) (*CallbackPayload, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.parse(ctx, b.request)
}

type callbackProcessBuilder struct {
	service *callbackService
	payload *CallbackPayload
}

func (s *callbackService) Process() CallbackProcessBuilder {
	return &callbackProcessBuilder{service: s}
}

func (b *callbackProcessBuilder) WithPayload(payload *CallbackPayload) CallbackProcessBuilder {
	b.payload = payload

	return b
}

func (b *callbackProcessBuilder) Do(ctx context.Context) (*DocumentPackage, error) {
	if err := validateExecutionContext(ctx); err != nil {
		return nil, err
	}

	return b.service.process(ctx, b.payload)
}

type callbackWriteSuccessBuilder struct {
	service *callbackService
	writer  http.ResponseWriter
}

func (s *callbackService) WriteSuccess() CallbackWriteSuccessBuilder {
	return &callbackWriteSuccessBuilder{service: s}
}

func (b *callbackWriteSuccessBuilder) WithWriter(writer http.ResponseWriter) CallbackWriteSuccessBuilder {
	b.writer = writer

	return b
}

func (b *callbackWriteSuccessBuilder) Do(ctx context.Context) error {
	if err := validateExecutionContext(ctx); err != nil {
		return err
	}

	if b.writer == nil {
		return callbackError("response writer is required", nil)
	}

	return b.service.writeSuccess(b.writer)
}

type callbackWriteFailureBuilder struct {
	service *callbackService
	writer  http.ResponseWriter
	err     error
}

func (s *callbackService) WriteFailure() CallbackWriteFailureBuilder {
	return &callbackWriteFailureBuilder{service: s}
}

func (b *callbackWriteFailureBuilder) WithWriter(writer http.ResponseWriter) CallbackWriteFailureBuilder {
	b.writer = writer

	return b
}

func (b *callbackWriteFailureBuilder) WithError(err error) CallbackWriteFailureBuilder {
	b.err = err

	return b
}

func (b *callbackWriteFailureBuilder) Do(ctx context.Context) error {
	if err := validateExecutionContext(ctx); err != nil {
		return err
	}

	if b.writer == nil {
		return callbackError("response writer is required", nil)
	}

	return b.service.writeFailure(b.writer, b.err)
}
