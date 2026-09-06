# NUMBERS

Every constant in the engine, why it holds that value, and what would break at
half of it. All of them live in [config/ledger.go](config/ledger.go); if a
number appears in the engine and not there, that is a bug.

Constants fall into two groups. Some are **given** by the brief, and the choice
I actually made was how to interpret them — those are the interesting ones.
Others are **chosen**, and have to be defended outright.

---

## Given by the brief

### `OverdraftFeeMinor = 2500` — the overdraft fee, 25.00

Given. What was chosen is the **denomination**.

The brief says "AED 25.00" while ACC-002 is a BHD account. An AED-denominated
entry cannot be booked to a BHD account without an exchange rate, and the brief
supplies none. Three options:

1. Book AED 25.00 into a BHD account. Rejected: the account would hold two
   currencies, and every balance on it becomes a question about which.
2. Convert at some rate. Rejected: the rate would be invented, and an invented
   rate in a ledger is worse than a missing feature.
3. **Treat the fee as 25 units of the account's own currency.** Chosen.

ACC-002 never goes negative, so the canonical stream never exercises this — but
the engine still needs an answer, and the other two are worse.
`TestFee_IsDenominatedInTheAccountCurrency` pins it.

**Why not 12.50?** The value is given, so the question is really *why not make
the fee proportional*. Because a flat fee is what the brief specifies, and
because a proportional fee interacts badly with the cascade: a fee that scales
with the deficit makes each day's fee larger than the last, and the sweep no
longer converges for reasons that have nothing to do with the account's
behaviour. Flat keeps assessment a fixed point.

### `InterestRateNum / InterestRateDen = 4 / 10000` — 0.04% per day

Given. What was chosen is holding it as an **exact rational** rather than a
parsed decimal literal.

`0.0004` as a float64 is not 0.0004. Even with `shopspring/decimal`, writing
the rate as a string invites someone to write `0.004` and lose a factor of ten
silently. As a numerator and denominator, the rate is two integers that a
reviewer can check against the brief at a glance, and the multiplication happens
before the division so the whole interest calculation rounds exactly once.

**Why not 2/10000?** Halving the rate would change every accrual, and
specifically would collapse the interesting case: at 0.02%, ACC-001's dailies
are 0.05 / 0.05 / 0.13 / 0.08 / 0.08 / 0.08 = 0.47, and the exact unrounded
total is 0.4590 → 0.46. The routes still differ, so criterion 8 would still be
refutable — but the *given* rate is 0.04%, and changing it to make a point would
be arguing against a rule I invented.

### `AED scale 2, BHD scale 3`

Given, and they are the ISO 4217 exponents for those currencies rather than
anything this ledger decided. They live in `app/entity/currency.go` as a lookup
with an explicit "unknown" result: an unrecognised currency returns
`(0, false)`, and the caller must handle it. Defaulting an unknown currency to 2
is how a JPY amount ends up a hundred times too small.

**Why not one scale for everything?** Because the brief has two currencies with
different precision and the difference is load-bearing: BHD 10.000 split three
ways is the whole of criterion 7, and at 2 places the problem disappears
(10.00 / 3 has the same difficulty, but the specific figure 3.334 does not
arise). Storing both at a common higher precision and rounding at display would
be worse — the ledger would hold amounts that cannot be paid.

### `WindowDays = 6`, `CapitalisationDay = 6`

Given: "The window is six days, Day 1 through Day 6" and "capitalize as a single
credit at end of Day 6".

They are separate constants because they answer different questions, and a
system where interest capitalises monthly inside a longer window is the normal
case. Keeping them separate means the capitalisation-day logic is exercised as a
condition rather than as an implicit end-of-loop.

**Why not capitalise on day 3?** The rounded dailies would then have to sum
exactly to *each* capitalisation, not just one — and mid-window capitalisation
raises a question the brief never has to answer: does capitalised interest
itself accrue interest for the rest of the window? See AMBIGUITIES.md.

### `Instalments = 3` for E10

Given. What was chosen is the **allocation**, and it is the largest-remainder
method: every part takes the floor, then the remainder is handed out one minor
unit at a time.

For BHD 10.000 into 3: **3.334 / 3.333 / 3.333**. No two parts differ by more
than a single minor unit — the minimum possible spread — and they sum to exactly
10.000.

