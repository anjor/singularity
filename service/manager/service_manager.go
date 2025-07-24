package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/data-preservation-programs/singularity/api"
	"github.com/data-preservation-programs/singularity/database"
	"github.com/data-preservation-programs/singularity/model"
	"github.com/data-preservation-programs/singularity/service/contentprovider"
	"github.com/ipfs/go-log/v2"
	"gorm.io/gorm"
)

var logger = log.Logger("service-manager")

// ServiceType represents the type of service that can be managed
type ServiceType string

const (
	ServiceTypeAPI             ServiceType = "api"
	ServiceTypeContentProvider ServiceType = "content_provider"
	ServiceTypeWorkerManager   ServiceType = "worker_manager"
)

// ServiceState represents the current state of a service
type ServiceState string

const (
	ServiceStateStopped  ServiceState = "stopped"
	ServiceStateStarting ServiceState = "starting"
	ServiceStateRunning  ServiceState = "running"
	ServiceStateStopping ServiceState = "stopping"
	ServiceStateError    ServiceState = "error"
)

// ServiceConfig holds the configuration for a service
type ServiceConfig struct {
	Type      ServiceType            `json:"type"`
	Config    map[string]interface{} `json:"config"`
	AutoStart bool                   `json:"autoStart"`
	Enabled   bool                   `json:"enabled"`
}

// ServiceStatus represents the current status of a service
type ServiceStatus struct {
	Type         ServiceType   `json:"type"`
	State        ServiceState  `json:"state"`
	StartedAt    *time.Time    `json:"startedAt,omitempty"`
	LastError    string        `json:"lastError,omitempty"`
	HealthStatus string        `json:"healthStatus"`
	Config       ServiceConfig `json:"config"`
}

// Service represents a managed service
type Service struct {
	Type      ServiceType
	State     ServiceState
	StartedAt *time.Time
	LastError string
	Config    ServiceConfig
	cancelCtx context.CancelFunc
	done      chan struct{}
	mutex     sync.RWMutex
}

