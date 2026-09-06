.PHONY: run run-reversal test verify test-coverage fmt vet lint clean-mock generate-mock

## run: replay the six-day stream under the literal reading of the brief
run:
	@go run main.go

## run-reversal: replay under the fee-reversal policy (acceptance criterion 6)
run-reversal:
	@LEDGER_FEE_REVERSAL_POLICY=on_cause_reversal go run main.go

## test: the whole suite. EXPECTED TO REPORT EXACTLY ONE FAILURE --
## TestKnownGap_OverdraftFeesSurviveReversalOfTheirCause, which is a deliberate
## argument against the specification. See REJECTED.md and the test's own
## annotation. Use `make verify` for a green run.
test:
	@echo "--- running tests (one failure is expected: TestKnownGap_) ---"
	@go test ./... -count=1

## verify: the suite without the known-gap test. This must be green.
verify:
	@echo "--- running tests, excluding the annotated failing test ---"
	@go test ./... -count=1 -skip 'TestKnownGap_'

test-coverage:
	@mkdir -p report
	@go test ./... -count=1 -skip 'TestKnownGap_' \
		-coverprofile="report/coverage.out" -coverpkg="./app/...,./config/..."
	@go tool cover -html="report/coverage.out" -o "report/coverage.html"
	@go tool cover -func="report/coverage.out" | tail -1

fmt:
	@gofmt -l -w .

vet:
	@go vet ./...

lint: fmt vet

## The ledger core has no collaborators to mock -- it is a pure function of the
## event stream, with no repository, transport or clock. The mockery config is
## kept for consistency with the wider codebase, but the suite does not need it
## and `make test` does not run it.
clean-mock:
	@echo "--- cleaning mocks ---"
ifeq ($(OS),Windows_NT)
	@powershell -Command "Get-ChildItem -Recurse -Filter '*_mock.go' | Remove-Item -Force"
else
	@find . -name '*_mock.go' -delete
endif

generate-mock: clean-mock
	@echo "--- generating mocks by mockery ---"
	@mockery
