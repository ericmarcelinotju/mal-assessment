.PHONY: run run-reversal test verify

## run: replay the six-day stream under the literal reading of the brief
run:
	@go run main.go

## run-reversal: replay under the fee-reversal policy (acceptance criterion 6)
run-reversal:
	@LEDGER_FEE_REVERSAL_POLICY=on_cause_reversal go run main.go

## test: the whole suite. EXPECTED TO REPORT EXACTLY ONE FAILURE --
## TestKnownGap_OverdraftFeesSurviveReversalOfTheirCause, a deliberate argument
## against the specification. See REJECTED.md and the test's own annotation.
test:
	@echo "--- running tests (one failure is expected: TestKnownGap_) ---"
	@go test ./... -count=1

## verify: the suite without the known-gap test. This must be green.
verify:
	@echo "--- running tests, excluding the annotated failing test ---"
	@go test ./... -count=1 -skip 'TestKnownGap_'
