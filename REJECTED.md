# REJECTED

Acceptance criteria refused, with the strongest argument *for* each one before
the argument against it — a refusal that only engages the weak reading is not a
refusal. Then approaches abandoned during the build.

Every verdict here is executable:
[criteria_test.go](app/module/ledger/test/criteria_test.go) has one test per
criterion, asserting the claim where accepted and its negation where refused.

| # | Verdict |
|---|---|
| 1 | ACCEPTED |
| 2 | **REFUSED** — self-contradictory |
| 3 | ACCEPTED |
| 4 | ACCEPTED (policy choice; alternative documented) |
| 5 | Principle accepted, **premise refused** — vacuous |
| 6 | **REFUSED as stated** — repairable by adding a rule |
| 7 | **REFUSED** — invents 0.002 |
| 8 | **REFUSED as a rule** — vacuous here, hazardous in general |

---

## Criterion 2 — REFUSED

> E7 causes exactly one overdraft fee to be assessed, on Day 2.

**Refused. It demands two mutually exclusive properties.**

### The strongest case for it

E7 is value-dated Day 2, and Day 2 is the day that goes negative first —
−370.00, exactly as criterion 1 says. Day 3's credit of 400.00 then restores the
account. So the intuition is: one back-valued debit, one overdrawn day, one fee.
That is a coherent picture, and it is nearly right.

### Why it fails anyway

To assess a fee dated Day 2 **at all**, E7 must be inside the
`value_date <= Day 2` set — that is the only way a day which closed at +250.00
gets reopened.

But the same set-membership rule puts E7 inside `value_date <= Day 4` and
`value_date <= Day 5`, because Day 2 ≤ Day 4 and Day 2 ≤ Day 5. A back-valued
debit does not land on one day; it depresses **every** subsequent cumulative
balance:

| day | pre-fee | |
|---|---|---|
| 2 | −370.00 | fee |
| 3 | +30.00 | recovers |
| 4 | −155.00 | negative again — fee |
| 5 | −155.00 | still negative — fee |

Accepting the Day 2 fee *forces* the Day 4 and Day 5 fees. **Three, not one.**

The only reading yielding exactly one fee is "a day's fee is decided at its own
close and never revisited". That is coherent — but it puts the single fee on
**Day 5**, not Day 2, because Day 2 closed at +250.00 on Day 2's information.
Retroactive assessment and no-forward-propagation cannot both hold unless
closing balance is redefined as a per-day delta, which the brief's own
parenthetical — "(all entries with value_date ≤ that day)" — forbids.

### How narrowly it fails

Worth stating precisely, because the criterion is wrong rather than absurd. The
Day 3 credit **does** restore the account to +5.00. What kills it is E5: settling
185.00 on Day 4 pushes it back under. Had Auth-A settled for 5.00 or less, Day 4
would have stayed positive, Day 5 with it, and E7 really would have caused
exactly one fee dated Day 2.

`TestFee_Criterion2BoundaryCase` runs both settlement amounts and shows the
count flipping from 3 to 1. The criterion is wrong about this stream by the size
of one settlement.

**No acceptance path exists.**

---

## Criterion 6 — REFUSED as stated

> After E9, all balances and fees return to their pre-E7 values.

**Refused under the brief as written. Repairable by adding one rule — and the
repair is implemented in this repository.**

### The strongest case for it

E7 and E9 are equal, opposite, and share a value date. On the entries alone they
cancel perfectly: 1200 − 950 − 620 + 620 + 400 − 185 = 465.00, which *is* the
pre-E7 balance. And a fee assessed for an overdraft that the corrected record
says never happened is, on the face of it, a fee that should not stand.

This is a better argument than criteria 2, 7 or 8 have. It is not arithmetically
wrong at all.

### Why it fails as stated

The brief's non-negotiable rules grant an assessment primitive and **no
de-assessment primitive**. Nothing in the specified rule set can give the money
back, so the three fees stand:

- Day 6 closes at **390.93**, not 466.03
- fees standing: **75.00**, not 0.00
- interest: 0.93, not 1.03 — the fees suppressed 0.10 of accrual too

The permanent divergence is **75.10**.

Note this is a failure of the *rule set*, not of the arithmetic — which is what
makes it different in kind from the other three refusals.

### The repair, and why it is not the default

De-assessment does **not** breach append-only: a fee is reversed by appending a
contra entry, and the original is never touched. So the missing rule can be
added without violating anything the brief insists on.

