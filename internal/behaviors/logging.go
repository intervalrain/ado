package behaviors

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/rainhu/ado/internal/cqrs"
)

type LoggingBehavior struct{}

func (l *LoggingBehavior) Handle(ctx context.Context, req cqrs.Request, w io.Writer, next cqrs.NextFunc) error {
	start := time.Now()
	fmt.Fprintf(w, "=> %s\n", req.RequestName())

	err := next(ctx)

	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(w, "=> %s failed (%s): %v\n", req.RequestName(), elapsed, err)
	} else {
		fmt.Fprintf(w, "=> %s done (%s)\n", req.RequestName(), elapsed)
	}
	return err
}
