package service

import (
	"context"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/data-preservation-programs/singularity/handler/handlererror"
	"github.com/data-preservation-programs/singularity/service/manager"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler provides service management API endpoints
type Handler struct {
	serviceManager *manager.ServiceManager
}

// ServiceStatusResponse represents a service status response
type ServiceStatusResponse struct {
	Type         string                     `json:"type"`
	State        string                     `json:"state"`
	StartedAt    *string                    `json:"startedAt,omitempty"`
	LastError    string                     `json:"lastError,omitempty"`
	HealthStatus string                     `json:"healthStatus"`
	Config       map[string]interface{}     `json:"config"`
}

// ServiceStartRequest represents a service start request
type ServiceStartRequest struct {
	Config map[string]interface{} `json:"config,omitempty"`
}

// ServiceStopRequest represents a service stop request
type ServiceStopRequest struct {
	Force bool `json:"force,omitempty"`
}

// ServiceConfigRequest represents a service configuration update request
type ServiceConfigRequest struct {
	Config    map[string]interface{} `json:"config"`
	AutoStart bool                   `json:"autoStart"`
	Enabled   bool                   `json:"enabled"`
}

// NewHandler creates a new service handler
func NewHandler(serviceManager *manager.ServiceManager) *Handler {
	return &Handler{
		serviceManager: serviceManager,
	}
}

// GetServiceStatus gets the status of a specific service
// @Summary Get service status
// @Description Get the current status of a specific service
// @Tags Services
// @Param service path string true "Service type (api, content_provider, worker_manager)"
// @Success 200 {object} ServiceStatusResponse
// @Failure 400 {object} handlererror.APIError
// @Failure 404 {object} handlererror.APIError
// @Failure 500 {object} handlererror.APIError
// @Router /services/{service}/status [get]
func (h *Handler) GetServiceStatus(c *gin.Context) {
	serviceTypeStr := c.Param("service")
	serviceType := manager.ServiceType(serviceTypeStr)

	status, err := h.serviceManager.GetServiceStatus(serviceType)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			handlererror.NewAPIError(http.StatusNotFound, "service not found").Abort(c)
			return
		}
		handlererror.NewAPIError(http.StatusInternalServerError, err.Error()).Abort(c)
		return
	}

	response := ServiceStatusResponse{
		Type:         string(status.Type),
		State:        string(status.State),
		LastError:    status.LastError,
		HealthStatus: status.HealthStatus,
		Config:       status.Config.Config,
	}

	if status.StartedAt != nil {
		startedAt := status.StartedAt.Format("2006-01-02T15:04:05Z")
		response.StartedAt = &startedAt
	}

	c.JSON(http.StatusOK, response)
}

// GetAllServiceStatuses gets the status of all services
// @Summary Get all service statuses
// @Description Get the current status of all services
// @Tags Services
// @Success 200 {object} map[string]ServiceStatusResponse
// @Failure 500 {object} handlererror.APIError
// @Router /services/status [get]
func (h *Handler) GetAllServiceStatuses(c *gin.Context) {
	statuses := h.serviceManager.GetAllServiceStatuses()
	
	response := make(map[string]ServiceStatusResponse)
	for serviceType, status := range statuses {
		serviceResponse := ServiceStatusResponse{
			Type:         string(status.Type),
			State:        string(status.State),
			LastError:    status.LastError,
			HealthStatus: status.HealthStatus,
			Config:       status.Config.Config,
		}

		if status.StartedAt != nil {
			startedAt := status.StartedAt.Format("2006-01-02T15:04:05Z")
			serviceResponse.StartedAt = &startedAt
		}

		response[string(serviceType)] = serviceResponse
	}

	c.JSON(http.StatusOK, response)
}

