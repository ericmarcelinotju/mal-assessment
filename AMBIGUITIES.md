# AMBIGUITIES

Every place the brief admits more than one reading, what I chose, and — where it
matters — what the alternative would have cost in actual figures.

They are ordered by how much the choice moves the numbers. The first four change
the answer; the rest change the design.

---

## 1. Is "the Day 2 closing balance" one number?

**No, and this is the exercise.**

E7 posts on Day 5 carrying value date Day 2. Every entry therefore has two
clocks: a *posting day* (when the ledger learned of it) and a *value date* (when
it economically applies). "Day 2's closing balance" is a function of both:

| observed from | Day 2 closing balance |
|---|---|
| end of Day 2 | **+250.00** |
| end of Day 5, pre-fee | **−370.00** |
| end of Day 5, post-fee | −395.00 |
| end of Day 6 | +225.00 |

All four are correct. Criterion 1 pins both coordinates — "evaluated at end of
Day 5 and before any fee is assessed" — which is what makes it answerable at
all, and is the tell that this is what is being tested.

**Resolution.** Entries carry both dates. `closingBalance` gives the value-dated
figure, `closingBalanceAsObserved` restricts it to what was known on a given
processing day. `TestBalance_IsBitemporal` asserts both readings.

Backdated postings are not an edge case to be tolerated — they are routine in
core banking (late network files, corrections, back-value adjustments). What
append-only forbids is *mutating* history, not *learning* about it late.

## 2. Is an orphan settlement rejected or force-posted? — worth 180.00

E6 settles "Auth-Z", which has no preceding authorization. Criterion 4 says
reject it. Card networks, in reality, often **force-post** an unmatched
settlement and flag it for investigation, on the grounds that the merchant has
already been paid and refusing the entry does not un-pay them — it just creates
an unreconciled break somewhere else.

Both are defensible. This is the largest open choice in the brief:

| | reject *(chosen)* | force-post |
|---|---|---|
| Day 4 close | **415.00** | 235.00 |
| Day 5 close | **390.00** | 210.00 |
| Day 6 close | **390.93** | 210.69 |
| interest | **0.93** | 0.69 |
| fees | 3 (Days 2, 4, 5) | 3 (Days 2, 4, 5) |
| Auth-B | declined | declined |

**Resolution: reject.** Two reasons beyond deferring to criterion 4. First, an
in-memory core with no upstream network to reconcile against has no basis on
which to fabricate a debit — the force-post argument rests on knowledge this
system does not have. Second, rejection is the recoverable option: a rejected
settlement can be re-presented once its authorization arrives, whereas a
wrongly-booked debit has already left the account.

Worth noting the choice does **not** change the fee count or the Auth-B
decision, so the parts of the exercise that test judgement are robust to it.

**Left open:** whether a late-arriving authorization should retroactively match
an already-rejected settlement. Not exercised here; would need a matching queue.

## 3. Do backdated entries restate earlier interest accruals? — worth 0.22

Days 1–4 accrued interest on balances that E7 later invalidated. Nothing is
capitalised until Day 6, so the accruals are still correctable.

| | restate *(chosen)* | freeze at each day's close |
|---|---|---|
| Day 1 | 0.10 | 0.10 |
| Day 2 | **0.09** | 0.10 |
| Day 3 | **0.25** | 0.26 |
| Day 4 | **0.17** | 0.19 |
| Day 5 | **0.16** | 0.00 |
| Day 6 | 0.16 | 0.16 |
| **total** | **0.93** | 0.71 |

**Resolution: restate, by appending corrections.** Interest is a function of the
value-dated balance; if the balance was wrong, the accrual was wrong, and it has
not yet been paid. This is a back-value interest adjustment, which is what a
core banking system does.

Crucially, restating does **not** breach append-only. The Day 2 record still
reads `+0.10` exactly as written; a `−0.10` correction is appended, then `+0.09`.
The net is the sum, and the trail shows an auditor what the ledger believed and
when. Freezing is simpler and defensible, but it leaves the accrual subledger
describing balances the ledger no longer holds.

## 4. Are overdraft fees de-assessed when their cause is reversed? — worth 75.10

The brief grants an assessment primitive and no de-assessment primitive. So
under the literal reading the three fees stand even after E9 reverses E7 — which
is what makes criterion 6 false.

But de-assessment does not breach append-only either: a fee is reversed by
appending a contra entry. And criterion 6 fails on a *missing rule* rather than
on arithmetic, unlike criteria 2, 7 and 8.

**Resolution: implement both, default to the literal reading.**
`LEDGER_FEE_REVERSAL_POLICY=none` (default) or `on_cause_reversal`. Under the
extension every figure lands on the pre-E7 counterfactual (466.03, interest
1.03, net fees 0.00) and criterion 6 becomes exactly true.

Defaulting to the extension would substitute my judgement for the specification
while looking like compliance. Shipping only the literal reading would make the
objection unfalsifiable. See REJECTED.md, criterion 6.

**Left open, and I think genuinely unresolved:** the implemented policy reverses
a fee whenever its day recovers *for any reason*, including a genuine deposit
days later — which is too generous, since an overdraft that really happened
should still be charged. Doing this properly needs a causal link from a fee to
the entries that triggered it, which the data model does not currently carry.
This is the substance of the annotated failing test.

## 5. Sum-of-rounded-dailies, or largest-remainder against the exact total? — worth 0.01

"The rounded daily accruals must sum exactly to the capitalized total" is
satisfied by two different arrangements, and they disagree here:

- **daily is primitive**, total is its sum → 0.10+0.09+0.25+0.17+0.16+0.16 = **0.93**
- total is primitive (exact 0.9180 → 0.92), dailies adjusted to fit → **0.92**

