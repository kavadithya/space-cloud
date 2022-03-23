package apis

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

// APIHandler is responsible to call the appropriate module to process an incoming API request
type APIHandler struct {
	App    string                 `json:"app"`
	Op     string                 `json:"op"`
	Params map[string]interface{} `json:"params"`

	Path    string            `json:"path"`
	Indexes map[string]string `json:"indexes"`

	// For internal use
	logger  *zap.Logger
	handler http.HandlerFunc
}

// CaddyModule returns the Caddy module information.
func (APIHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.sc_api_handler",
		New: func() caddy.Module { return new(APIHandler) },
	}
}

// Provision runs as a prehook to the handler operation
func (h *APIHandler) Provision(ctx caddy.Context) error {
	// Store the logger for later use
	h.logger = ctx.Logger(h)

	// Load the app this handler is made for
	appTemp, err := ctx.App(h.App)
	if err != nil {
		h.logger.Error("Unable to load app to serve SpaceCloud APIs", zap.String("app", h.App))
		return err
	}

	// Store the app for future use. We don't need to check success of type assertion since its already done
	// in the root handler
	h.handler, err = appTemp.(App).GetHandler(h.Op)
	if err != nil {
		h.logger.Error("Unable to load handler from app to serve SpaceCloud APIs", zap.String("app", h.App), zap.String("op", h.Op))
		return err
	}
	return nil
}

// ServeHTTP handles the http request
func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	h.handler(w, r)
	return nil
}

// Interface guard
var _ caddy.Provisioner = (*RootAPIHandler)(nil)
var _ caddyhttp.MiddlewareHandler = (*RootAPIHandler)(nil)
