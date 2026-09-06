# In-memory account ledger core

An append-only, bitemporal account ledger. No web layer, no persistence, no UI,
no database — the engine is a pure function of the event stream, so the six-day
replay is deterministic and every figure below is reproducible.

Built on the supplied Go boilerplate: same layout (`app/entity`,
`app/module/<name>/test`), same idioms (interface + unexported struct + `New`,
coded errors, `log/slog`), with gin, MongoDB, viper and the private
`jet_go_lib` module removed. A public repository has to build for anyone who
clones it, and the brief forbids the layers those dependencies serve.

---

## Running it

Requires Go 1.25+. Nothing else — no database, no services, no config files.

```bash
go run main.go
```

That replays the ten-event stream across Day 1 to Day 6 and prints, per day per
account: closing ledger balance, fee assessments, authorization states, and
errors — followed by the full append-only log.

The one genuine runtime choice is what happens to an overdraft fee when the
entry that caused it is reversed. Both readings are implemented:

```bash
LEDGER_FEE_REVERSAL_POLICY=on_cause_reversal go run main.go
```

| | `none` *(default)* | `on_cause_reversal` |
|---|---|---|
| fees standing | 3 × 25.00 = **75.00** | **0.00** (3 assessed, 3 reversed) |
| interest capitalised | **0.93** | **1.03** |
| ACC-001 Day 6 close | **AED 390.93** | **AED 466.03** |
| acceptance criterion 6 | FALSE | TRUE |

`none` is the default because the brief grants an assessment primitive and no
de-assessment primitive. Defaulting to my own extension would quietly overwrite
the specification. See [REJECTED.md](REJECTED.md), criterion 6.

## Running the tests

```bash
go test ./... -count=1
```

**This reports exactly one failure, and that is correct.**
`TestKnownGap_OverdraftFeesSurviveReversalOfTheirCause` is the deliberately
failing test the brief asks for. It is not an unfixed bug — it is an argument
against the specification, annotated in full at
[known_gap_test.go](app/module/ledger/test/known_gap_test.go).

For a green run:

```bash
go test ./... -count=1 -skip 'TestKnownGap_'
```

Equivalent `make` targets exist — `make run`, `make run-reversal`, `make test`,
`make verify`, `make test-coverage` — but the `go` commands above are the
primary, verified path.

---

## Reading the output

Each day prints one block per account:

```
  ACC-001  (AED)
    closing ledger balance           225.00     every entry with value_date <= this day
    active holds                     200.00     approved, unsettled authorizations
    available balance                 25.00     closing balance minus active holds
    overdraft fee                    -25.00   assessed on day 5
    interest accrued (net)             0.09
        accrued           0.10   booked day 2, on balance 250.00
        adjusted         -0.10   booked day 5, on balance -395.00
        adjusted          0.09   booked day 6, on balance 225.00
    entries
        E7          -620.00  DEBIT              [posted day 5, back-valued]
```

Three things are worth knowing to read this.

**Two clocks.** Every entry carries a *posting day* (when the ledger learned of
it) and a *value date* (when it economically applies). E7 posts on Day 5
carrying value date Day 2. Entries where the two differ are flagged
`[back-valued]`, and the log marks them `*`.

**A day's balance depends on when you ask.** Day 2 closed at **+250.00**
observed from Day 2, and at **−370.00** observed from Day 5, because E7 had not
been posted yet on Day 2. Both are correct. Acceptance criterion 1 pins both
coordinates, which is the tell that this is what the exercise is testing.

**The accrual trail is append-only.** The three lines under Day 2's interest are
not a recalculation — they are three separate records. When E7 invalidated the
Day 2 accrual, the original `+0.10` was left exactly as written and a `−0.10`
correction was appended; `+0.09` followed when E9 reversed E7. The net is the
sum. Nothing is ever rewritten.

---

## What the replay produces

ACC-001 (AED), closing balances as observed at the end of the window:

| Day | 1 | 2 | 3 | 4 | 5 | 6 |
|---|---|---|---|---|---|---|
| **closing** | 250.00 | 225.00 | 625.00 | 415.00 | 390.00 | **390.93** |
| **interest** | 0.10 | 0.09 | 0.25 | 0.17 | 0.16 | 0.16 |

Fees: three, all assessed on processing Day 5 when E7 arrived, value-dated
Day 2, Day 4 and Day 5. Interest capitalised: **AED 0.93**, one credit on Day 6.

ACC-002 (BHD): flat at 0.000 until Day 5, then 10.000 — posted as
**3.334 / 3.333 / 3.333**, which sum to exactly 10.000. Interest 0.004 + 0.004,
capitalised **BHD 0.008**, closing **BHD 10.008**.

Authorizations: Auth-A approved Day 2, settled Day 4 (hold released in full).
Auth-Z rejected — no such authorization. **Auth-B declined**: available −155.00
less a 90.00 hold is −245.00.

Errors: two, both engineered by the brief — E6 (orphan settlement) and E8
(insufficient funds).

---

## The acceptance criteria

Some of the supplied criteria are wrong. Verdicts:

| # | Claim | Verdict |
|---|---|---|
| 1 | Day 2 closes at −370.00 from Day 5, pre-fee | **ACCEPTED** |
| 2 | E7 causes exactly one fee, on Day 2 | **REFUSED** — self-contradictory |
| 3 | The Day 4 settlement of Auth-A must be accepted | **ACCEPTED** |
| 4 | An orphan settlement must be rejected | **ACCEPTED** |
| 5 | If Auth-B is approved, its hold cuts available not ledger | **VACUOUS** — Auth-B is declined |
| 6 | After E9, balances and fees return to pre-E7 values | **REFUSED** — repairable by adding a rule |
| 7 | The three BHD instalments are each 3.334 | **REFUSED** — sums to 10.002 |
| 8 | An unmatched interest remainder is discarded | **REFUSED** — contradicts a non-negotiable rule |

Reasoning in [REJECTED.md](REJECTED.md), including the strongest counter-argument
for each refusal. The verdicts are also executable: `criteria_test.go` has one
test per criterion, asserting the claim where accepted and its negation where
refused.

---

## Documents

| | |
|---|---|
| [NUMBERS.md](NUMBERS.md) | every constant, and why that value and not half it |
| [AMBIGUITIES.md](AMBIGUITIES.md) | every ambiguity found, how it was resolved, and what the alternative would have cost |
| [REJECTED.md](REJECTED.md) | criteria refused with reasoning, plus approaches abandoned mid-build |
| [WORKLOG.md](WORKLOG.md) | timestamped record of the work |

## Layout

```
main.go                      runnable replay
config/ledger.go             every constant, one file, 1:1 with NUMBERS.md
app/entity/                  Money (shopspring/decimal), Event, LedgerEntry, Accrual
app/module/ledger/
  ledger.go                  the append-only log — no update or delete exists
  balance.go                 bitemporal balance projections
  authorization.go           availability test
  settlement.go              auth matching, orphan rejection, reversal
  fee.go                     overdraft sweep + policy-gated reversal sweep
  interest.go                accrual, restatement, capitalisation
  instalment.go              largest-remainder split
  engine.go                  Post / CloseDay / Report
  stream.go                  the ten canonical events
  test/                      the suite
app/presenter/report.go      text rendering
apperror/, logger/           de-branded from the boilerplate
```

`Money` wraps `shopspring/decimal` and adds one invariant: every amount is
quantised to its own currency's scale at construction, so sub-minor-unit dust
cannot enter the ledger. AED is 2 places, BHD is 3, taken from ISO 4217
exponents and never inferred from a value.
