// Package config holds every tunable constant of the ledger core in one place.
// It is deliberately 1:1 with NUMBERS.md: if a number appears in the engine and
// not here, that is a bug.
package config

import "os"

// FeeReversalPolicy decides what happens to an overdraft fee once the entry
// that pushed the day negative is itself reversed.
type FeeReversalPolicy string

const (
	// FeeReversalNone is the literal reading of the brief. The brief grants an
	// assessment primitive and no de-assessment primitive, so a fee, once
	// booked, stands forever. Acceptance criterion 6 is FALSE under this policy.
	FeeReversalNone FeeReversalPolicy = "none"

	// FeeReversalOnCauseReversal appends a contra entry for any fee whose day is
	// no longer negative once that fee is excluded. Append-only is preserved:
	// the original fee entry is never touched. Criterion 6 is TRUE under this
	// policy. See REJECTED.md, criterion 6.
	FeeReversalOnCauseReversal FeeReversalPolicy = "on_cause_reversal"
)

const EnvFeeReversalPolicy = "LEDGER_FEE_REVERSAL_POLICY"

// Config is passed to the engine explicitly rather than read from the
// environment deep inside it, so tests select a policy without mutating
// process state.
type Config struct {
	// OverdraftFeeMajor is the overdraft fee in major units of the account's own
	// currency: 25 means AED 25.00 and BHD 25.000. Assessed at most once per
	// account per day. Held in major units so no scale conversion is needed at
	// the use site -- a divisor inlined there would be a constant living outside
	// this file. See NUMBERS.md for why the fee is bound to the account currency
	// rather than denominated in AED for every account.
	OverdraftFeeMajor int64

	// Daily interest is InterestRateNum/InterestRateDen per day = 0.04%.
	// Kept as an exact rational; the engine never sees a floating point number.
	InterestRateNum int64
	InterestRateDen int64

	// WindowDays is the inclusive replay window, Day 1..Day 6.
	WindowDays int

	// CapitalisationDay is the day the accrued interest is booked as a single
	// credit. Must be <= WindowDays.
	CapitalisationDay int

	// FeeReversal selects the criterion-6 branch.
	FeeReversal FeeReversalPolicy

	// MaxSweepIterations bounds the fee assessment/reversal fixed-point loop so
	// that a rule change which introduces oscillation fails loudly instead of
	// hanging.
	MaxSweepIterations int
}

func Default() Config {
	return Config{
		OverdraftFeeMajor:  25,
		InterestRateNum:    4,
		InterestRateDen:    10000,
		WindowDays:         6,
		CapitalisationDay:  6,
		FeeReversal:        FeeReversalNone,
		MaxSweepIterations: 64,
	}
}

// Load returns the default configuration with the fee-reversal policy taken
// from the environment. An unrecognised value falls back to the default rather
// than failing, and the caller reports the active policy in the report header.
func Load() Config {
	cfg := Default()
	if v := os.Getenv(EnvFeeReversalPolicy); v != "" {
		if p := FeeReversalPolicy(v); p == FeeReversalNone || p == FeeReversalOnCauseReversal {
			cfg.FeeReversal = p
		}
	}
	return cfg
}
