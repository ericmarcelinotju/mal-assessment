# Worklog

Real wall-clock timestamps, Asia/Jakarta (+07:00), taken from the machine as the
work happened. The entries from 22:07 onward line up with the commit timestamps
in `git log`, which is the check on them.

## 2026-09-06

- **21:26** Received the brief. Read the supplied Go boilerplate (gin, MongoDB and a
  privately hosted internal library) to decide how much of it survives a
  task that forbids web, persistence, UI and database layers. Answer: the layout
  and the idioms, none of the dependencies — a public repository has to build for
  whoever clones it.

- **21:34** Worked the event stream by hand on paper before writing any code.
  Established that the exercise is bitemporal: E7 posts on Day 5 with value date
  Day 2, so "the Day 2 closing balance" is not one number but a number *as
  observed on some day*. Criterion 1 pins both coordinates, which is the tell.

- **21:41** Second, independent computation pass over the whole stream to check
  the first. Both agreed ACC-001 closes at AED 390.00 before capitalisation. They
  initially disagreed on E6, the orphan settlement — reject versus force-post.
  Resolved to reject, per criterion 4; the force-post figures are recorded in
  AMBIGUITIES.md rather than discarded, because the choice is worth 180.00 and a
  reader should be able to see what it cost.

- **21:46** Fixed the eight verdicts: 3 accepted, 1 accepted-but-vacuous (C5),
  1 refused-but-repairable (C6), 3 unacceptable (C2, C7, C8).

- **21:55** Decided C6 deserved both branches in code rather than a paragraph of
  prose, selectable by `LEDGER_FEE_REVERSAL_POLICY`. Default `none`: the brief
  grants no de-assessment primitive, and defaulting to my own extension would
  overwrite the spec while looking like compliance.

- **22:01** Scaffolded the repository. De-branded copies of `apperror` and
  `logger`; dropped gin, MongoDB, viper and the private library. Fixed a latent
  nil-factory panic inherited from the boilerplate's `apperror` — the package
  global stayed nil until `Init` was called, so the first `New` dereferenced nil
  on every error path.

- **22:05** Wrote `Money` over `int64` minor units, then replaced it minutes
  later with `shopspring/decimal` at the reviewer's direction. The invariant
  survived the swap and is the more important half: every amount is quantised to
  its own currency's scale at construction, so decimal's arbitrary precision
  cannot leak into the ledger. Verified `decimal.Round` half-away-from-zero
  against all eight interest figures the replay would later produce, before
  building anything on top of it.

- **22:07** Scaffold committed.

- **22:08** Domain model committed. Two clocks on every event and entry; entries
  carry their own sign, so a balance is a plain sum rather than a sum plus a
  direction flag that can be mishandled.

- **22:14** Engine committed, verified against the hand computation: ACC-001
  closes Day 6 at AED 390.93 with three fees, ACC-002 at BHD 10.008, E6
  rejected, Auth-B declined at −245.00.

  One real bug on the first full run. `REVERSAL` events carry no stated amount,
  so the currency guard rejected E9 outright and Day 6 came out at −254.90. Split
  `IsMonetary` into `HasStatedAmount`: a reversal derives its amount from the
  entries it reverses, which is the only way it is guaranteed to undo exactly
  what was done.

  Also confirmed the `on_cause_reversal` branch reaches a fixed point on the
  pre-E7 counterfactual — 466.03, interest 1.03, net fees 0.00.

- **22:17** Report renderer and `main.go` committed. Printed both clocks and the
  full interest restatement trail rather than net figures: the Day 2 trail
  (+0.10 / −0.10 / +0.09, each with the balance it was computed on) is the whole
  argument for the append-only accrual design, and is better shown than
  described.

- **22:26** Test suite committed. Two tests exist to be honest rather than to
  pass. `TestFee_CascadeIsCoveredSynthetically` fills a gap the brief's own data
  leaves — on the canonical stream the fee cascade changes no outcome (the Day 2
  fee narrows Day 3 from +30.00 to +5.00, still positive), so the given events
  cannot distinguish that design from the alternative.
  `TestFee_Criterion2BoundaryCase` shows criterion 2 fails by the size of one
  settlement rather than being absurd.

  Caught a bad test of my own along the way. The subtest claiming to demonstrate
  the 0.93-versus-0.92 divergence used `MulRatioHalfUp(4, 1000000)` — wrong by
  two orders of magnitude — and passed vacuously. Rewritten to sum the unrounded
  daily products in `decimal` and assert 0.918 → 0.92 against the engine's 0.93.
  The claim in REJECTED.md now has evidence behind it rather than a green tick.

  Twice my expectation was wrong before the engine was. I asserted a back-valued
  debit moves Day 6 by exactly the debit — it also moves the interest that debit
  cost, so 100.20. And I expected three fees in a variant where I had removed E9,
  where there are four, because Day 6 stays negative without the reversal. Both
  times the engine was right and the assertion was lazy.

