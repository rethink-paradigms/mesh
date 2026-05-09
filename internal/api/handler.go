package api

// Handler holds the router configuration and provides HTTP handler methods.
type Handler struct {
	cfg RouterConfig
}

// NewHandler creates a new Handler with the given configuration.
func NewHandler(cfg RouterConfig) *Handler {
	return &Handler{cfg: cfg}
}
