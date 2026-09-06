package entity_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// aed and bhd build expected amounts in the test's own words, so an assertion
// reads as the figure it is checking rather than as a constructor call.
func aed(s string) entity.Money { return entity.MustParseMoney(s, entity.AED) }
func bhd(s string) entity.Money { return entity.MustParseMoney(s, entity.BHD) }

// assertMoney compares by value and prints both sides in the currency's own
// scale on failure. Comparing the rendered strings instead would pass a BHD
// amount off against an AED one whenever the digits happened to match.
func assertMoney(t *testing.T, want, got entity.Money, format string, args ...any) {
	t.Helper()
	note := ""
	if format != "" {
		note = " -- " + fmt.Sprintf(format, args...)
	}
	assert.True(t, want.Equal(got),
		"want %s, got %s%s", want.Display(), got.Display(), note)
}