// ServiceManager manages the lifecycle of various services
type ServiceManager struct {
	db         *gorm.DB
	services   map[ServiceType]*Service
	repository *ServiceRepository
	mutex      sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// ServiceManagerConfig holds configuration for the service manager
type ServiceManagerConfig struct {
	APIConfig             APIServiceConfig             `json:"api"`
	ContentProviderConfig ContentProviderServiceConfig `json:"contentProvider"`
	WorkerManagerConfig   WorkerManagerServiceConfig   `json:"workerManager"`
}

// APIServiceConfig holds configuration for the API service
type APIServiceConfig struct {
	Bind    string `json:"bind"`
	Enabled bool   `json:"enabled"`
}

// ContentProviderServiceConfig holds configuration for the content provider service
type ContentProviderServiceConfig struct {
	HTTPBind                  string   `json:"httpBind"`
	EnableHTTPPiece           bool     `json:"enableHttpPiece"`
	EnableHTTPPieceMetadata   bool     `json:"enableHttpPieceMetadata"`
	EnableBitswap             bool     `json:"enableBitswap"`
	LibP2PIdentityKey         string   `json:"libp2pIdentityKey"`
	LibP2PListenMultiAddrs    []string `json:"libp2pListenMultiAddrs"`
	Enabled                   bool     `json:"enabled"`
}

// WorkerManagerServiceConfig holds configuration for the worker manager service
type WorkerManagerServiceConfig struct {
	MinWorkers         int           `json:"minWorkers"`
	MaxWorkers         int           `json:"maxWorkers"`
	ScaleUpThreshold   int           `json:"scaleUpThreshold"`
	ScaleDownThreshold int           `json:"scaleDownThreshold"`
	CheckInterval      time.Duration `json:"checkInterval"`
	WorkerIdleTimeout  time.Duration `json:"workerIdleTimeout"`
	AutoScaling        bool          `json:"autoScaling"`
	ScanWorkerRatio    float64       `json:"scanWorkerRatio"`
	PackWorkerRatio    float64       `json:"packWorkerRatio"`
	DagGenWorkerRatio  float64       `json:"dagGenWorkerRatio"`
	Enabled            bool          `json:"enabled"`
}

// DefaultServiceManagerConfig returns default configuration for the service manager
func DefaultServiceManagerConfig() ServiceManagerConfig {
	return ServiceManagerConfig{
		APIConfig: APIServiceConfig{
			Bind:    ":9090",
			Enabled: true,
		},
		ContentProviderConfig: ContentProviderServiceConfig{
			HTTPBind:                "127.0.0.1:7777",
			EnableHTTPPiece:         true,
			EnableHTTPPieceMetadata: true,
			EnableBitswap:           false,
			LibP2PIdentityKey:       "",
			LibP2PListenMultiAddrs:  []string{},
			Enabled:                 true,
		},
		WorkerManagerConfig: WorkerManagerServiceConfig{
			MinWorkers:         1,
			MaxWorkers:         5,
			ScaleUpThreshold:   5,
			ScaleDownThreshold: 2,
			CheckInterval:      30 * time.Second,
			WorkerIdleTimeout:  5 * time.Minute,
			AutoScaling:        true,
			ScanWorkerRatio:    0.3,
			PackWorkerRatio:    0.5,
			DagGenWorkerRatio:  0.2,
			Enabled:            true,
		},
	}
}

// NewServiceManager creates a new service manager
func NewServiceManager(db *gorm.DB, config ServiceManagerConfig) *ServiceManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	sm := &ServiceManager{
		db:         db,
		services:   make(map[ServiceType]*Service),
		repository: NewServiceRepository(db),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Initialize services
	sm.initializeServices(config)
	
	return sm
}

// initializeServices initializes all services with their configurations
func (sm *ServiceManager) initializeServices(config ServiceManagerConfig) {
	// Initialize API service
	sm.services[ServiceTypeAPI] = &Service{
		Type:  ServiceTypeAPI,
		State: ServiceStateStopped,
		Config: ServiceConfig{
			Type: ServiceTypeAPI,
			Config: map[string]interface{}{
				"bind": config.APIConfig.Bind,
			},
			AutoStart: true,
			Enabled:   config.APIConfig.Enabled,
		},
		done: make(chan struct{}),
	}

	// Initialize Content Provider service
	sm.services[ServiceTypeContentProvider] = &Service{
		Type:  ServiceTypeContentProvider,
		State: ServiceStateStopped,
		Config: ServiceConfig{
			Type: ServiceTypeContentProvider,
			Config: map[string]interface{}{
				"httpBind":                config.ContentProviderConfig.HTTPBind,
				"enableHttpPiece":         config.ContentProviderConfig.EnableHTTPPiece,
				"enableHttpPieceMetadata": config.ContentProviderConfig.EnableHTTPPieceMetadata,
				"enableBitswap":           config.ContentProviderConfig.EnableBitswap,
				"libp2pIdentityKey":       config.ContentProviderConfig.LibP2PIdentityKey,
				"libp2pListenMultiAddrs":  config.ContentProviderConfig.LibP2PListenMultiAddrs,
			},
			AutoStart: true,
			Enabled:   config.ContentProviderConfig.Enabled,
		},
		done: make(chan struct{}),
	}

	// Initialize Worker Manager service (if needed as a separate service)
	sm.services[ServiceTypeWorkerManager] = &Service{
		Type:  ServiceTypeWorkerManager,
		State: ServiceStateStopped,
		Config: ServiceConfig{
			Type: ServiceTypeWorkerManager,
			Config: map[string]interface{}{
				"minWorkers":         config.WorkerManagerConfig.MinWorkers,
				"maxWorkers":         config.WorkerManagerConfig.MaxWorkers,
				"scaleUpThreshold":   config.WorkerManagerConfig.ScaleUpThreshold,
				"scaleDownThreshold": config.WorkerManagerConfig.ScaleDownThreshold,
				"checkInterval":      config.WorkerManagerConfig.CheckInterval,
				"workerIdleTimeout":  config.WorkerManagerConfig.WorkerIdleTimeout,
				"autoScaling":        config.WorkerManagerConfig.AutoScaling,
				"scanWorkerRatio":    config.WorkerManagerConfig.ScanWorkerRatio,
				"packWorkerRatio":    config.WorkerManagerConfig.PackWorkerRatio,
				"dagGenWorkerRatio":  config.WorkerManagerConfig.DagGenWorkerRatio,
			},
			AutoStart: false, // Usually managed by onboard command
			Enabled:   config.WorkerManagerConfig.Enabled,
		},
		done: make(chan struct{}),
	}
}

// StartService starts a specific service
func (sm *ServiceManager) StartService(serviceType ServiceType) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	service, exists := sm.services[serviceType]
	if !exists {
		return errors.Errorf("service type %s not found", serviceType)
	}

	if !service.Config.Enabled {
		return errors.Errorf("service %s is disabled", serviceType)
	}

	service.mutex.Lock()
	defer service.mutex.Unlock()

	if service.State == ServiceStateRunning || service.State == ServiceStateStarting {
		return errors.Errorf("service %s is already running or starting", serviceType)
	}

	logger.Infof("Starting service: %s", serviceType)
	service.State = ServiceStateStarting
	service.LastError = ""

	// Handle service dependencies
	if err := sm.checkAndStartDependencies(serviceType); err != nil {
		service.State = ServiceStateError
		service.LastError = err.Error()
		return errors.Wrapf(err, "failed to start dependencies for service %s", serviceType)
	}

	// Create service context
	ctx, cancel := context.WithCancel(sm.ctx)
	service.cancelCtx = cancel

	// Start the appropriate service
	switch serviceType {
	case ServiceTypeAPI:
		go sm.runAPIService(ctx, service)
	case ServiceTypeContentProvider:
		go sm.runContentProviderService(ctx, service)
	case ServiceTypeWorkerManager:
		go sm.runWorkerManagerService(ctx, service)
	default:
		service.State = ServiceStateError
		service.LastError = "unknown service type"
		cancel()
		return errors.Errorf("unknown service type: %s", serviceType)
	}

	// Update service status
	now := time.Now()
	service.StartedAt = &now
	service.State = ServiceStateRunning

	// Update database status
	if err := sm.updateServiceStatusInDB(service); err != nil {
		logger.Errorf("Failed to update service status in database: %v", err)
	}

	logger.Infof("Service %s started successfully", serviceType)
	return nil
}

