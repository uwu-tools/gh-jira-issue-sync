package config

import (
	"context"
	"github.com/dghubble/oauth1"
	jira "github.com/uwu-tools/go-jira/v2/cloud"
	"time"
)

type IConfig interface {
	Context() context.Context
	LoadJiraConfig(client *jira.Client) error
	GetConfigFile() string
	GetConfigString(key string) string
	IsBasicAuth() bool
	GetSinceParam() time.Time
	IsDryRun() bool
	IsDaemon() bool
	GetDaemonPeriod() time.Duration
	GetTimeout() time.Duration
	GetFieldID(key FieldKey) string
	GetFieldKey(key FieldKey) string
	GetProject() *jira.Project
	GetProjectKey() string
	GetRepo() (string, string)
	SetJiraToken(token *oauth1.Token)
	SaveConfig() error
}
