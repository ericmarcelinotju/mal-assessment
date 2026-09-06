package ledger

import (
	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Account IDs from the brief.
const (
	ACC001 = "ACC-001"
	ACC002 = "ACC-002"
)

// CanonicalAccounts are the two accounts of the brief, both opening at zero.
func CanonicalAccounts() []entity.Account {
	return []entity.Account{
		entity.NewAccount(ACC001, entity.AED, "0.00"),
		entity.NewAccount(ACC002, entity.BHD, "0.000"),
	}
}

// CanonicalStream is the event stream from the brief, transcribed verbatim and
// in the order given.
//
// Note E9 (posting day 6) is listed ahead of E10 (posting day 5). That is the
// brief's ordering, kept here exactly as stated; Replay sorts by posting day
// before processing, because a ledger cannot learn of a Day 6 event before a
// Day 5 one. Keeping the transcription faithful and doing the reordering in the
// engine means the discrepancy stays visible instead of being quietly tidied
// away in the data.
func CanonicalStream() []entity.Event {
	aed := func(s string) entity.Money { return entity.MustParseMoney(s, entity.AED) }
	bhd := func(s string) entity.Money { return entity.MustParseMoney(s, entity.BHD) }

	return []entity.Event{
		{
			ID: "E1", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1,
			AccountID: ACC001, Amount: aed("1200.00"),
		},
		{
			ID: "E2", Type: entity.EventDebit, PostingDay: 1, ValueDate: 1,
			AccountID: ACC001, Amount: aed("950.00"),
		},
		{
			ID: "E3", Type: entity.EventAuthorization, PostingDay: 2, ValueDate: 2,
			AccountID: ACC001, Amount: aed("200.00"), AuthID: "Auth-A",
		},
		{
			ID: "E4", Type: entity.EventCredit, PostingDay: 3, ValueDate: 3,
			AccountID: ACC001, Amount: aed("400.00"),
		},
		{
			ID: "E5", Type: entity.EventSettlement, PostingDay: 4, ValueDate: 4,
			AccountID: ACC001, Amount: aed("185.00"), AuthID: "Auth-A",
		},
		{
			// Auth-Z has no preceding authorization event.
			ID: "E6", Type: entity.EventSettlement, PostingDay: 4, ValueDate: 4,
			AccountID: ACC001, Amount: aed("180.00"), AuthID: "Auth-Z",
		},
		{
			// Back-valued: posted on day 5, applies from day 2.
			ID: "E7", Type: entity.EventDebit, PostingDay: 5, ValueDate: 2,
			AccountID: ACC001, Amount: aed("620.00"),
		},
		{
			ID: "E8", Type: entity.EventAuthorization, PostingDay: 5, ValueDate: 5,
			AccountID: ACC001, Amount: aed("90.00"), AuthID: "Auth-B",
		},
		{
			ID: "E9", Type: entity.EventReversal, PostingDay: 6, ValueDate: 2,
			AccountID: ACC001, ReversesEventID: "E7",
		},
		{
			ID: "E10", Type: entity.EventCredit, PostingDay: 5, ValueDate: 5,
			AccountID: ACC002, Amount: bhd("10.000"), Instalments: 3,
		},
	}
}
