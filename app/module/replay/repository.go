package replay

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository holds the event stream to be replayed.
//
// Separating it from the service is what lets the six-day brief scenario and a
// test fixture enter the same way: the service asks for events, it does not
// know they were hard-coded. Like every repository here it is append-only and
// read-only after loading -- the stream is history, and history is not edited.
type Repository interface {
	Load(context.Context, []entity.Event) error
	Events(context.Context) ([]entity.Event, error)
}

type repository struct {
	events []entity.Event
}

func NewRepository() Repository {
	return &repository{}
}

func (s *repository) Load(_ context.Context, events []entity.Event) error {
	s.events = append(s.events, events...)
	return nil
}

func (s *repository) Events(_ context.Context) ([]entity.Event, error) {
	out := make([]entity.Event, len(s.events))
	copy(out, s.events)
	return out, nil
}