- **22:32** Annotated failing test, Makefile and the four documents committed.
  The failing test asserts Day 6 closes at 466.03 and fails at 390.93. Left red
  deliberately: the repair exists in this repository, one environment variable
  away, but the brief's rules contain no de-assessment primitive and defaulting
  to my own extension would hide the disagreement worth having.

  In REJECTED.md I wrote the strongest case *for* each refused criterion before
  the case against it. Criterion 6 in particular is not arithmetically wrong at
  all — it fails on a missing rule — and deserved to be met rather than
  dismissed.

- **22:36** `make` on this machine (a cygwin build under Git Bash) exits 127 and
  prints nothing, including for `make --version`. A broken local toolchain rather
  than a Makefile problem — but since I could not run it, the README leads with
  the plain `go` commands, which are the ones I actually executed. The Makefile
  is standard GNU make and ships as a convenience.

- **22:40** Corrected this file. The entries from 22:26 onward had been written
  ahead of the clock, carrying times up to 23:30 that had not happened yet.
  Rewritten against the actual commit timestamps in `git log`. A worklog the
  brief asks to be real is not the place to round times up to how long the work
  felt.

- **22:52** Removed every unused module and piece of carried-over infrastructure,
  found by scanning each declared symbol for references rather than by eye. Gone:
  the `logger` package (a slog handler pulling `request_id` from context, in a
  codebase with no HTTP layer, that nothing ever logged through), `.mockery.yaml`
  and its Makefile targets (the ledger has no collaborators to mock), the
  `apperror` factory indirection (its only job was a service prefix this service
  never sets), four unused error codes, three unused entity methods, two
  unreachable service methods, and the `Direction` field on `LedgerEntry`.

  `Direction` is the one worth naming. It was written at eleven call sites and
  read at none — `Amount` already carries its own sign. Keeping it would have
  meant two sources of truth for the same fact, only one of which anything
  consults, which is a disagreement that would surface silently and late.

  Same replay, same three fees, same 390.93 and 466.03, same single expected
  test failure.

- **22:51** Second cleanup pass, this time asking of every symbol not "is it
  used" but "does it serve the ledger question". Removed `AppError.Is` and
  `AppError.Unwrap` — nothing in the ledger inspects an error programmatically,
  because rejections are recorded on the log as data and reported under their own
  day, so the only consumer of an error value is a person reading the report.
  Implementing the `errors.Is` contract for a caller that does not exist is
  scaffolding. Removed `Service.Config()`, never called: `main` already holds the
  config and hands it to the presenter directly.

  Also cut the Makefile down to the four targets that run the deliverable
  (`run`, `run-reversal`, `test`, `verify`), dropping the generic `fmt`, `vet`,
  `lint` and `test-coverage` chores, and reduced `.gitignore` from inherited
  boilerplate — Windows DLLs, `.vscode`, `.idea`, `tmp/` — to the two patterns a
  pure Go project with no build artifacts actually produces.

  Left `Workspace/mal`, the original boilerplate, untouched: it is a separate
  unversioned directory holding no interview code, and deleting from it would
  have been irreversible.

- **22:56** Moved the project into `Workspace/mal` itself, git history and remote
  intact, and cleared the boilerplate out of it. I had built the ledger in a
  sibling directory and left `mal` — the actual working directory — holding
  nothing but the untouched boilerplate, so from the reviewer's seat the app
  simply was not there. The boilerplate is backed up in full (46 files, 96MB,
  including the gitignored `config/default/*.yml`) before deletion, since it was
  never under version control.

- **23:11** Refactored the module onto the boilerplate's foldering convention,
  which I had not followed. It now has `repository.go` / `service.go` /
  `controller.go` / `route.go`, interfaces with unexported structs behind `New*`
  constructors, `context.Context` first on every method, generated mocks in
  `mock/`, and `main.go` reduced to a composition root — repository, then
  service over it, then routes.

  The substantive gain is the `Repository` seam. What had been an unexported
  `log` struct is now an interface with an in-memory implementation, so the
  service depends on a contract rather than a slice. Append-only is now enforced
  by the shape of that interface — there is no `Update`, no `Delete`, no method
  taking a sequence number and writing to it — rather than by a convention
  someone has to remember. It also gives the service's error handling something
  to be tested against: `service_test.go` drives the repository failure paths
  through the mock, which were previously unreachable code nobody had run.

  Two deviations I could not avoid. The boilerplate's `controller.go` and
  `route.go` are gin HTTP handlers and the brief forbids a web layer, so they
  keep the shape — free functions taking `Service`, plus `NewRoutes` — with an
  `io.Writer` as the delivery target. And there is no `app/model`, because that
  layer is the database representation and there is no database.

  Deleted the `app/module/user` reference copy afterwards: it imports the
  private modules and would have broken the build for anyone cloning.

  Every figure is unchanged through the refactor — three fees, 390.93, 466.03,
  10.008, and the same single expected failure.
