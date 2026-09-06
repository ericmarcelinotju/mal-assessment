package entity

// Account is an in-memory account. The opening balance is modelled as an
// explicit amount rather than an implicit zero so that a non-zero opening
// balance needs no special case; both accounts here happen to open at zero.
type Account struct {
	ID       string
	Currency Currency
	Opening  Money
}

func NewAccount(id string, ccy Currency, opening string) Account {
	return Account{ID: id, Currency: ccy, Opening: MustParseMoney(opening, ccy)}
}