// StopService stops a specific service
func (sm *ServiceManager) StopService(serviceType ServiceType) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	service, exists := sm.services[serviceType]
	if !exists {
		return errors.Errorf("service type %s not found", serviceType)
	}

	service.mutex.Lock()
	defer service.mutex.Unlock()

	if service.State != ServiceStateRunning {
		return errors.Errorf("service %s is not running", serviceType)
	}

	logger.Infof("Stopping service: %s", serviceType)
	service.State = ServiceStateStopping

	// Cancel the service context
	if service.cancelCtx != nil {
		service.cancelCtx()
	}

	// Wait for service to stop with timeout
	select {
	case <-service.done:
		logger.Infof("Service %s stopped gracefully", serviceType)
	case <-time.After(30 * time.Second):
		logger.Warnf("Service %s stop timeout, may not have stopped cleanly", serviceType)
	}

	service.State = ServiceStateStopped
	service.StartedAt = nil
	service.LastError = ""

	// Update database status
	if err := sm.updateServiceStatusInDB(service); err != nil {
		logger.Errorf("Failed to update service status in database: %v", err)
	}

	return nil
}

// StartAllServices starts all enabled services with proper dependency order
func (sm *ServiceManager) StartAllServices() error {
	// Define service start order (API first, then content provider)
	startOrder := []ServiceType{
		ServiceTypeAPI,
		ServiceTypeContentProvider,
	}

	for _, serviceType := range startOrder {
		if service := sm.services[serviceType]; service != nil && service.Config.Enabled && service.Config.AutoStart {
			if err := sm.StartService(serviceType); err != nil {
				logger.Errorf("Failed to start service %s: %v", serviceType, err)
				return err
			}
		}
	}

	return nil
}

