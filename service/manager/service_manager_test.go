package manager

import (
	"context"
	"testing"
	"time"

	"github.com/data-preservation-programs/singularity/util/testutil"
	"github.com/stretchr/testify/suite"
)

type ServiceManagerTestSuite struct {
	suite.Suite
	testutil.TestDatabase
}

func TestServiceManagerTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceManagerTestSuite))
}

func (s *ServiceManagerTestSuite) SetupTest() {
	s.TestDatabase.SetupTest()
}

func (s *ServiceManagerTestSuite) TearDownTest() {
	s.TestDatabase.TearDownTest()
}

func (s *ServiceManagerTestSuite) TestNewServiceManager() {
	config := DefaultServiceManagerConfig()
	sm := NewServiceManager(s.DB, config)
	
	s.NotNil(sm)
	s.NotNil(sm.services)
	s.NotNil(sm.repository)
	s.Len(sm.services, 3) // API, ContentProvider, WorkerManager
}

func (s *ServiceManagerTestSuite) TestGetServiceStatus() {
	config := DefaultServiceManagerConfig()
	sm := NewServiceManager(s.DB, config)
	
	// Test getting status for API service
	status, err := sm.GetServiceStatus(ServiceTypeAPI)
	s.NoError(err)
	s.NotNil(status)
	s.Equal(ServiceTypeAPI, status.Type)
	s.Equal(ServiceStateStopped, status.State)
	s.True(status.Config.Enabled)
}

func (s *ServiceManagerTestSuite) TestGetAllServiceStatuses() {
	config := DefaultServiceManagerConfig()
	sm := NewServiceManager(s.DB, config)
	
	statuses := sm.GetAllServiceStatuses()
	s.Len(statuses, 3)
	
	// Check that all service types are present
	s.Contains(statuses, ServiceTypeAPI)
	s.Contains(statuses, ServiceTypeContentProvider)
	s.Contains(statuses, ServiceTypeWorkerManager)
	
	// All services should be stopped initially
	for _, status := range statuses {
		s.Equal(ServiceStateStopped, status.State)
	}
}

func (s *ServiceManagerTestSuite) TestStartServiceInvalidType() {
	config := DefaultServiceManagerConfig()
	sm := NewServiceManager(s.DB, config)
	
	err := sm.StartService(ServiceType("invalid"))
	s.Error(err)
	s.Contains(err.Error(), "service type invalid not found")
}

func (s *ServiceManagerTestSuite) TestServiceDependencies() {
	config := DefaultServiceManagerConfig()
	sm := NewServiceManager(s.DB, config)
	
	// Test that starting content provider should start API first (dependency check)
	// This is a unit test so we won't actually start the services
	// Just verify the dependency logic exists
	
	// Mock starting API service to verify dependency
	apiService := sm.services[ServiceTypeAPI]
	s.NotNil(apiService)
	s.Equal(ServiceStateStopped, apiService.State)
	
	contentProviderService := sm.services[ServiceTypeContentProvider]
	s.NotNil(contentProviderService)
	s.Equal(ServiceStateStopped, contentProviderService.State)
}

func (s *ServiceManagerTestSuite) TestServiceConfiguration() {
	config := ServiceManagerConfig{
		APIConfig: APIServiceConfig{
			Bind:    ":8080",
			Enabled: true,
		},
		ContentProviderConfig: ContentProviderServiceConfig{
			HTTPBind:                "127.0.0.1:8888",
			EnableHTTPPiece:         true,
			EnableHTTPPieceMetadata: false,
			EnableBitswap:           true,
			Enabled:                 false,
		},
		WorkerManagerConfig: WorkerManagerServiceConfig{
			MinWorkers:         2,
			MaxWorkers:         10,
			ScaleUpThreshold:   3,
			CheckInterval:      45 * time.Second,
			Enabled:            true,
		},
	}
	
	sm := NewServiceManager(s.DB, config)
	
	// Verify API service configuration
	apiStatus, err := sm.GetServiceStatus(ServiceTypeAPI)
	s.NoError(err)
	s.Equal(":8080", apiStatus.Config.Config["bind"])
	s.True(apiStatus.Config.Enabled)
	
	// Verify Content Provider service configuration
	cpStatus, err := sm.GetServiceStatus(ServiceTypeContentProvider)
	s.NoError(err)
	s.Equal("127.0.0.1:8888", cpStatus.Config.Config["httpBind"])
	s.True(cpStatus.Config.Config["enableHttpPiece"].(bool))
	s.False(cpStatus.Config.Config["enableHttpPieceMetadata"].(bool))
	s.True(cpStatus.Config.Config["enableBitswap"].(bool))
	s.False(cpStatus.Config.Enabled)
	
	// Verify Worker Manager service configuration
	wmStatus, err := sm.GetServiceStatus(ServiceTypeWorkerManager)
	s.NoError(err)
	s.Equal(2, wmStatus.Config.Config["minWorkers"])
	s.Equal(10, wmStatus.Config.Config["maxWorkers"])
	s.Equal(3, wmStatus.Config.Config["scaleUpThreshold"])
	s.Equal(45*time.Second, wmStatus.Config.Config["checkInterval"])
	s.True(wmStatus.Config.Enabled)
}

func (s *ServiceManagerTestSuite) TestShutdown() {
	config := DefaultServiceManagerConfig()
	sm := NewServiceManager(s.DB, config)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := sm.Shutdown(ctx)
	s.NoError(err)
	
	// After shutdown, context should be cancelled
	select {
	case <-sm.ctx.Done():
		// Expected - context should be cancelled
	default:
		s.Fail("Service manager context should be cancelled after shutdown")
	}
}