**Resolution: the daily is primitive.** It is what a subledger books each day;
capitalisation is a derived roll-up. The alternative lets a rounding decision
taken at the end of the window retroactively adjust a day's booked accrual,
which is exactly the sort of unexplainable line an accrual subledger must not
contain.

Both satisfy the stated rule, so this is a real choice and not a correctness
question. It is also why criterion 8 is refused: see REJECTED.md.

## 6. "Three equal instalments" of BHD 10.000 is unsatisfiable

BHD stores three decimals and no 3dp value `x` has `3x = 10.000`. The brief asks
for equality *and* for a total of 10.000, and only one can hold.

**Resolution: conservation wins.** 10.000 is the credit; equality only describes
how it is delivered. Largest-remainder gives 3.334 / 3.333 / 3.333 — the minimum
possible spread, one minor unit.

A third reading exists and is worth naming: post three instalments of 3.334 each
*plus* a fourth −0.002 rounding true-up. Each instalment is then literally 3.334
and value is conserved. Rejected because it invents 0.002 and claws it back, the
true-up entry has no business meaning, and "three instalments" becomes four
entries.

## 7. The stream is listed out of order

E9 is listed before E10, but E9 posts on Day 6 and E10 on Day 5. "Replayed in
this order" and "replayed in time order" disagree.

**Resolution: posting day wins; listed order breaks ties within a day.** A
ledger cannot learn of a Day 6 event before a Day 5 one. The transcription in
`stream.go` keeps the brief's ordering verbatim and the reordering happens in
the engine, so the discrepancy stays visible instead of being tidied away in the
data.

The two events touch different accounts, so nothing here depends on it — but a
replay engine whose answer varies with input order gives different results for
the same facts. `TestAppendOnly_PostingOrderBeatsListedOrder` pins it, including
against a fully reversed stream.

## 8. Are debits and settlements subject to the availability test?

The brief applies the test to authorizations only.

**Resolution: authorizations only.** A debit that cannot be funded is precisely
the situation the overdraft fee exists to price — if debits were refused, the
fee rule could never fire at all. A settlement is exempt for a different reason:
the funds were reserved when the authorization was approved, and reneging on a
promise already made to a merchant is not a decision available at settlement
time. `TestSettlement_AuthAIsAccepted` covers a settlement that overdraws.

## 9. What happens to the unused 15.00 of Auth-A's hold?

Auth-A holds 200.00 and settles 185.00.

**Resolution: release the hold in full.** The authorization is closed, so there
is nothing left for a residue to secure — and with no expiry rule in the brief, a
retained 15.00 would never be released at all. Nothing in the window depends on
it: Auth-B is declined either way (−245.00, or −260.00 with a residue).

## 10. Does the Day 6 interest credit participate in the Day 6 fee sweep, or in its own accrual?

**Resolution: neither.** Capitalisation runs last in the day close, after the
fee sweep and after accrual. It is interest *on* the window, not part of the
window's balances; letting it feed either would be circular.

Both accounts close Day 6 positive, so this is not exercised — but it would
decide whether interest can rescue a day from its own overdraft fee, and the
answer is no. `TestInterest_CapitalisationDoesNotFeedItself` pins it: Day 6
closes at 390.93 but accrues on 390.00.

## 11. May a day whose fee was reversed be charged again?

Only arises under `on_cause_reversal`. "Once per day per account, ever" is
coherent only while fees are irreversible.

**Resolution: under the reversal policy, a day becomes chargeable again.** The
cap relaxes to once per overdraft *episode*. Under the default policy the
original "ever" cap stands, since nothing is ever reversed. Not exercised by the
canonical stream.

## 12. In what order are fees swept, and does a fee cause another fee?

Fees are ledger entries, so a fee on Day *d* is inside the `value_date <= d+1`
set and compounds.

**Resolution: sweep ascending, with cascade.** A fee booked on an earlier day is
already in the balance when a later day is tested.

**This is an honest coverage gap in the brief's own data.** On the canonical
stream the cascade changes no outcome — the Day 2 fee narrows Day 3 from +30.00
to +5.00, still positive — so the given events cannot distinguish
ascending-with-cascade from evaluating every day against pre-fee balances.
`TestFee_CascadeIsCoveredSynthetically` supplies a case that can: a day pushed
negative *only* by the previous day's fee.

## 13. In what currency is the overdraft fee charged on a BHD account?

The brief says "AED 25.00"; ACC-002 is BHD.

**Resolution: 25 units of the account's own currency.** Booking AED into a BHD
account makes every balance on it ambiguous; converting requires a rate the
brief never supplies. Not exercised — ACC-002 never goes negative — but the
engine needs an answer. See NUMBERS.md.

## 14. Which rounding mode?

**Resolution: half away from zero**, and it makes no difference here: not one of
the twelve accruals in the window lands on a half, so banker's rounding gives
identical output. Pinned by a test using a value the replay never produces, so
the choice is deliberate rather than invisible. See NUMBERS.md for why half-up
over banker's, and why that would be the first thing I revisited at volume.

---

## Ambiguities I resolved by reading the brief more carefully, not by choosing

- **"at or above zero"** — an authorization landing on exactly 0.00 is approved;
  only a strictly negative result declines. Pinned one minor unit either side by
  `TestAuthorization_BoundaryIsAtOrAboveZero`.
- **"positive balances only"** for interest — a zero balance accrues nothing, and
  a negative balance is priced by the overdraft fee instead. Charging both would
  be charging twice for one condition.
- **"booked with value_date equal to the day assessed"** — "the day assessed" is
  the day *being* assessed, not the day the assessment runs. The Day 2 fee is
  value-dated Day 2 and posted Day 5. Any other reading makes the clause
  redundant, since it would just restate the posting day.