**Why not three equal parts?** Because there are none. No 3-decimal value `x`
has `3x = 10.000`. See REJECTED.md, criterion 7.

**Why does the residual go to the first instalment and not the last?** It is a
convention, not a derivation. What matters is that it is *fixed*, so the split
is reproducible rather than dependent on iteration order. Recorded in
AMBIGUITIES.md as a choice rather than a fact.

---

## Chosen

### `FeeReversal = FeeReversalNone` — the default policy

The one runtime choice, and the default is the conservative one.

The brief's rules are non-negotiable and contain an assessment primitive with no
de-assessment primitive. Shipping `on_cause_reversal` as the default would
substitute my judgement for the specification while looking like compliance.

**Why implement the other branch at all?** Because criterion 6 fails on a
*missing rule* rather than on arithmetic, which makes it different in kind from
criteria 2, 7 and 8. Implementing both turns that claim into something a
reviewer can execute rather than take on trust. See REJECTED.md.

### `MaxSweepIterations = 64`

A guard on the fee assessment/reversal fixed point.

The loop provably terminates: assessment tests the fee-**inclusive** balance and
requires it negative, reversal tests the fee-**exclusive** balance and requires
it non-negative, and no single state satisfies both for the same day. So the cap
is never reached in practice, and `TestFee_SweepDoesNotOscillate` pins the
property.

The cap exists because that proof depends on the current rules. A future change
— a proportional fee, a different reversal trigger — could break it, and the
difference between a hung process and a loud error is worth eight lines of code.

**Why not 8?** The window is six days and two accounts, so the real bound is
around 12; 64 is comfortably above any legitimate value and far below anything
that would feel like a hang. **Why not 10,000?** Because at that point a genuine
oscillation looks like a hang rather than an error, which defeats the purpose.

### Rounding mode: half away from zero

`decimal.Round`'s mode, applied at every `NewMoney`.

**It is not load-bearing on this stream.** Not one of the twelve accruals in the
six-day window lands exactly on a half — the products are 0.1000, 0.0900,
0.2500, 0.1660, 0.1560, 0.1560 and 0.0040. Banker's rounding would produce
identical output. `TestMoney_HalfUpRoundsAwayFromZero` pins the mode using
0.125, a value the replay never produces, precisely so the choice is a
deliberate act rather than an accident that happens to be invisible.

**Why not banker's rounding?** It is the better default for large volumes of
independent roundings, because it does not bias upward. Half-up was chosen
because it is what a customer reading a statement expects and can reproduce by
hand, and because the bias argument needs volume this ledger does not have. If
the ledger grew to real volume, this is the first constant I would revisit.

### Capitalisation = the sum of the rounded dailies

Not, in fact, a constant — but it is a number the engine produces, and it is
chosen rather than given.

The brief requires that "the rounded daily accruals must sum exactly to the
capitalized total". Two arrangements satisfy that:

- **the daily is primitive**, and the total is defined as its sum → **0.93**
- the total is primitive, and largest-remainder forces the dailies to fit → 0.92

The first is chosen. The daily accrual is what a subledger actually books each
day; capitalisation is a derived roll-up. Making the total primitive means a
day's booked accrual can be retroactively adjusted by a rounding decision taken
at the end of the window, which is exactly the sort of unexplainable line an
accrual subledger must not contain.

The two differ on this very stream: ACC-001's exact unrounded interest is
**0.9180**, which rounds to 0.92, while the rounded dailies sum to **0.93**.
`TestInterest_CapitalisationSumsExactly` proves both figures rather than
asserting them. This is also why criterion 8 is refused — see REJECTED.md.

### No authorization expiry

The brief gives no expiry rule, so holds do not expire. Auth-B is declined and
Auth-A settles on Day 4, so nothing in the window depends on it.

**Why not expire holds after 5 days?** Because it would be invented, and it
would silently change the answer: a 5-day expiry releases Auth-B's hold inside
the window, and the brief's closing line ("Auth-B is never settled inside the
window") stops being about an outstanding hold at all. Inventing a rule that
changes the result is worse than leaving a gap and naming it.

### Full hold release on settlement

Auth-A holds 200.00 and settles 185.00. The remaining 15.00 is released, not
retained.

The authorization is closed by the settlement, so there is nothing left for a
residue to secure. Retaining it would mean holding funds against a
reservation that can never be drawn again — and with no expiry rule, that hold
would never be released at all. See AMBIGUITIES.md.
