package gitclient

import (
	"context"
	"fmt"
	// "strings"

	"github.com/google/go-github/v31/github"
	"golang.org/x/oauth2"

	"demius.md/deployment-operator/api"
)

// GithubClient incapsulate github client api
type GithubClient struct {
	ctx    context.Context
	client *github.Client
}

// ConnectGithub connects to gitlab
// my connect token is: remote-api-token
func ConnectGithub(provider, secret string) *GithubClient {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: secret},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	// client.SetBaseURL("https://git.mydomain.com/api/v4")

	return &GithubClient{ctx, client}
}

// ProviderName return docker registry provider name
func (c *GithubClient) ProviderName() string {
	return "gitHUB"
}

// LoadImageTag load tag of docker image for project
func (c *GithubClient) LoadImageTag(groupName, projectName, mode string) (*api.ReleaseInfo, error) {
	organization, _, err := c.client.Organizations.Get(c.ctx, groupName)
	if err != nil {
		return nil, fmt.Errorf("find organization `%s` error: %v", groupName, err)
	}

	login := organization.GetLogin()
	if len(login) == 0 {
		return nil, fmt.Errorf("organization `%s` does not have login", groupName)
	}

	repository, _, err := c.client.Repositories.Get(c.ctx, login, projectName)
	if err != nil {
		return nil, fmt.Errorf("find project `%s` error: %v", projectName, err)
	}

	releases, _, err := c.client.Repositories.ListReleases(c.ctx, login, repository.GetName(), &github.ListOptions{
		Page: 0,
		PerPage: 3,
	})
	if err != nil {
		return nil, fmt.Errorf("list releases of project `%s` error: %v", projectName, err)
	}
	for _, rel := range releases {
		/*
		if !strings.HasSuffix(rel.GetTagName(), mode) {
			fmt.Printf("LoadImageTag, tag %s has invalid suffix, must be: %s\n", rel.GetTagName(), mode)
			continue
		}
		*/
		return &api.ReleaseInfo{
			ImageTag:    rel.GetTagName(),
			ReleaseDate: rel.GetPublishedAt().String(),
		}, nil
	}

	return nil, nil
}
