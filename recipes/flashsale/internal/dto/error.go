package dto

// ErrResp is the uniform error shape for every non-2xx response from the
// flashsale recipe.
type ErrResp struct {
	Error string `json:"error"`
}
