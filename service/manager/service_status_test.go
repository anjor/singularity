package manager

import (
	"testing"
	"time"

	"github.com/data-preservation-programs/singularity/model"
	"github.com/data-preservation-programs/singularity/util/testutil"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type ServiceStatusTestSuite struct {
	suite.Suite
	testutil.TestDatabase
	repo *ServiceRepository
}

func TestServiceStatusTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceStatusTestSuite))
}

func (s *ServiceStatusTestSuite) SetupTest() {
	s.TestDatabase.SetupTest()
	s.repo = NewServiceRepository(s.DB)
}

func (s *ServiceStatusTestSuite) TearDownTest() {
	s.TestDatabase.TearDownTest()
}

func (s *ServiceStatusTestSuite) TestServiceStatusModel_ToServiceStatus() {
	now := time.Now()
	dbModel := &ServiceStatusModel{
		ServiceType:  "api",
		State:        "running",
		StartedAt:    &now,
		LastError:    "",
		HealthStatus: "healthy",
		Config: model.ConfigMap{
			"bind": ":9090",
		},
		Metadata: model.ConfigMap{
			"autoStart": true,
			"enabled":   true,
		},
	}

	status := dbModel.ToServiceStatus()
	s.Equal(ServiceTypeAPI, status.Type)
	s.Equal(ServiceStateRunning, status.State)
	s.Equal(&now, status.StartedAt)
	s.Equal("", status.LastError)
	s.Equal("healthy", status.HealthStatus)
	s.Equal(":9090", status.Config.Config["bind"])
	s.True(status.Config.AutoStart)
	s.True(status.Config.Enabled)
}

func (s *ServiceStatusTestSuite) TestFromServiceStatus() {
	now := time.Now()
	status := &ServiceStatus{
		Type:         ServiceTypeContentProvider,
		State:        ServiceStateRunning,
		StartedAt:    &now,
		LastError:    "test error",
		HealthStatus: "unhealthy",
		Config: ServiceConfig{
			Type: ServiceTypeContentProvider,
			Config: map[string]interface{}{
				"httpBind": "127.0.0.1:7777",
			},
			AutoStart: false,
			Enabled:   true,
		},
	}

	dbModel := FromServiceStatus(status)
	s.Equal("content_provider", dbModel.ServiceType)
	s.Equal("running", dbModel.State)
	s.Equal(&now, dbModel.StartedAt)
	s.Equal("test error", dbModel.LastError)
	s.Equal("unhealthy", dbModel.HealthStatus)
	s.Equal("127.0.0.1:7777", dbModel.Config["httpBind"])
	s.False(dbModel.Metadata["autoStart"].(bool))
	s.True(dbModel.Metadata["enabled"].(bool))
}

func (s *ServiceStatusTestSuite) TestServiceRepository_CreateAndGet() {
	status := &ServiceStatusModel{
		ServiceType:  "api",
		State:        "running",
		HealthStatus: "healthy",
		Config: model.ConfigMap{
			"bind": ":9090",
		},
		Metadata: model.ConfigMap{
			"autoStart": true,
			"enabled":   true,
		},
	}

	// Create service status
	err := s.repo.CreateServiceStatus(status)
	s.NoError(err)
	s.NotZero(status.ID)

	// Get service status
	retrieved, err := s.repo.GetServiceStatus(ServiceTypeAPI)
	s.NoError(err)
	s.Equal(status.ServiceType, retrieved.ServiceType)
	s.Equal(status.State, retrieved.State)
	s.Equal(status.HealthStatus, retrieved.HealthStatus)
}

func (s *ServiceStatusTestSuite) TestServiceRepository_Upsert() {
	status := &ServiceStatusModel{
		ServiceType:  "api",
		State:        "stopped",
		HealthStatus: "unknown",
		Config: model.ConfigMap{
			"bind": ":9090",
		},
	}

	// First upsert (create)
	err := s.repo.UpsertServiceStatus(status)
	s.NoError(err)

	// Second upsert (update)
	status.State = "running"
	status.HealthStatus = "healthy"
	err = s.repo.UpsertServiceStatus(status)
	s.NoError(err)

	// Verify update
	retrieved, err := s.repo.GetServiceStatus(ServiceTypeAPI)
	s.NoError(err)
	s.Equal("running", retrieved.State)
	s.Equal("healthy", retrieved.HealthStatus)
}

