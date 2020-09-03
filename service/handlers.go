package service

import (
	"context"
	"log"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	apibatch "k8s.io/api/batch/v1beta1"
)

type artifactHandler interface {
	Find() (bool, error)
	Kustomize() error
	Create() error
	Update() (bool, error)
	Remove() error
}

type baseHandler struct {
	ctx           context.Context
	server        *deploymentServer
	tmpl          *template.Template
	kustomization *Kustomization
	initVariables []EnvVar
	manifest      []byte
}

type cronjobHandler struct {
	baseHandler
	job *apibatch.CronJob
}

type deploymentHandler struct {
	baseHandler
	deployment *appsv1.Deployment
}

func createBaseHandler(ctx context.Context, server *deploymentServer, tmpl *template.Template, kustomization *Kustomization, initVariables []EnvVar) baseHandler {
	return baseHandler{ctx, server, tmpl, kustomization, initVariables, nil}
}

func createCronjobHandler(bh baseHandler) *cronjobHandler {
	return &cronjobHandler{bh, nil}
}

func createDeploymentHandler(bh baseHandler) *deploymentHandler {
	return &deploymentHandler{bh, nil}
}

func (c *cronjobHandler) Find() (bool, error) {
	job, err := c.server.findCronjob(c.ctx, c.kustomization.Ns, c.kustomization.Name, c.kustomization.Tier)
	if err != nil {
		return false, err
	}
	c.job = job
	return job != nil, nil
}

func (c *cronjobHandler) Kustomize() error {
	if c.manifest != nil {
		return nil
	}
	manifest, err := KustomizeCronJob(c.kustomization, c.tmpl)
	if err != nil {
		return err
	}
	c.manifest = manifest
	return nil
}

func (c *cronjobHandler) Create() error {
	return c.server.createCronjob(c.ctx, c.manifest, c.kustomization.Env, c.initVariables)
}

func (c *cronjobHandler) Update() (bool, error) {
	repo := &c.kustomization.Repository
	updated, err := c.server.updateCronjob(c.ctx, c.job, repo)
	if err != nil {
		return false, err
	}
	return updated, nil
}

func (c *cronjobHandler) Remove() error {
	return c.server.removeCronjob(c.ctx, c.job)
}

func (c *deploymentHandler) Find() (bool, error) {
	deployment, err := c.server.findDeployment(c.ctx, c.kustomization.Ns, c.kustomization.Name, c.kustomization.Tier)
	if err != nil {
		return false, err
	}
	c.deployment = deployment
	return deployment != nil, nil
}

func (c *deploymentHandler) Kustomize() error {
	if c.manifest != nil {
		return nil
	}
	log.Printf("Kustomize deployment with %v\n", c.tmpl)
	manifest, err := KustomizeDeployment(c.kustomization, c.tmpl)
	if err != nil {
		return err
	}
	c.manifest = manifest
	return nil
}

func (c *deploymentHandler) Create() error {
	return c.server.createDeployment(c.ctx, c.manifest, c.kustomization.Env, c.initVariables)
}

func (c *deploymentHandler) Update() (bool, error) {
	repo := &c.kustomization.Repository
	updated, err := c.server.updateDeployment(c.ctx, c.deployment, repo)
	if err != nil {
		return false, err
	}
	return updated, nil
}

func (c *deploymentHandler) Remove() error {
	return c.server.removeDeployment(c.ctx, c.deployment)
}