It is implemented as `LEDGER_FEE_REVERSAL_POLICY=on_cause_reversal`. Under it,
the sweep reaches a fixed point on the pre-E7 counterfactual:

| step | D1 | D2 | D3 | D4 | D5 |
|---|---|---|---|---|---|
| before sweep | 250.00 | 225.00 | 625.00 | 415.00 | 390.00 |
| reverse fee@D2 | 250.00 | 250.00 | 650.00 | 440.00 | 415.00 |
| reverse fee@D4 | 250.00 | 250.00 | 650.00 | 465.00 | 440.00 |
| reverse fee@D5 | 250.00 | 250.00 | 650.00 | 465.00 | **465.00** |

Interest restates to 1.03, Day 6 closes at **466.03**, net fees **0.00** —
criterion 6 becomes exactly true.
`TestCriterion6_RefusedUnderDefaultPolicy` proves it by computing the
counterfactual independently, replaying the stream with E7 and E9 deleted, and
comparing day for day.

**It is off by default** because the brief's rules are non-negotiable and do not
contain de-assessment. Making my own extension the default would substitute my
judgement for the specification while looking like compliance.

**And the repair is still not right.** It reverses a fee whenever its day
recovers for *any* reason — including a genuine deposit days later — which is too
generous, since an overdraft that really happened should still be charged. Doing
it properly needs a causal link from a fee to the entries that triggered it,
which the data model does not carry. That is the substance of the annotated
failing test, [known_gap_test.go](app/module/ledger/test/known_gap_test.go).

---

## Criterion 7 — REFUSED

> The three BHD instalments in E10 must each be BHD 3.334.

**Refused. It credits 0.002 that nobody instructed.**

    3 × 3.334 = 10.002 ≠ 10.000

### The strongest case for it

3.333 is worse in one specific way: 3 × 3.333 = 9.999 **short-changes** the
customer, and 3.334 at least errs in the customer's favour. If forced to choose
between two equal splits, 3.334 is the defensible one. The criterion is
answering a real question.

There is also a reading on which it is literally true: post three instalments of
3.334 each **plus a fourth −0.002 rounding true-up**. Every instalment is then
exactly 3.334 and value is conserved.

### Why it fails anyway

The choice is not between the two equal splits, because a third option
conserves value exactly:

    3.334 + 3.333 + 3.333 = 10.000

Three genuinely equal 3dp instalments summing to 10.000 do not exist — 10.000 is
not divisible by 3 in thousandths. The brief asks for equality **and** for a
total of 10.000, and only one can hold. Conservation wins: 10.000 is the credit,
equality merely describes how it is delivered.

The true-up reading is rejected on merit rather than arithmetic: it invents
0.002 and claws it back, the true-up entry carries no business meaning anyone
could explain on a statement, and "three instalments" quietly becomes four
entries.

`TestSplitInstalments_RefutesCriterion7` asserts all three sums.

---

## Criterion 8 — REFUSED as a rule

> If the rounded daily interest accruals do not sum to the capitalized total,
> the remainder is discarded.

**Refused. It contradicts a non-negotiable rule, and the discrepancy on this
stream runs the opposite way from what it assumes.**

### The strongest case for it

Under this design the antecedent never fires. The capitalised total is *defined
as* the sum of the dailies, so they always agree, so there is never a remainder
to discard. Read that way, criterion 8 is vacuously true and describes exactly
what the engine does. Accepting it would cost nothing today.

### Why it is refused anyway

The brief's own non-negotiable rule says the rounded daily accruals **must sum
exactly** to the capitalised total. "Discard the remainder" is precisely the
operation that makes them not sum exactly. A clause cannot be accepted on the
grounds that it never fires when the whole reason it never fires is that the
design already refuses it.

And it is not academic here. ACC-001's exact unrounded interest is **0.9180**,
which rounds to **0.92**, while the rounded dailies sum to **0.93**:

| day | balance | × 0.0004 | rounded |
|---|---|---|---|
| 1 | 250.00 | 0.1000 | 0.10 |
| 2 | 225.00 | 0.0900 | 0.09 |
| 3 | 625.00 | 0.2500 | 0.25 |
| 4 | 415.00 | 0.1660 | **0.17** |
| 5 | 390.00 | 0.1560 | **0.16** |
| 6 | 390.00 | 0.1560 | **0.16** |
| | **0.9180** → 0.92 | | **sum 0.93** |

