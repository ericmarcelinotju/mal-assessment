# Worklog

Real wall-clock timestamps, Asia/Jakarta (+07:00), captured from the machine as
the work happened. Not reconstructed afterwards.

## 2026-09-06

- **21:26** Received the brief. Read the supplied Go boilerplate (gin + MongoDB +
  a private Bitbucket module `jet_go_lib`) to decide how much of it survives a
  task that forbids web, persistence, UI and database layers.
- **21:34** Worked the event stream by hand on paper before writing any code.
  Established that the exercise is bitemporal: E7 posts on Day 5 with value date
  Day 2, so "the Day 2 closing balance" is not one number but a number *as
  observed on some day*. Criterion 1 pins both coordinates, which is the tell.
- **21:41** Second, independent computation pass over the whole stream to check
  the first. Both agree on ACC-001 closing at AED 390.00 before capitalisation.
  The two passes initially disagreed on E6 (orphan settlement): reject versus
  force-post. Resolved to reject, per criterion 4; the force-post numbers are
  recorded in AMBIGUITIES.md rather than discarded.
- **21:46** Fixed the eight criteria verdicts: 3 accepted, 1 accepted-but-vacuous
  (C5), 1 rejected-but-repairable (C6), 3 unacceptable (C2, C7, C8).
- **21:55** Decided C6 deserves both branches in code rather than a paragraph of
  prose, selectable by `LEDGER_FEE_REVERSAL_POLICY`. Default `none`, because the
  brief grants no de-assessment primitive and defaulting to my own extension
  would quietly overwrite the spec.
- **22:01** Scaffolded the repository. De-branded copies of `apperror` and
  `logger` from the boilerplate; dropped gin, MongoDB, viper and `jet_go_lib`
  entirely. Fixed a latent nil-factory panic in the copied `apperror` (see
  REJECTED.md).
- **22:12** Wrote `Money` over int64 minor units. Replaced it minutes later with
  `shopspring/decimal` at the reviewer's direction; kept the quantise-at-
  construction invariant so arbitrary-precision dust cannot enter the ledger.
- **22:18** Verified `decimal.Round` half-away-from-zero against all eight
  interest figures the replay will produce, before building anything on top.
- **22:26** Engine complete and verified against the hand computation: ACC-001
  closes Day 6 at AED 390.93 with three fees, ACC-002 at BHD 10.008, E6
  rejected, Auth-B declined at −245.00. Found one real bug on the first full
  run — `REVERSAL` events carry no stated amount, so the currency guard rejected
  E9. Split `IsMonetary` into `HasStatedAmount`; a reversal derives its amount
  from the entries it reverses, which is the only way it is guaranteed to undo
  exactly what was done.
- **22:31** Verified the `on_cause_reversal` branch reaches a fixed point on the
  pre-E7 counterfactual: 466.03, interest 1.03, net fees 0.00. Criterion 6
  becomes exactly true under it.
- **22:40** Report renderer. Printed both clocks and the full interest
  restatement trail rather than net figures — the Day 2 trail
  (+0.10 / −0.10 / +0.09) is the whole argument for the append-only accrual
  design and is worth showing rather than describing.
- **22:55** Test suite. Two tests exist to be honest rather than to pass:
  `TestFee_CascadeIsCoveredSynthetically` fills a gap the brief's own data
  leaves — on the canonical stream the fee cascade changes no outcome, so the
  given events cannot distinguish that design from the alternative.
  `TestFee_Criterion2BoundaryCase` shows criterion 2 fails by the size of one
  settlement rather than being absurd.
- **23:02** Caught a bad test of my own. The subtest claiming to show the
  0.93-versus-0.92 divergence used `MulRatioHalfUp(4, 1000000)` — wrong by two
  orders of magnitude — and passed vacuously. Rewritten to sum the unrounded
  daily products in `decimal` and assert 0.918 → 0.92 against the engine's 0.93.
  The claim now has evidence behind it instead of a green tick.
- **23:10** Annotated failing test. It asserts Day 6 closes at 466.03 and fails
  at 390.93. Left red deliberately: the repair exists in the same repository,
  one environment variable away, but the brief's rules contain no de-assessment
  primitive and making my own extension the default would hide the disagreement
  worth having.
- **23:14** `make` on this machine (cygwin build under Git Bash) exits 127 and
  prints nothing, including for `make --version` — a broken local toolchain, not
  a Makefile problem. The Makefile is standard GNU make and is shipped, but the
  README leads with the plain `go` commands, which are the ones I actually ran.
- **23:30** Wrote README, NUMBERS, AMBIGUITIES and REJECTED. Deliberately wrote
  the strongest case *for* each refused criterion before the case against it;
  criterion 6 in particular is a better argument than the other three and
  deserved to be met rather than dismissed.
