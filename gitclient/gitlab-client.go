package gitclient

import (
	"crypto/tls"
	"fmt"
	"net/http"
	// "strings"
	"time"

	gitlab "github.com/xanzy/go-gitlab"

	"demius.md/deployment-operator/api"
)

// TODO: if provider is docker.pkg.github.com then use github
// for provier: registry.gitlab.com
// Connect to gitlab
// my connect token is: remote-api-token

// GitlabClient incapsulate gitlab client api
type GitlabClient struct {
	httpclient *http.Client
	client *gitlab.Client
}

// ConnectGitlab connects to gitlab
func ConnectGitlab(provider, secret string) *GitlabClient {
	// fmt.Printf("GITLAB api secret token: %s\n", secret)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpclient := &http.Client{Transport: tr}

	git, err := gitlab.NewClient(secret,
		gitlab.WithBaseURL("https://"+provider+"/api/v4"),
		gitlab.WithHTTPClient(httpclient),
	)
	if err != nil {
		panic(err.Error())
	}

	return &GitlabClient{httpclient, git}
}

// ProviderName return docker registry provider name
func (c *GitlabClient) ProviderName() string {
	return "gitLAB"
}

// LoadImageTag load tag of docker image for project
func (c *GitlabClient) LoadImageTag(groupName, projectName, mode string) (*api.ReleaseInfo, error) {
	// fmt.Printf("LoadImageTag, find: %s:%s\n", groupName, projectName)

	groups, _, err := c.client.Groups.SearchGroup(groupName)
	if err != nil {
		fmt.Printf("LoadImageTag, find group error: %v\n", err)
		return nil, fmt.Errorf("find group `%s` error: %v", groupName, err)
	}

	for _, g := range groups {
		// fmt.Printf("LoadImageTag, found group: %v\n", g.ID)
		owned := true
		opt := &gitlab.ListGroupProjectsOptions{
			Owned:  &owned,
			Search: &projectName,
		}

		projects, _, err := c.client.Groups.ListGroupProjects(g.ID, opt)
		if err != nil {
			return nil, fmt.Errorf("find project `%s` error: %v", projectName, err)
		}

		for _, p := range projects {
			fmt.Printf("LoadImageTag, found project: %v\n", p.Path)
			if p.Path != projectName {
				continue
			}

			opt := &gitlab.ListReleasesOptions{
				Page:    0,
				PerPage: 3,
			}

			releases, _, err := c.client.Releases.ListReleases(p.ID, opt)
			if err != nil {
				return nil, fmt.Errorf("list release error: %v", err)
			}
			for _, rel := range releases {
				/*
				if !strings.HasSuffix(rel.TagName, mode) {
					fmt.Printf("LoadImageTag, tag %s has invalid suffix, must be: %s\n", rel.TagName, mode)
					continue
				}
				*/
				createdAt := rel.CreatedAt
				createdAtStr := ""
				if createdAt != nil {
					createdAtStr = createdAt.Format(time.RFC822)
				}
				return &api.ReleaseInfo{
					ImageTag:    rel.TagName,
					ReleaseDate: createdAtStr,
				}, nil
			}

		}
	}

	return nil, nil
}
