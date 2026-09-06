package ledger

import (
	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// log is the append-only store. It is unexported and its slices are never
// handed out by reference: Entries returns a copy, so a caller cannot reach in
// and rewrite history through the slice header. There is deliberately no
// Update, no Delete, and no index-assignment anywhere in this file.
type log struct {
	entries  []entity.LedgerEntry
	accruals []entity.Accrual
	errors   []entity.LedgerError
	seq      int
}

func newLog() *log { return &log{} }

// append adds an entry and stamps it with the next sequence number. The
// returned entry is the stamped copy; the caller cannot influence Seq.
func (l *log) append(e entity.LedgerEntry) entity.LedgerEntry {
	l.seq++
	e.Seq = l.seq
	l.entries = append(l.entries, e)
	return e
}

func (l *log) appendAccrual(a entity.Accrual) entity.Accrual {
	l.seq++
	a.Seq = l.seq
	l.accruals = append(l.accruals, a)
	return a
}

func (l *log) appendError(e entity.LedgerError) { l.errors = append(l.errors, e) }

// Entries returns a copy of the log. Callers get the history, not a handle on
// it.
func (l *log) Entries() []entity.LedgerEntry {
	out := make([]entity.LedgerEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

func (l *log) Accruals() []entity.Accrual {
	out := make([]entity.Accrual, len(l.accruals))
	copy(out, l.accruals)
	return out
}

func (l *log) Errors() []entity.LedgerError {
	out := make([]entity.LedgerError, len(l.errors))
	copy(out, l.errors)
	return out
}
