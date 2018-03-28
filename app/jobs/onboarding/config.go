package onboarding

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"log"

	yaml "gopkg.in/yaml.v2"
)

type (
	// TaskEntry represents individual tasks to be assigned
	TaskEntry struct {
		Title       string
		Assignee    indirectAssignee `yaml:"assignee"`
		Description string           `yaml:"description,omitempty"`
		Tags        []string         `yaml:"tags,omitempty"` //TODO: add all the tags and determine if omitempty is what we want"
	}

	indirectAssignee struct {
		GithubUsername string `yaml:"github_username"`
	}

	// SetupScheme represents the whole workload to be scheduled.
	SetupScheme struct {
		ClientID           string                      `yaml:"clientId"`
		ClientSecret       string                      `yaml:"clientSecret"`
		GithubOrganization string                      `yaml:"githubOrganization"`
		GithubRepository   string                      `yaml:"githubRepository"`
		Tasks              []TaskEntry                 `yaml:"tasks"`
		TaskOwners         map[string]indirectAssignee `yaml:"task_owners"`
	}
)

func (assignee *indirectAssignee) String() string {
	return assignee.GithubUsername
}

func (setup *SetupScheme) ingest(data []byte, environ *map[string]string) error {
	var rendered bytes.Buffer

	context := map[string]map[string]string{
		"Environ": *environ,
	}

	tpl, err := template.New("config").Parse(string(data))

	if err != nil {
		return err
	}

	if err = tpl.Execute(&rendered, context); err != nil {
		return err
	}

	if err = yaml.Unmarshal(rendered.Bytes(), &setup); err != nil {
		log.Fatal(err)
	}

	return err
}

func (setup *SetupScheme) load(filename string, environ *map[string]string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	return setup.ingest(data, environ)
}

// NewSetupScheme constructs a SetupScheme instance, the combined effect of a template file and environment variables.
func NewSetupScheme(filename string, environ *map[string]string) (*SetupScheme, error) {
	setup := SetupScheme{}
	setup.load(filename, environ)

	return &setup, nil
}
