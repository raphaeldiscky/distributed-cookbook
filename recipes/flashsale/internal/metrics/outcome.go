package metrics

// Outcome is the value used for the `outcome` label on
// cookbook_flashsale_checkout_attempts_total. Keeping these as typed
// constants (rather than scattered string literals across the service
// and main.go) prevents silent "ghost series" bugs when a typo splits
// one outcome into two different Prometheus series.
type Outcome string

// The possible outcomes of a /checkout attempt.
//
// OutcomeRetryExhausted is distinct from OutcomeError on purpose: an
// optimistic-locking adapter that runs out of retries has not hit a fault, it
// has lost a race too many times. Folding that into `error` would hide a
// contention signal inside a fault count.
const (
	OutcomeOK             Outcome = "ok"
	OutcomeOutOfStock     Outcome = "out_of_stock"
	OutcomeNotFound       Outcome = "not_found"
	OutcomeError          Outcome = "error"
	OutcomeRetryExhausted Outcome = "retry_exhausted"
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
	OutcomeRetryExhausted,
}
