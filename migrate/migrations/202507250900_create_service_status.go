package migrations

import (
	"time"

	"github.com/data-preservation-programs/singularity/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// ServiceStatusModel represents a service status record in the database
type ServiceStatusModel struct {
	gorm.Model
	ID           uint            `gorm:"primaryKey" json:"id"`
	ServiceType  string          `gorm:"uniqueIndex;not null" json:"serviceType"`
	State        string          `gorm:"not null" json:"state"`
	StartedAt    *time.Time      `json:"startedAt,omitempty"`
	StoppedAt    *time.Time      `json:"stoppedAt,omitempty"`
	LastError    string          `json:"lastError,omitempty"`
	HealthStatus string          `json:"healthStatus"`
	Config       model.ConfigMap `gorm:"type:jsonb" json:"config"`
	Metadata     model.ConfigMap `gorm:"type:jsonb" json:"metadata"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

// TableName returns the table name for the ServiceStatusModel
func (ServiceStatusModel) TableName() string {
	return "service_statuses"
}

var Migration202507250900CreateServiceStatus = &gormigrate.Migration{
	ID: "202507250900_create_service_status",
	Migrate: func(tx *gorm.DB) error {
		// Create the service_statuses table
		return tx.AutoMigrate(&ServiceStatusModel{})
	},
	Rollback: func(tx *gorm.DB) error {
		// Drop the service_statuses table
		return tx.Migrator().DropTable("service_statuses")
	},
}