The two routes genuinely disagree by a cent on this very stream. So the moment
anyone computes the total independently — a perfectly natural thing to do —
"discard the remainder" opens a permanent one-cent break between the accrual
subledger and the capitalisation entry. Small, permanent, and exactly the kind
of break that makes a ledger untrustworthy.

Note also the sign: capitalisation slightly **over**-credits here (0.93 against
an exact 0.9180). There is nothing to discard; if anything there is something to
invent. The criterion has assumed the error runs one way when on this data it
runs the other.

`TestInterest_CapitalisationSumsExactly` proves both figures.

---

## Criterion 5 — principle accepted, premise refused

> If Auth-B is approved, its hold reduces available balance but not ledger
> balance.

**The consequent is correct hold semantics. The antecedent is false.**

Auth-B is **declined**. By the time E8 arrives on Day 5, E7 has already posted
back-valued to Day 2 and the account is 155.00 overdrawn:

    available −155.00 − hold 90.00 = −245.00 < 0

The criterion is therefore vacuously true. It is worth flagging rather than
waving through, because the brief's closing line — "Auth-B is never settled
inside the window" — invites the reader to assume an approved hold that simply
never settles. It is not outstanding; it never existed.

The principle itself is right and is tested on Auth-A, the hold that does exist:
Day 3 shows ledger 625.00, holds 200.00, available 425.00.

The decline is robust to ordering: −245.00 before the fee sweep, −320.00 after.

---

## Criteria accepted, re-attacked

Refusing four is only credible if the other four were attacked as hard.

**Criterion 1** (Day 2 closes at −370.00 from Day 5, pre-fee). Verified:
1200.00 − 950.00 − 620.00. It also survives the force-post reading of E6, since
E6 is value-dated Day 4 and cannot reach Day 2. Holds unconditionally.

**Criterion 3** (the Day 4 settlement of Auth-A must be accepted). Auth-A exists
and is active, and 185.00 is within the 200.00 hold. It survives even the
counterfactual where settlements *are* subjected to the availability test:
450.00 available against a 185.00 settlement. Holds unconditionally.

**Criterion 4** (an orphan settlement must be rejected, funds must not leave).
Accepted — but this is a **policy choice, not a derivation**, and the honest
version of accepting it is saying so. Card networks do force-post unmatched
settlements. AMBIGUITIES.md §2 gives the full numeric sensitivity: force-posting
moves Day 6 from 390.93 to 210.69. Accepted because an in-memory core with no
upstream network to reconcile against has no basis on which to fabricate a
debit, and because rejection is recoverable where a wrong debit is not.

---

## Approaches abandoned mid-build

**`int64` minor units for Money.** The first implementation stored amounts as
integer minor units — 46500 for AED 465.00 — with a hand-rolled half-up divide.
Exact, fast, and impossible to get sub-minor-unit dust into. Replaced with
`shopspring/decimal` on the reviewer's direction. The invariant survived the
change and is the more important half: every `Money` is quantised to its
currency's scale at construction, so decimal's arbitrary precision cannot leak
into the ledger. Without that, `10/3` is representable to sixteen places and
will eventually fail to reconcile.

**A single-clock ledger.** The first sketch of `LedgerEntry` had one `Day`
field. It survived about ten minutes — until E7, which cannot be represented at
all without separating posting day from value date. Everything else in the
exercise follows from that split, so it is worth naming as the mistake it would
have been.

**Mutating entries in place for reversals.** Briefly considered marking E7 as
`reversed = true` rather than appending a contra entry. Rejected: it breaks
append-only outright, and it also loses information — the value-dated position
of Days 2 through 5 *between* E7 and E9 becomes unrecoverable, and that interval
is exactly where the three fees were assessed.

**Rejecting debits that would overdraw.** Considered applying the availability
test to debits as well as authorizations, for symmetry. Abandoned once it became
clear it would make the overdraft fee rule unreachable: if no debit can overdraw,
no day can close negative, and the fee never fires. The asymmetry is the point.

**Caching each day's figures at close time.** The report originally stored
balances and fees as they were computed. Removed in favour of deriving
everything on demand from the log. With back-value postings, a cached figure is
wrong the moment a later event lands, and the resulting drift between the report
and the entries justifying it is precisely the class of bug this exercise is
built to expose.

**A nil-factory panic inherited from the boilerplate.** The copied `apperror`
package left its package-global factory nil until `Init` was called, so the very
first `apperror.New` dereferenced nil — a latent panic on every error path,
including one in the boilerplate's own test. Fixed by initialising the factory
eagerly, with `Init` retained for setting a service prefix.
