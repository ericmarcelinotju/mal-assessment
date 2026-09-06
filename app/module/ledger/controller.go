package ledger

import (
	"context"
	"io"

	"github.com/ericmarcelinotju/mal-assessment/app/presenter"
)

// Handler is what a controller returns: something the route layer can invoke.
//
// In the user module a controller returns a gin.HandlerFunc, because HTTP is
// the delivery mechanism there. The brief forbids a web layer, so the delivery
// mechanism here is a writer and this is the equivalent shape. The pattern is
// otherwise identical: a free function that takes the Service and closes over
// it, never a method on a struct, and never any business logic of its own.
type Handler func(ctx context.Context, w io.Writer) error

// Replay drives the six-day replay of the canonical stream.
//
// Compare user.Create: bind the input, call the service, render the result.
// Here the input is a fixed stream rather than a request body, so there is
// nothing to bind -- but the shape holds, and the controller decides nothing.
func Replay(svc Service) Handler {
	return func(ctx context.Context, _ io.Writer) error {
		return svc.Replay(ctx, CanonicalStream())
	}
}

// Report renders the per-day position: closing ledger balance, fee assessments,
// authorization states and errors, followed by the append-only log.
func Report(svc Service) Handler {
	return func(ctx context.Context, w io.Writer) error {
		rows, err := svc.Report(ctx)
		if err != nil {
			return err
		}
		entries, err := svc.Entries(ctx)
		if err != nil {
			return err
		}
		presenter.Render(w, svc.Config(), rows, entries)
		return nil
	}
}
