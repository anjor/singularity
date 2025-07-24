package manager

import (
	"time"

	"github.com/data-preservation-programs/singularity/model"
	"gorm.io/gorm"
)

// ServiceStatusModel represents a service status record in the database
type ServiceStatusModel struct {
	gorm.Model
	ID           uint                 `gorm:"primaryKey" json:"id"`
	ServiceType  string               `gorm:"uniqueIndex;not null" json:"serviceType"`
	State        string               `gorm:"not null" json:"state"`
	StartedAt    *time.Time           `json:"startedAt,omitempty"`
	StoppedAt    *time.Time           `json:"stoppedAt,omitempty"`
	LastError    string               `json:"lastError,omitempty"`
	HealthStatus string               `json:"healthStatus"`
	Config       model.ConfigMap      `gorm:"type:jsonb" json:"config"`
	Metadata     model.ConfigMap      `gorm:"type:jsonb" json:"metadata"`
	CreatedAt    time.Time            `json:"createdAt"`
	UpdatedAt    time.Time            `json:"updatedAt"`
}

// TableName returns the table name for the ServiceStatusModel
func (ServiceStatusModel) TableName() string {
	return "service_statuses"
}

// ToServiceStatus converts the database model to a ServiceStatus struct
func (s *ServiceStatusModel) ToServiceStatus() *ServiceStatus {
	serviceType := ServiceType(s.ServiceType)
	state := ServiceState(s.State)
	
	// Convert config map to proper ServiceConfig
	config := ServiceConfig{
		Type: serviceType,
		Config: make(map[string]interface{}),
		AutoStart: true, // Default value
		Enabled: true,   // Default value
	}
	
	// Copy config from database
	for key, value := range s.Config {
		config.Config[key] = value
	}
	
	// Extract autoStart and enabled from metadata if available
	if s.Metadata != nil {
		if autoStart, ok := s.Metadata["autoStart"].(bool); ok {
			config.AutoStart = autoStart
		}
		if enabled, ok := s.Metadata["enabled"].(bool); ok {
			config.Enabled = enabled
		}
	}

	return &ServiceStatus{
		Type:         serviceType,
		State:        state,
		StartedAt:    s.StartedAt,
		LastError:    s.LastError,
		HealthStatus: s.HealthStatus,
		Config:       config,
	}
}

// FromServiceStatus creates a ServiceStatusModel from a ServiceStatus
func FromServiceStatus(status *ServiceStatus) *ServiceStatusModel {
	var stoppedAt *time.Time
	if status.State == ServiceStateStopped {
		now := time.Now()
		stoppedAt = &now
	}

	// Convert ServiceConfig to database format
	config := make(model.ConfigMap)
	for key, value := range status.Config.Config {
		config[key] = value
	}

	metadata := model.ConfigMap{
		"autoStart": status.Config.AutoStart,
		"enabled":   status.Config.Enabled,
	}

	return &ServiceStatusModel{
		ServiceType:  string(status.Type),
		State:        string(status.State),
		StartedAt:    status.StartedAt,
		StoppedAt:    stoppedAt,
		LastError:    status.LastError,
		HealthStatus: status.HealthStatus,
		Config:       config,
		Metadata:     metadata,
	}
}

// ServiceRepository provides database operations for service status
type ServiceRepository struct {
	db *gorm.DB
}

// NewServiceRepository creates a new service repository
func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

// GetServiceStatus gets the status of a service from the database
func (r *ServiceRepository) GetServiceStatus(serviceType ServiceType) (*ServiceStatusModel, error) {
	var status ServiceStatusModel
	err := r.db.Where("service_type = ?", string(serviceType)).First(&status).Error
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// GetAllServiceStatuses gets all service statuses from the database
func (r *ServiceRepository) GetAllServiceStatuses() ([]ServiceStatusModel, error) {
	var statuses []ServiceStatusModel
	err := r.db.Find(&statuses).Error
	return statuses, err
}

// UpsertServiceStatus inserts or updates a service status in the database
func (r *ServiceRepository) UpsertServiceStatus(status *ServiceStatusModel) error {
	return r.db.Save(status).Error
}

// DeleteServiceStatus deletes a service status from the database
func (r *ServiceRepository) DeleteServiceStatus(serviceType ServiceType) error {
	return r.db.Where("service_type = ?", string(serviceType)).Delete(&ServiceStatusModel{}).Error
}

// CreateServiceStatus creates a new service status record
func (r *ServiceRepository) CreateServiceStatus(status *ServiceStatusModel) error {
	return r.db.Create(status).Error
}

// UpdateServiceStatus updates an existing service status record
func (r *ServiceRepository) UpdateServiceStatus(serviceType ServiceType, updates map[string]interface{}) error {
	return r.db.Model(&ServiceStatusModel{}).Where("service_type = ?", string(serviceType)).Updates(updates).Error
}

// GetServicesByState gets all services with a specific state
func (r *ServiceRepository) GetServicesByState(state ServiceState) ([]ServiceStatusModel, error) {
	var statuses []ServiceStatusModel
	err := r.db.Where("state = ?", string(state)).Find(&statuses).Error
	return statuses, err
}

// GetRunningServices gets all currently running services
func (r *ServiceRepository) GetRunningServices() ([]ServiceStatusModel, error) {
	return r.GetServicesByState(ServiceStateRunning)
}

// GetFailedServices gets all services in error state
func (r *ServiceRepository) GetFailedServices() ([]ServiceStatusModel, error) {
	return r.GetServicesByState(ServiceStateError)
}

// CleanupStaleServices removes service records that haven't been updated recently
func (r *ServiceRepository) CleanupStaleServices(olderThan time.Time) error {
	return r.db.Where("updated_at < ?", olderThan).Delete(&ServiceStatusModel{}).Error
}