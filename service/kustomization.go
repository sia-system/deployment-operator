package service

import (
	"bytes"
	"fmt"
	"text/template"

	yaml "gopkg.in/yaml.v2"
)

// Kustomization of k8s manifests
type Kustomization struct {
	Tier       string     `yaml:"tier"`
	Ns         string     `yaml:"ns"`
	Name       string     `yaml:"name"`
	Kind       string     `yaml:"kind"`
	OnlyFor    string     `yaml:"only-for"` // default `all`, for devel: 'devel', for prod: 'prod'
	Service    *Service   `yaml:"service"`
	Repository Repository `yaml:"repository"`
	Schedule   string     `yaml:"schedule"`
	Env        []EnvVar   `yaml:"env"`
}

// Repository is a Gitlab registry details
type Repository struct {
	Provider string `yaml:"provider"`
	Group    string `yaml:"group"`
	Project  string `yaml:"project"`
}

// Service details for proxy-manager
// See proxy-manager project, annotation aria.io/proxy-config
type Service struct {
	Timeout            string `yaml:"timeout"`
	ServiceTemplate    string `yaml:"service-template"`
	DeploymentTemplate string `yaml:"dxeployment-template"`
}

// EnvVar contains info for injecting environment variables into container
type EnvVar struct {
	Name      string        `yaml:"name"`
	Value     string        `yaml:"value"`
	ValueFrom *EnvVarSource `yaml:"valueFrom"`
}

// EnvVarSource represents a source for the value of an EnvVar.
type EnvVarSource struct {
	// Selects a key of a ConfigMap.
	// +optional
	ConfigMapKeyRef *ObjectKeyRef `yaml:"configMapKeyRef,omitempty"`
	// Selects a key of a secret in the pod's namespace
	// +optional
	SecretKeyRef *ObjectKeyRef `yaml:"secretKeyRef,omitempty"`
}

// ObjectKeyRef represents resource
type ObjectKeyRef struct {
	Name string `yaml:"name"`
	Key  string `yaml:"key"`
}

// ParseKustomization service annotation aria.io/proxy-config
func ParseKustomization(content []byte) (*Kustomization, error) {
	c := Kustomization{}
	if err := yaml.Unmarshal(content, &c); err != nil {
		return nil, fmt.Errorf("Can not unmarshar ProxyConfig: %v", err)
	}
	return &c, nil
}

type cronJobData struct {
	Ns       string
	Tier     string
	Name     string
	Group    string
	Project  string
	Schedule string
}

// KustomizeCronJob generate cronjob manifest for k8s
func KustomizeCronJob(kustomization *Kustomization, tmpl *template.Template) ([]byte, error) {
	repo := &kustomization.Repository

	data := cronJobData{
		Ns:       kustomization.Ns,
		Tier:     kustomization.Tier,
		Name:     kustomization.Name,
		Schedule: kustomization.Schedule,
		Group:    repo.Group,
		Project:  repo.Project,
	}

	manifestBuffer := new(bytes.Buffer)
	err := tmpl.Execute(manifestBuffer, data)
	if err != nil {
		return nil, fmt.Errorf("can not apply variables to cronjob template: %v", err)
	}
	return manifestBuffer.Bytes(), nil
}

type deploymentData struct {
	Ns      string
	Tier    string
	Name    string
	Group   string
	Project string
}

// example of github image:
// docker.pkg.github.com/sia-cronjobs/efacturi-client/sia-cronjobs-efacturi-client:v2020.04.24
// repository: docker.pkg.github.com
// organization/group: sia-cronjobs
// project: efacturi-client
// base image name: sia-cronjobs-efacturi-client

// KustomizeDeployment generate cronjob manifest for k8s
func KustomizeDeployment(kustomization *Kustomization, tmpl *template.Template) ([]byte, error) {
	repo := &kustomization.Repository

	data := deploymentData{
		Ns:      kustomization.Ns,
		Tier:    kustomization.Tier,
		Name:    kustomization.Name,
		Group:   repo.Group,
		Project: repo.Project,
	}

	manifestBuffer := new(bytes.Buffer)
	err := tmpl.Execute(manifestBuffer, data)
	if err != nil {
		return nil, fmt.Errorf("can not apply variables to deployment template: %v", err)
	}
	return manifestBuffer.Bytes(), nil
}

type serviceData struct {
	Ns      string
	Tier    string
	Name    string
	Default bool
	Timeout string
}

// KustomizeService generate cronjob manifest for k8s
func KustomizeService(kustomization *Kustomization, tmpl *template.Template) ([]byte, error) {
	data := serviceData{
		Ns:      kustomization.Ns,
		Tier:    kustomization.Tier,
		Name:    kustomization.Name,
		Timeout: kustomization.Service.Timeout,
	}

	manifestBuffer := new(bytes.Buffer)
	err := tmpl.Execute(manifestBuffer, data)
	if err != nil {
		return nil, fmt.Errorf("can not apply variables to service template: %v", err)
	}
	return manifestBuffer.Bytes(), nil
}
