package server

import (
	"context"
	"github.com/operator-framework/operator-registry/pkg/api"
)

type HealthServer struct {
}

var _ api.HealthServer= &HealthServer{}

func NewHealthServer() *HealthServer {
	return &HealthServer{}
}

func (s *HealthServer) Check(ctx context.Context, req *api.HealthCheckRequest) (*api.HealthCheckResponse, error) {
	return &api.HealthCheckResponse{Status: api.HealthCheckResponse_SERVING}, nil
}
