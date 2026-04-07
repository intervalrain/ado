package cqrs

import (
	"context"
	"fmt"
	"io"
)

type Request interface {
	RequestName() string
}

type RequestHandler interface {
	Handle(ctx context.Context, req Request, w io.Writer) error
}

type PipelineBehavior interface {
	Handle(ctx context.Context, req Request, w io.Writer, next NextFunc) error
}

type NextFunc func(ctx context.Context) error

type Mediator struct {
	handlers  map[string]RequestHandler
	behaviors []PipelineBehavior
}

func NewMediator() *Mediator {
	return &Mediator{handlers: make(map[string]RequestHandler)}
}

func (m *Mediator) Register(name string, h RequestHandler) {
	m.handlers[name] = h
}

func (m *Mediator) Use(b PipelineBehavior) {
	m.behaviors = append(m.behaviors, b)
}

func (m *Mediator) Send(ctx context.Context, req Request, w io.Writer) error {
	h, ok := m.handlers[req.RequestName()]
	if !ok {
		return fmt.Errorf("no handler registered for %q", req.RequestName())
	}

	final := func(ctx context.Context) error {
		return h.Handle(ctx, req, w)
	}

	chain := final
	for i := len(m.behaviors) - 1; i >= 0; i-- {
		b := m.behaviors[i]
		next := chain
		chain = func(ctx context.Context) error {
			return b.Handle(ctx, req, w, next)
		}
	}

	return chain(ctx)
}
