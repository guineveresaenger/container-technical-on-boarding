package app

import (
	"fmt"

	"github.com/masterminds/semver"
	"github.com/revel/revel"
	"github.com/samsung-cnct/container-technical-on-boarding/app/jobs/onboarding"
)

// Version represents the application version
type Version struct {
	Name     string          `json:"name"`
	Number   string          `json:"number"`
	Build    string          `json:"build"`
	Semantic *semver.Version `json:"semantic"`
}

var (
	// SemanticVersion of app
	SemanticVersion *Version

	// BuildTime revel app build-time (ldflags)
	BuildTime string

	// Configs for onboard app loaded from conf/app.conf. These are required at startup.
	Configs = make(map[string]string)

	// Setup contains settings for the onboarding github job
	Setup *onboarding.SetupScheme

	// Credentials contains gitub app credentials
	Credentials *onboarding.Credentials
)

func init() {
	// Filters is the default set of global filters.
	revel.Filters = []revel.Filter{
		revel.PanicFilter,             // Recover from panics and display an error page instead.
		revel.RouterFilter,            // Use the routing table to select the right Action
		revel.FilterConfiguringFilter, // A hook for adding or removing per-Action filters.
		revel.ParamsFilter,            // Parse parameters into Controller.Params.
		revel.SessionFilter,           // Restore and write the session cookie.
		revel.FlashFilter,             // Restore and write the flash cookie.
		revel.ValidationFilter,        // Restore kept validation errors and save new ones from cookie.
		revel.I18nFilter,              // Resolve the requested language
		HeaderFilter,                  // Add some security based headers
		revel.InterceptorFilter,       // Run interceptors around the action.
		revel.CompressFilter,          // Compress the result.
		revel.ActionInvoker,           // Invoke the action.
	}

	revel.OnAppStart(SetupVersion)
	revel.OnAppStart(LoadConfigs)
	revel.OnAppStart(SetupScheme)
	revel.OnAppStart(SetupCredentials)
}

// HeaderFilter is used by the revel server
var HeaderFilter = func(c *revel.Controller, fc []revel.Filter) {
	c.Response.Out.Header().Add("X-Frame-Options", "SAMEORIGIN")
	c.Response.Out.Header().Add("X-XSS-Protection", "1; mode=block")
	c.Response.Out.Header().Add("X-Content-Type-Options", "nosniff")
	fc[0](c, fc[1:]) // Execute the next filter stage.
}

// Onboard specific configuration names
const (
	OnboardClientIDName     string = "onboard.client.id"
	OnboardClientSecretName string = "onboard.client.secret"
	OnboardOrgName          string = "onboard.org"
	OnboardRepoName         string = "onboard.repo"
	OnboardTasksFileName    string = "onboard.tasks.file"
	OnboardUserName         string = "onboard.user"
)

// SetupVersion for revel web app from revel configs
func SetupVersion() {
	name := revel.Config.StringDefault("app.name", "")
	version := revel.Config.StringDefault("app.version", "")
	build := revel.Config.StringDefault("app.build", "")
	semVersionString := fmt.Sprintf("%s+%s", version, build)

	semVersion, err := semver.NewVersion(semVersionString)
	if err != nil {
		revel.WARN.Printf("Cannot setup semantic version from configuration ('%s': %v)", semVersionString, err)
		semVersion, _ = semver.NewVersion("0.0.0+000")
	}

	SemanticVersion = &Version{
		Name:     name,
		Number:   version,
		Build:    build,
		Semantic: semVersion,
	}
	revel.INFO.Printf("Semantic version setup: %v", SemanticVersion)
}

// LoadConfigs for onboarding workflow
func LoadConfigs() {
	Configs[OnboardClientIDName] = revel.Config.StringDefault(OnboardClientIDName, "")
	Configs[OnboardClientSecretName] = revel.Config.StringDefault(OnboardClientSecretName, "")
	Configs[OnboardOrgName] = revel.Config.StringDefault(OnboardOrgName, "")
	Configs[OnboardRepoName] = revel.Config.StringDefault(OnboardRepoName, "")
	Configs[OnboardTasksFileName] = revel.Config.StringDefault(OnboardTasksFileName, "")

	for env, value := range Configs {
		if len(value) == 0 {
			revel.ERROR.Fatalf("The '%s' property is required on startup. check the conf/app.conf", env)
		}
	}
	revel.INFO.Printf("Configs Loaded")
}

// SetupScheme for executing an onboarding workflow
func SetupScheme() {
	configFilename := Configs[OnboardTasksFileName]
	setup, err := onboarding.NewSetupScheme(configFilename, &Configs)
	if err != nil {
		revel.ERROR.Fatalf("Cannat create an onboarding github setup scheme: %v", err)
	}
	Setup = setup
	revel.INFO.Printf("Scheme Setup")
}

// SetupCredentials for github oauth2 authorization code grant workflow
func SetupCredentials() {
	Credentials = &onboarding.Credentials{
		ClientID:     Setup.ClientID,
		ClientSecret: Setup.ClientSecret,
		Scopes:       []string{"user", "repo", "issues", "milestones"},
	}
	revel.INFO.Printf("Credentials Setup")
}