// StopAllServices stops all running services in reverse dependency order
func (sm *ServiceManager) StopAllServices() error {
	// Stop in reverse order
	stopOrder := []ServiceType{
		ServiceTypeContentProvider,
		ServiceTypeAPI,
		ServiceTypeWorkerManager,
	}

	var lastError error
	for _, serviceType := range stopOrder {
		if service := sm.services[serviceType]; service != nil && service.State == ServiceStateRunning {
			if err := sm.StopService(serviceType); err != nil {
				logger.Errorf("Failed to stop service %s: %v", serviceType, err)
				lastError = err
			}
		}
	}

	return lastError
}

// GetServiceStatus returns the status of a specific service
func (sm *ServiceManager) GetServiceStatus(serviceType ServiceType) (*ServiceStatus, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	service, exists := sm.services[serviceType]
	if !exists {
		return nil, errors.Errorf("service type %s not found", serviceType)
	}

	service.mutex.RLock()
	defer service.mutex.RUnlock()

	return &ServiceStatus{
		Type:         service.Type,
		State:        service.State,
		StartedAt:    service.StartedAt,
		LastError:    service.LastError,
		HealthStatus: sm.getServiceHealthStatus(service),
		Config:       service.Config,
	}, nil
}

// GetAllServiceStatuses returns the status of all services
func (sm *ServiceManager) GetAllServiceStatuses() map[ServiceType]*ServiceStatus {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	statuses := make(map[ServiceType]*ServiceStatus)
	for serviceType, service := range sm.services {
		service.mutex.RLock()
		statuses[serviceType] = &ServiceStatus{
			Type:         service.Type,
			State:        service.State,
			StartedAt:    service.StartedAt,
			LastError:    service.LastError,
			HealthStatus: sm.getServiceHealthStatus(service),
			Config:       service.Config,
		}
		service.mutex.RUnlock()
	}

	return statuses
}

// Shutdown gracefully shuts down the service manager
func (sm *ServiceManager) Shutdown(ctx context.Context) error {
	logger.Info("Shutting down service manager")
	
	// Stop all services
	if err := sm.StopAllServices(); err != nil {
		logger.Errorf("Error stopping services: %v", err)
	}

	// Cancel manager context
	sm.cancel()

	return nil
}

// checkAndStartDependencies ensures service dependencies are met
func (sm *ServiceManager) checkAndStartDependencies(serviceType ServiceType) error {
	switch serviceType {
	case ServiceTypeContentProvider:
		// Content provider depends on API service
		apiService := sm.services[ServiceTypeAPI]
		if apiService != nil && apiService.Config.Enabled && apiService.State != ServiceStateRunning {
			logger.Info("Starting API service as dependency for content provider")
			if err := sm.StartService(ServiceTypeAPI); err != nil {
				return errors.Wrap(err, "failed to start API service dependency")
			}
		}
	}
	return nil
}

// getServiceHealthStatus returns the health status of a service
func (sm *ServiceManager) getServiceHealthStatus(service *Service) string {
	if service.State == ServiceStateRunning {
		return "healthy"
	} else if service.State == ServiceStateError && service.LastError != "" {
		return "unhealthy"
	}
	return "unknown"
}

// updateServiceStatusInDB updates the service status in the database
func (sm *ServiceManager) updateServiceStatusInDB(service *Service) error {
	status := &ServiceStatus{
		Type:         service.Type,
		State:        service.State,
		StartedAt:    service.StartedAt,
		LastError:    service.LastError,
		HealthStatus: sm.getServiceHealthStatus(service),
		Config:       service.Config,
	}
	
	statusModel := FromServiceStatus(status)
	return sm.repository.UpsertServiceStatus(statusModel)
}

