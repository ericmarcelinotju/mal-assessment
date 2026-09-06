package entity

// EntryOrigin records why an entry exists. It is the audit trail that lets a
// reader tell a customer-instructed movement from one the ledger generated
// itself, without re-deriving it from the event stream.
type EntryOrigin string

const (
	OriginInstruction    EntryOrigin = "INSTRUCTION"
	OriginSettlement     EntryOrigin = "SETTLEMENT"
	OriginReversal       EntryOrigin = "REVERSAL"
	OriginOverdraftFee   EntryOrigin = "OVERDRAFT_FEE"
	OriginFeeReversal    EntryOrigin = "FEE_REVERSAL"
	OriginCapitalisation EntryOrigin = "INTEREST_CAPITALISATION"
)

// LedgerEntry is one posting. Entries are append-only: nothing in this package
// or in the ledger module exposes a way to modify or remove one once appended.
// A correction is a new entry with the opposite sign, never an edit.
//
// Signed convention: Amount carries its own sign. A debit of 950.00 is stored
// as -950.00 rather than as a positive number plus a direction flag, so that a
// balance is a plain sum and cannot be got wrong by mishandling the flag.
// Direction is retained for presentation only.
type LedgerEntry struct {
	Seq        int     // append order, unique and monotonic
	EventID    EventID // the event that produced this entry
	AccountID  string
	PostingDay Day   // when the ledger learned of it
	ValueDate  Day   // when it economically applies
	Amount     Money // signed
	Direction  Direction
	Origin     EntryOrigin
	Memo       string

	// ReversesSeq links a contra entry to the entry it reverses, so a fee
	// reversal can be told from an ordinary credit without parsing the memo.
	ReversesSeq int
}

func (e LedgerEntry) IsFee() bool         { return e.Origin == OriginOverdraftFee }
func (e LedgerEntry) IsFeeReversal() bool { return e.Origin == OriginFeeReversal }
