package cmd

import (
	"flag"
	"testing"

	"github.com/data-preservation-programs/singularity/service/manager"
	"github.com/data-preservation-programs/singularity/util/testutil"
	"github.com/stretchr/testify/suite"
	"github.com/urfave/cli/v2"
)

type OnboardServiceTestSuite struct {
	suite.Suite
	testutil.TestDatabase
}

func TestOnboardServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OnboardServiceTestSuite))
}

func (s *OnboardServiceTestSuite) SetupTest() {
	s.TestDatabase.SetupTest()
}

func (s *OnboardServiceTestSuite) TearDownTest() {
	s.TestDatabase.TearDownTest()
}

func (s *OnboardServiceTestSuite) TestOnboardServiceFlags() {
	// Test that the onboard command has the expected service management flags
	flags := OnboardCmd.Flags
	s.NotEmpty(flags)

	// Find service management flags
	var foundFlags []string
	for _, flag := range flags {
		switch f := flag.(type) {
		case *cli.BoolFlag:
			if f.Category == "Service Management" {
				foundFlags = append(foundFlags, f.Name)
			}
		case *cli.StringFlag:
			if f.Category == "Service Management" {
				foundFlags = append(foundFlags, f.Name)
			}
		}
	}

	// Verify expected service management flags are present
	expectedFlags := []string{
		"start-services",
		"api-bind",
		"content-provider-bind",
		"enable-content-provider-bitswap",
		"stop-services-on-completion",
	}

	for _, expected := range expectedFlags {
		s.Contains(foundFlags, expected, "Expected flag %s not found", expected)
	}
}

func (s *OnboardServiceTestSuite) TestServiceManagerConfiguration() {
	// Create a mock CLI context with service flags
	app := &cli.App{
		Flags: OnboardCmd.Flags,
	}

	// Set up test flags
	set := flag.NewFlagSet("test", 0)
	set.String("api-bind", ":8080", "API bind address")
	set.String("content-provider-bind", "127.0.0.1:8888", "Content provider bind address")
	set.Bool("enable-content-provider-bitswap", true, "Enable bitswap")

	ctx := cli.NewContext(app, set, nil)

	// Test that we can create a service manager config from CLI context
	config := manager.ServiceManagerConfig{
		APIConfig: manager.APIServiceConfig{
			Bind:    ctx.String("api-bind"),
			Enabled: true,
		},
		ContentProviderConfig: manager.ContentProviderServiceConfig{
			HTTPBind:                ctx.String("content-provider-bind"),
			EnableHTTPPiece:         true,
			EnableHTTPPieceMetadata: true,
			EnableBitswap:           ctx.Bool("enable-content-provider-bitswap"),
			Enabled:                 true,
		},
		WorkerManagerConfig: manager.WorkerManagerServiceConfig{
			Enabled: false,
		},
	}

	s.Equal(":8080", config.APIConfig.Bind)
	s.True(config.APIConfig.Enabled)
	s.Equal("127.0.0.1:8888", config.ContentProviderConfig.HTTPBind)
	s.True(config.ContentProviderConfig.EnableBitswap)
	s.False(config.WorkerManagerConfig.Enabled)
}

func (s *OnboardServiceTestSuite) TestServiceManagerCreation() {
	// Test that we can create a service manager with database
	config := manager.DefaultServiceManagerConfig()
	sm := manager.NewServiceManager(s.DB, config)

	s.NotNil(sm)

	// Test that all expected services are configured
	statuses := sm.GetAllServiceStatuses()
	s.Len(statuses, 3)

	expectedServices := []manager.ServiceType{
		manager.ServiceTypeAPI,
		manager.ServiceTypeContentProvider,
		manager.ServiceTypeWorkerManager,
	}

	for _, expectedService := range expectedServices {
		s.Contains(statuses, expectedService)
		s.Equal(manager.ServiceStateStopped, statuses[expectedService].State)
	}
}

func (s *OnboardServiceTestSuite) TestOnboardCommandIntegration() {
	// Test the integration between onboard command and service management
	// This is a unit test that verifies the command structure without actually running services

	// Verify that the onboard command includes service management in its description
	s.Contains(OnboardCmd.Description, "Optionally starts API and content provider services")

	// Verify that service management is mentioned as step 3 in the workflow
	s.Contains(OnboardCmd.Description, "3. Optionally starts API and content provider services")

	// Test that default values are reasonable
	for _, flag := range OnboardCmd.Flags {
		switch f := flag.(type) {
		case *cli.BoolFlag:
			switch f.Name {
			case "start-services":
				s.False(f.Value, "start-services should default to false")
			case "stop-services-on-completion":
				s.False(f.Value, "stop-services-on-completion should default to false")
			case "enable-content-provider-bitswap":
				s.False(f.Value, "enable-content-provider-bitswap should default to false")
			}
		case *cli.StringFlag:
			switch f.Name {
			case "api-bind":
				s.Equal(":9090", f.Value, "api-bind should default to :9090")
			case "content-provider-bind":
				s.Equal("127.0.0.1:7777", f.Value, "content-provider-bind should default to 127.0.0.1:7777")
			}
		}
	}
}