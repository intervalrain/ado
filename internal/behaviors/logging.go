package behaviors

import (
	"context"
	"io"
	"time"

	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/logging"
)

// LoggingBehavior is a pipeline behavior that records every mediator request
// to the shared ado log file (~/.ado/logs/ado-YYYY-MM-DD.log). It does not
// write to the stdout stream used for command results.
type LoggingBehavior struct{}

func (l *LoggingBehavior) Handle(ctx context.Context, req cqrs.Request, w io.Writer, next cqrs.NextFunc) error {
	start := time.Now()
	name := req.RequestName()
	log := logging.L()
	log.InfoContext(ctx, "mediator request start", "request", name)

	err := next(ctx)

	elapsed := time.Since(start)
	if err != nil {
		log.ErrorContext(ctx, "mediator request failed",
			"request", name,
			"elapsed_ms", elapsed.Milliseconds(),
			"error", err.Error(),
		)
	} else {
		log.InfoContext(ctx, "mediator request done",
			"request", name,
			"elapsed_ms", elapsed.Milliseconds(),
		)
	}
	return err
}