func (s *ServiceStatusTestSuite) TestServiceRepository_GetAllServiceStatuses() {
	// Create multiple service statuses
	statuses := []*ServiceStatusModel{
		{
			ServiceType:  "api",
			State:        "running",
			HealthStatus: "healthy",
		},
		{
			ServiceType:  "content_provider",
			State:        "stopped",
			HealthStatus: "unknown",
		},
	}

	for _, status := range statuses {
		err := s.repo.CreateServiceStatus(status)
		s.NoError(err)
	}

	// Get all statuses
	allStatuses, err := s.repo.GetAllServiceStatuses()
	s.NoError(err)
	s.Len(allStatuses, 2)

	// Verify both services are present
	serviceTypes := make(map[string]bool)
	for _, status := range allStatuses {
		serviceTypes[status.ServiceType] = true
	}
	s.True(serviceTypes["api"])
	s.True(serviceTypes["content_provider"])
}

func (s *ServiceStatusTestSuite) TestServiceRepository_GetServicesByState() {
	// Create services with different states
	statuses := []*ServiceStatusModel{
		{
			ServiceType:  "api",
			State:        "running",
			HealthStatus: "healthy",
		},
		{
			ServiceType:  "content_provider",
			State:        "running",
			HealthStatus: "healthy",
		},
		{
			ServiceType:  "worker_manager",
			State:        "stopped",
			HealthStatus: "unknown",
		},
	}

	for _, status := range statuses {
		err := s.repo.CreateServiceStatus(status)
		s.NoError(err)
	}

	// Get running services
	runningServices, err := s.repo.GetRunningServices()
	s.NoError(err)
	s.Len(runningServices, 2)

	for _, service := range runningServices {
		s.Equal("running", service.State)
	}

	// Get stopped services
	stoppedServices, err := s.repo.GetServicesByState(ServiceStateStopped)
	s.NoError(err)
	s.Len(stoppedServices, 1)
	s.Equal("worker_manager", stoppedServices[0].ServiceType)
}

func (s *ServiceStatusTestSuite) TestServiceRepository_DeleteServiceStatus() {
	status := &ServiceStatusModel{
		ServiceType:  "api",
		State:        "running",
		HealthStatus: "healthy",
	}

	// Create service status
	err := s.repo.CreateServiceStatus(status)
	s.NoError(err)

	// Verify it exists
	_, err = s.repo.GetServiceStatus(ServiceTypeAPI)
	s.NoError(err)

	// Delete it
	err = s.repo.DeleteServiceStatus(ServiceTypeAPI)
	s.NoError(err)

	// Verify it's gone
	_, err = s.repo.GetServiceStatus(ServiceTypeAPI)
	s.Error(err)
	s.Equal(gorm.ErrRecordNotFound, err)
}

func (s *ServiceStatusTestSuite) TestServiceRepository_UpdateServiceStatus() {
	status := &ServiceStatusModel{
		ServiceType:  "api",
		State:        "stopped",
		HealthStatus: "unknown",
	}

	// Create service status
	err := s.repo.CreateServiceStatus(status)
	s.NoError(err)

	// Update it
	updates := map[string]interface{}{
		"state":         "running",
		"health_status": "healthy",
	}
	err = s.repo.UpdateServiceStatus(ServiceTypeAPI, updates)
	s.NoError(err)

	// Verify update
	retrieved, err := s.repo.GetServiceStatus(ServiceTypeAPI)
	s.NoError(err)
	s.Equal("running", retrieved.State)
	s.Equal("healthy", retrieved.HealthStatus)
}

func (s *ServiceStatusTestSuite) TestServiceRepository_CleanupStaleServices() {
	// Create old service status
	oldTime := time.Now().Add(-2 * time.Hour)
	status := &ServiceStatusModel{
		ServiceType:  "api",
		State:        "running",
		HealthStatus: "healthy",
	}
	err := s.repo.CreateServiceStatus(status)
	s.NoError(err)

	// Manually update the UpdatedAt field to make it old
	err = s.DB.Model(status).UpdateColumn("updated_at", oldTime).Error
	s.NoError(err)

	// Create recent service status
	recentStatus := &ServiceStatusModel{
		ServiceType:  "content_provider",
		State:        "running",
		HealthStatus: "healthy",
	}
	err = s.repo.CreateServiceStatus(recentStatus)
	s.NoError(err)

	// Cleanup services older than 1 hour
	cutoff := time.Now().Add(-1 * time.Hour)
	err = s.repo.CleanupStaleServices(cutoff)
	s.NoError(err)

	// Old service should be gone
	_, err = s.repo.GetServiceStatus(ServiceTypeAPI)
	s.Error(err)
	s.Equal(gorm.ErrRecordNotFound, err)

	// Recent service should still exist
	_, err = s.repo.GetServiceStatus(ServiceTypeContentProvider)
	s.NoError(err)
}