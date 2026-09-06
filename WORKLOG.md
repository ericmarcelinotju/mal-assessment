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