// StartService starts a specific service
// @Summary Start service
// @Description Start a specific service with optional configuration
// @Tags Services
// @Param service path string true "Service type (api, content_provider, worker_manager)"
// @Param request body ServiceStartRequest false "Service start configuration"
// @Success 200 {object} ServiceStatusResponse
// @Failure 400 {object} handlererror.APIError
// @Failure 404 {object} handlererror.APIError
// @Failure 409 {object} handlererror.APIError
// @Failure 500 {object} handlererror.APIError
// @Router /services/{service}/start [post]
func (h *Handler) StartService(c *gin.Context) {
	serviceTypeStr := c.Param("service")
	serviceType := manager.ServiceType(serviceTypeStr)

	var request ServiceStartRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		handlererror.NewAPIError(http.StatusBadRequest, "invalid request body").Abort(c)
		return
	}

	// Start the service
	err := h.serviceManager.StartService(serviceType)
	if err != nil {
		if errors.Is(err, errors.New("service not found")) {
			handlererror.NewAPIError(http.StatusNotFound, "service not found").Abort(c)
			return
		}
		if errors.Is(err, errors.New("service is already running")) {
			handlererror.NewAPIError(http.StatusConflict, "service is already running").Abort(c)
			return
		}
		handlererror.NewAPIError(http.StatusInternalServerError, err.Error()).Abort(c)
		return
	}

	// Return updated status
	status, err := h.serviceManager.GetServiceStatus(serviceType)
	if err != nil {
		handlererror.NewAPIError(http.StatusInternalServerError, err.Error()).Abort(c)
		return
	}

	response := ServiceStatusResponse{
		Type:         string(status.Type),
		State:        string(status.State),
		LastError:    status.LastError,
		HealthStatus: status.HealthStatus,
		Config:       status.Config.Config,
	}

	if status.StartedAt != nil {
		startedAt := status.StartedAt.Format("2006-01-02T15:04:05Z")
		response.StartedAt = &startedAt
	}

	c.JSON(http.StatusOK, response)
}

// StopService stops a specific service
// @Summary Stop service
// @Description Stop a specific service
// @Tags Services
// @Param service path string true "Service type (api, content_provider, worker_manager)"
// @Param request body ServiceStopRequest false "Service stop configuration"
// @Success 200 {object} ServiceStatusResponse
// @Failure 400 {object} handlererror.APIError
// @Failure 404 {object} handlererror.APIError
// @Failure 409 {object} handlererror.APIError
// @Failure 500 {object} handlererror.APIError
// @Router /services/{service}/stop [post]
func (h *Handler) StopService(c *gin.Context) {
	serviceTypeStr := c.Param("service")
	serviceType := manager.ServiceType(serviceTypeStr)

	var request ServiceStopRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		handlererror.NewAPIError(http.StatusBadRequest, "invalid request body").Abort(c)
		return
	}

	// Stop the service
	err := h.serviceManager.StopService(serviceType)
	if err != nil {
		if errors.Is(err, errors.New("service not found")) {
			handlererror.NewAPIError(http.StatusNotFound, "service not found").Abort(c)
			return
		}
		if errors.Is(err, errors.New("service is not running")) {
			handlererror.NewAPIError(http.StatusConflict, "service is not running").Abort(c)
			return
		}
		handlererror.NewAPIError(http.StatusInternalServerError, err.Error()).Abort(c)
		return
	}

	// Return updated status
	status, err := h.serviceManager.GetServiceStatus(serviceType)
	if err != nil {
		handlererror.NewAPIError(http.StatusInternalServerError, err.Error()).Abort(c)
		return
	}

	response := ServiceStatusResponse{
		Type:         string(status.Type),
		State:        string(status.State),
		LastError:    status.LastError,
		HealthStatus: status.HealthStatus,
		Config:       status.Config.Config,
	}

	if status.StartedAt != nil {
		startedAt := status.StartedAt.Format("2006-01-02T15:04:05Z")
		response.StartedAt = &startedAt
	}

	c.JSON(http.StatusOK, response)
}

