package ledger

import (
	"context"
	"io"
)

// NewRoutes wires the module's controllers to its delivery mechanism.
//
// In the user module this takes a *gin.RouterGroup and registers handlers
// against HTTP verbs and paths. The brief forbids a web layer, so the delivery
// mechanism here is a writer and the "routes" are the ordered steps of the
// replay. The role of the file is unchanged: it is the only place that knows
// which handlers exist and in what order they run, so main stays a composition
// root and the controllers stay unaware of each other.
func NewRoutes(ctx context.Context, w io.Writer, service Service) error {
	for _, handle := range []Handler{
		Replay(service),
		Report(service),
	} {
		if err := handle(ctx, w); err != nil {
			return err
		}
	}
	return nil
}
