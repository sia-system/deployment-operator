package gitclient

import (
	"demius.md/deployment-operator/api"
)

// GitClient is abstraction over Github and Gitlab
type GitClient interface {
	LoadImageTag(groupName, projectName, mode string) (*api.ReleaseInfo, error)
	ProviderName() string
}
