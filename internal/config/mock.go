package config

import (
	"context"
	"github.com/dghubble/oauth1"
	"github.com/stretchr/testify/mock"
	jira "github.com/uwu-tools/go-jira/v2/cloud"
	"time"
)

type ConfigMock struct {
	mock.Mock
}

func NewConfigMock() *ConfigMock {
	return &ConfigMock{}
}

func (c *ConfigMock) Context() context.Context {
	args := c.Called()
	return args.Get(0).(context.Context)
}

func (c *ConfigMock) LoadJiraConfig(client *jira.Client) error {
	args := c.Called(client)
	return args.Error(0) //nolint:wrapcheck
}

func (c *ConfigMock) GetConfigFile() string {
	args := c.Called()
	return args.String(0)
}

func (c *ConfigMock) GetConfigString(key string) string {
	args := c.Called(key)
	return args.String(0)
}

func (c *ConfigMock) IsBasicAuth() bool {
	args := c.Called()
	return args.Bool(0)
}

func (c *ConfigMock) GetSinceParam() time.Time {
	args := c.Called()
	return args.Get(0).(time.Time)
}

func (c *ConfigMock) IsDryRun() bool {
	args := c.Called()
	return args.Bool(0)
}

func (c *ConfigMock) IsDaemon() bool {
	args := c.Called()
	return args.Bool(0)
}

func (c *ConfigMock) GetDaemonPeriod() time.Duration {
	args := c.Called()
	return args.Get(0).(time.Duration)
}

func (c *ConfigMock) GetTimeout() time.Duration {
	args := c.Called()
	return args.Get(0).(time.Duration)
}

func (c *ConfigMock) GetFieldID(key FieldKey) string {
	args := c.Called(key)
	return args.String(0)
}

func (c *ConfigMock) GetFieldKey(key FieldKey) string {
	args := c.Called(key)
	return args.String(0)
}

func (c *ConfigMock) GetProject() *jira.Project {
	args := c.Called()
	return args.Get(0).(*jira.Project)
}

func (c *ConfigMock) GetProjectKey() string {
	args := c.Called()
	return args.String(0)
}

func (c *ConfigMock) GetRepo() (string, string) {
	args := c.Called()
	return args.String(0), args.String(1)
}

func (c *ConfigMock) SetJiraToken(token *oauth1.Token) {
	c.Called(token)
}

func (c *ConfigMock) SaveConfig() error {
	args := c.Called()
	return args.Error(0) //nolint:wrapcheck
}