// runAPIService runs the API service
func (sm *ServiceManager) runAPIService(ctx context.Context, service *Service) {
	defer close(service.done)
	defer func() {
		service.mutex.Lock()
		service.State = ServiceStateStopped
		service.StartedAt = nil
		service.mutex.Unlock()
	}()

	bind, ok := service.Config.Config["bind"].(string)
	if !ok {
		bind = ":9090"
	}

	logger.Infof("Starting API service on %s", bind)
	
	// Use a mock CLI context for now - this would be better integrated with actual CLI context
	mockCtx := &mockCLI{bind: bind, ctx: ctx}
	
	if err := api.Run(mockCtx); err != nil {
		service.mutex.Lock()
		service.State = ServiceStateError
		service.LastError = err.Error()
		service.mutex.Unlock()
		logger.Errorf("API service error: %v", err)
	}
}

// runContentProviderService runs the content provider service
func (sm *ServiceManager) runContentProviderService(ctx context.Context, service *Service) {
	defer close(service.done)
	defer func() {
		service.mutex.Lock()
		service.State = ServiceStateStopped
		service.StartedAt = nil
		service.mutex.Unlock()
	}()

	logger.Info("Starting content provider service")

	config := contentprovider.Config{
		HTTP: contentprovider.HTTPConfig{
			EnablePiece:         getBoolFromConfig(service.Config.Config, "enableHttpPiece", true),
			EnablePieceMetadata: getBoolFromConfig(service.Config.Config, "enableHttpPieceMetadata", true),
			Bind:                getStringFromConfig(service.Config.Config, "httpBind", "127.0.0.1:7777"),
		},
		Bitswap: contentprovider.BitswapConfig{
			Enable:           getBoolFromConfig(service.Config.Config, "enableBitswap", false),
			IdentityKey:      getStringFromConfig(service.Config.Config, "libp2pIdentityKey", ""),
			ListenMultiAddrs: getStringSliceFromConfig(service.Config.Config, "libp2pListenMultiAddrs", []string{}),
		},
	}

	contentProviderService, err := contentprovider.NewService(sm.db, config)
	if err != nil {
		service.mutex.Lock()
		service.State = ServiceStateError
		service.LastError = err.Error()
		service.mutex.Unlock()
		logger.Errorf("Failed to create content provider service: %v", err)
		return
	}

	if err := contentProviderService.Start(ctx); err != nil {
		service.mutex.Lock()
		service.State = ServiceStateError
		service.LastError = err.Error()
		service.mutex.Unlock()
		logger.Errorf("Content provider service error: %v", err)
	}
}

// runWorkerManagerService runs the worker manager service (placeholder)
func (sm *ServiceManager) runWorkerManagerService(ctx context.Context, service *Service) {
	defer close(service.done)
	defer func() {
		service.mutex.Lock()
		service.State = ServiceStateStopped
		service.StartedAt = nil
		service.mutex.Unlock()
	}()

	logger.Info("Worker manager service would run here (placeholder)")
	
	// This is a placeholder - worker manager is typically handled by the onboard command directly
	<-ctx.Done()
}

// Helper functions for configuration parsing
func getStringFromConfig(config map[string]interface{}, key, defaultValue string) string {
	if val, ok := config[key].(string); ok {
		return val
	}
	return defaultValue
}

func getBoolFromConfig(config map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := config[key].(bool); ok {
		return val
	}
	return defaultValue
}

func getStringSliceFromConfig(config map[string]interface{}, key string, defaultValue []string) []string {
	if val, ok := config[key].([]string); ok {
		return val
	}
	return defaultValue
}

// mockCLI is a simple mock implementation of cli.Context for testing
type mockCLI struct {
	bind string
	ctx  context.Context
}

func (m *mockCLI) String(name string) string {
	if name == "bind" {
		return m.bind
	}
	return ""
}

func (m *mockCLI) Context() context.Context {
	return m.ctx
}

// Add other required methods as needed for the mock
func (m *mockCLI) StringSlice(name string) []string { return nil }
func (m *mockCLI) Bool(name string) bool            { return false }
func (m *mockCLI) Int(name string) int              { return 0 }
func (m *mockCLI) IsSet(name string) bool           { return false }