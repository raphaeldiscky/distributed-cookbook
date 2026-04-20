package metrics

// Outcome is the value used for the `outcome` label on
// cookbook_flashsale_checkout_attempts_total. Keeping these as typed
// constants (rather than scattered string literals across the service
// and main.go) prevents silent "ghost series" bugs when a typo splits
// one outcome into two different Prometheus series.
type Outcome string

// The four possible outcomes of a /checkout attempt.
const (
	OutcomeOK         Outcome = "ok"
	OutcomeOutOfStock Outcome = "out_of_stock"
	OutcomeNotFound   Outcome = "not_found"
	OutcomeError      Outcome = "error"
)

// AllOutcomes is the canonical list — used by startup code to pre-touch
// every (kind, outcome) series so dashboards render from second one.
//
//nolint:gochecknoglobals // canonical enumeration, read-only
var AllOutcomes = []Outcome{
	OutcomeOK,
	OutcomeOutOfStock,
	OutcomeNotFound,
	OutcomeError,
}
