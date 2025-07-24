package service

import (
	"context"

	"github.com/gin-gonic/gin"
)

// ServiceHandlerInterface defines the interface for service management handlers
type ServiceHandlerInterface interface {
	// GetServiceStatus gets the status of a specific service
	GetServiceStatus(c *gin.Context)
	
	// GetAllServiceStatuses gets the status of all services
	GetAllServiceStatuses(c *gin.Context)
	
	// StartService starts a specific service
	StartService(c *gin.Context)
	
	// StopService stops a specific service
	StopService(c *gin.Context)
	
	// RestartService restarts a specific service
	RestartService(c *gin.Context)
	
	// GetServiceHealth gets the health status of a specific service
	GetServiceHealth(c *gin.Context)
}

// ServiceHandlerInterfaceWithContext defines the interface for service management handlers with context
type ServiceHandlerInterfaceWithContext interface {
	// GetServiceStatus gets the status of a specific service
	GetServiceStatusHandler(ctx context.Context, c *gin.Context)
	
	// GetAllServiceStatuses gets the status of all services
	GetAllServiceStatusesHandler(ctx context.Context, c *gin.Context)
	
	// StartService starts a specific service
	StartServiceHandler(ctx context.Context, c *gin.Context)
	
	// StopService stops a specific service
	StopServiceHandler(ctx context.Context, c *gin.Context)
	
	// RestartService restarts a specific service
	RestartServiceHandler(ctx context.Context, c *gin.Context)
	
	// GetServiceHealth gets the health status of a specific service
	GetServiceHealthHandler(ctx context.Context, c *gin.Context)
}