// RestartService restarts a specific service
// @Summary Restart service
// @Description Restart a specific service (stop then start)
// @Tags Services
// @Param service path string true "Service type (api, content_provider, worker_manager)"
// @Success 200 {object} ServiceStatusResponse
// @Failure 400 {object} handlererror.APIError
// @Failure 404 {object} handlererror.APIError
// @Failure 500 {object} handlererror.APIError
// @Router /services/{service}/restart [post]
func (h *Handler) RestartService(c *gin.Context) {
	serviceTypeStr := c.Param("service")
	serviceType := manager.ServiceType(serviceTypeStr)

	// First check if service exists
	_, err := h.serviceManager.GetServiceStatus(serviceType)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			handlererror.NewAPIError(http.StatusNotFound, "service not found").Abort(c)
			return
		}
		handlererror.NewAPIError(http.StatusInternalServerError, err.Error()).Abort(c)
		return
	}

	// Stop the service if it's running
	err = h.serviceManager.StopService(serviceType)
	if err != nil && !errors.Is(err, errors.New("service is not running")) {
		handlererror.NewAPIError(http.StatusInternalServerError, err.Error()).Abort(c)
		return
	}

	// Start the service
	err = h.serviceManager.StartService(serviceType)
	if err != nil {
		handlererror.NewAPIError(http.StatusInternalServerError, err.Error()).Abort(c)
		return
	}

	// Return updated status
	status, err := h.serviceManager.GetServiceStatus(serviceType)
	if err != nil {
		handlererror.NewAPIError(http.StatusInternalServerError, err.Error()).Abort(c)
		return
	}

	response := ServiceStatusResponse{
		Type:         string(status.Type),
		State:        string(status.State),
		LastError:    status.LastError,
		HealthStatus: status.HealthStatus,
		Config:       status.Config.Config,
	}

	if status.StartedAt != nil {
		startedAt := status.StartedAt.Format("2006-01-02T15:04:05Z")
		response.StartedAt = &startedAt
	}

	c.JSON(http.StatusOK, response)
}

// GetServiceHealth gets the health status of a specific service
// @Summary Get service health
// @Description Get the health check status of a specific service
// @Tags Services
// @Param service path string true "Service type (api, content_provider, worker_manager)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} handlererror.APIError
// @Failure 404 {object} handlererror.APIError
// @Failure 500 {object} handlererror.APIError
// @Router /services/{service}/health [get]
func (h *Handler) GetServiceHealth(c *gin.Context) {
	serviceTypeStr := c.Param("service")
	serviceType := manager.ServiceType(serviceTypeStr)

	status, err := h.serviceManager.GetServiceStatus(serviceType)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			handlererror.NewAPIError(http.StatusNotFound, "service not found").Abort(c)
			return
		}
		handlererror.NewAPIError(http.StatusInternalServerError, err.Error()).Abort(c)
		return
	}

	health := map[string]interface{}{
		"service":      string(status.Type),
		"status":       string(status.State),
		"health":       status.HealthStatus,
		"lastError":    status.LastError,
		"startedAt":    status.StartedAt,
	}

	// Determine HTTP status based on health
	var httpStatus int
	switch status.HealthStatus {
	case "healthy":
		httpStatus = http.StatusOK
	case "unhealthy":
		httpStatus = http.StatusServiceUnavailable
	default:
		httpStatus = http.StatusOK
	}

	c.JSON(httpStatus, health)
}

// Default is the default handler instance
var Default = &Handler{}

// GlobalServiceManager is a global instance for service management
var GlobalServiceManager *manager.ServiceManager

// SetServiceManager sets the service manager for the default handler
func SetServiceManager(serviceManager *manager.ServiceManager) {
	Default.serviceManager = serviceManager
	GlobalServiceManager = serviceManager
}

// GetServiceStatus is a wrapper for the default handler
func GetServiceStatus(ctx context.Context, c *gin.Context) {
	Default.GetServiceStatus(c)
}

// GetAllServiceStatuses is a wrapper for the default handler
func GetAllServiceStatuses(ctx context.Context, c *gin.Context) {
	Default.GetAllServiceStatuses(c)
}

// StartService is a wrapper for the default handler
func StartService(ctx context.Context, c *gin.Context) {
	Default.StartService(c)
}

// StopService is a wrapper for the default handler
func StopService(ctx context.Context, c *gin.Context) {
	Default.StopService(c)
}

// RestartService is a wrapper for the default handler
func RestartService(ctx context.Context, c *gin.Context) {
	Default.RestartService(c)
}

// GetServiceHealth is a wrapper for the default handler
func GetServiceHealth(ctx context.Context, c *gin.Context) {
	Default.GetServiceHealth(c)
}