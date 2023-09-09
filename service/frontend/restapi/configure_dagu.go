// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"crypto/tls"
	pkgmiddleware "github.com/dagu-dev/dagu/service/frontend/middleware"
	"net/http"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/dagu-dev/dagu/service/frontend/restapi/operations"
)

//go:generate swagger generate server --target ../../frontend --name Dagu --spec ../../../swagger.yaml --principal interface{} --exclude-main

func configureFlags(api *operations.DaguAPI) {
	// handlers.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{ ... }
}

func configureAPI(api *operations.DaguAPI) http.Handler {
	// configure the handlers here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// handlers.Logger = log.Printf

	api.UseSwaggerUI()
	// To continue using redoc as your UI, uncomment the following line
	// handlers.UseRedoc()

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	if api.ListDagsHandler == nil {
		api.ListDagsHandler = operations.ListDagsHandlerFunc(func(params operations.ListDagsParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.ListDags has not yet been implemented")
		})
	}

	api.PreServerShutdown = func() {}

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix".
func configureServer(s *http.Server, scheme, addr string) {
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation.
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics.
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return pkgmiddleware.SetupGlobalMiddleware(handler)
}
