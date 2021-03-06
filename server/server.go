// Package server provides a server implementation to connect network transport
// protocols and service business logic by defining server endpoints.
package server

import (
	"context"
	"net/http"
	"sync"

	"github.com/giantswarm/microerror"
	microserver "github.com/giantswarm/microkit/server"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/viper"

	"github.com/giantswarm/ingress-operator/server/endpoint"
	"github.com/giantswarm/ingress-operator/server/middleware"
	"github.com/giantswarm/ingress-operator/service"
)

// Config represents the configuration used to create a new server object.
type Config struct {
	Logger  micrologger.Logger
	Service *service.Service
	Viper   *viper.Viper

	ProjectName string
}

type Server struct {
	// Dependencies.
	logger micrologger.Logger

	// Internals.
	bootOnce     sync.Once
	config       microserver.Config
	shutdownOnce sync.Once
}

// New creates a new configured server object.
func New(config Config) (*Server, error) {
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", config)
	}
	if config.Service == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Service must not be empty", config)
	}
	if config.Viper == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Viper must not be empty", config)
	}

	if config.ProjectName == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.ProjectName must not be empty", config)
	}

	var err error

	var middlewareCollection *middleware.Middleware
	{
		middlewareConfig := middleware.DefaultConfig()
		middlewareConfig.Logger = config.Logger
		middlewareConfig.Service = config.Service
		middlewareCollection, err = middleware.New(middlewareConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var endpointCollection *endpoint.Endpoint
	{
		endpointConfig := endpoint.DefaultConfig()
		endpointConfig.Logger = config.Logger
		endpointConfig.Middleware = middlewareCollection
		endpointConfig.Service = config.Service
		endpointCollection, err = endpoint.New(endpointConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	s := &Server{
		// Dependencies.
		logger: config.Logger,

		// Internals.
		bootOnce: sync.Once{},
		config: microserver.Config{
			Logger:      config.Logger,
			ServiceName: config.ProjectName,
			Viper:       config.Viper,

			Endpoints: []microserver.Endpoint{
				endpointCollection.Healthz,
				endpointCollection.Version,
			},
			ErrorEncoder: errorEncoder,
		},
		shutdownOnce: sync.Once{},
	}

	return s, nil
}

func (s *Server) Boot() {
	s.bootOnce.Do(func() {
		// Here goes your custom boot logic for your server/endpoint/middleware, if
		// any.
	})
}

func (s *Server) Config() microserver.Config {
	return s.config
}

func (s *Server) Shutdown() {
	s.shutdownOnce.Do(func() {
		// Here goes your custom shutdown logic for your server/endpoint/middleware,
		// if any.
	})
}

func errorEncoder(ctx context.Context, err error, w http.ResponseWriter) {
	rErr := err.(microserver.ResponseError)
	rErr.SetCode(microserver.CodeInternalError)
	rErr.SetMessage("An unexpected error occurred. Sorry for the inconvenience.")
	w.WriteHeader(http.StatusInternalServerError)
}
