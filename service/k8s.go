package service

import (
	"bytes"
	"context"
	"fmt"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	apibatch "k8s.io/api/batch/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sYaml "k8s.io/apimachinery/pkg/util/yaml"

	"k8s.io/apimachinery/pkg/api/errors"
)

// ManifestType enum of manifest file types
type ManifestType = int

const (
	// CronJobManifest contains k8s manifest for cronjob resource
	CronJobManifest ManifestType = iota
	// DeploymentManifest contains k8s manifest for deployment resource
	DeploymentManifest
	// ServiceManifest contains k8s manifest for service resource
	ServiceManifest
)

// FindCronjob find allready existed cronjob with namespace ns
func (s *deploymentServer) findCronjob(ctx context.Context, ns, name, tier string) (*apibatch.CronJob, error) {
	// log.Println("find cronjob " + ns + " : " + name + "." + tier)
	log.Println("find cronjob " + ns + " : " + name)
	batchAPI := s.clientset.BatchV1beta1()
	apiJobs := batchAPI.CronJobs(ns)

	// cronjob, err := apiJobs.Get(ctx, name+"."+tier, metav1.GetOptions{})
	cronjob, err := apiJobs.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		switch t := err.(type) {
		case *errors.StatusError:
			{
				statusCode := t.Status().Code
				if statusCode == 404 {
					return nil, nil
				}
				return nil, fmt.Errorf("could not get cronjob `%s`, got error '%v' with status %d", name, err, statusCode)
			}
		}
		return nil, fmt.Errorf("could not get cronjob `%s`, got error '%v'", name, err)
	}
	return cronjob, nil
}

// CreateCronjob create new cronjob from manifest
func (s *deploymentServer) createCronjob(ctx context.Context, manifest []byte, env []EnvVar, initVariables []EnvVar) error {
	decoder := k8sYaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 1000)

	j := &apibatch.CronJob{}

	if err := decoder.Decode(&j); err != nil {
		return err
	}

	if len(env) > 0 {
		containers := j.Spec.JobTemplate.Spec.Template.Spec.Containers
		applyEnvironment(containers, env)
	}

	initContainers := j.Spec.JobTemplate.Spec.Template.Spec.InitContainers
	if len(initContainers) > 0 {
		fmt.Println("job " + j.Namespace + "." + j.Name + " has initContainers")
		applyEnvironment(initContainers, initVariables)
	} else {
		fmt.Println("job " + j.Namespace + "." + j.Name + " has not initContainers; bug in config")
	}

	batchAPI := s.clientset.BatchV1beta1()
	apiJobs := batchAPI.CronJobs(j.Namespace)

	if _, err := apiJobs.Create(ctx, j, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("job create error '%s'", err.Error())
	}
	return nil
}

// UpdateCronjob update allready existed cronjob with new image
func (s *deploymentServer) updateCronjob(ctx context.Context, job *apibatch.CronJob, repo *Repository) (bool, error) {
	containers := job.Spec.JobTemplate.Spec.Template.Spec.InitContainers

	if len(containers) > 0 {
		fmt.Println("job " + job.Namespace + "." + job.Name + " has initContainers")
	} else {
		fmt.Println("job " + job.Namespace + "." + job.Name + " has not initContainers; can not update")
	}

	var grace int64 = 5
	podsAPI := s.clientset.CoreV1().Pods(job.Namespace)
	if err := podsAPI.DeleteCollection(
		ctx, metav1.DeleteOptions{GracePeriodSeconds: &grace},
		metav1.ListOptions{LabelSelector: "sia-app=" + job.Name}); err != nil {
		return false, fmt.Errorf("could not find and delete pods for restart: %v", err)
	}

	return true, nil
}

// RemoveCronjob remove cronjob from k8s
func (s *deploymentServer) removeCronjob(ctx context.Context, job *apibatch.CronJob) error {
	batchAPI := s.clientset.BatchV1beta1()
	apiJobs := batchAPI.CronJobs(job.Namespace)

	if err := apiJobs.Delete(ctx, job.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("cronjob delete error `%v`", err)
	}

	return nil
}

// FindDeployment find allready existed deployment with namespace ns
func (s *deploymentServer) findDeployment(ctx context.Context, ns, name, tier string) (*appsv1.Deployment, error) {
	// log.Println("find deployment " + ns + " : " + name + "." + tier)
	log.Println("find deployment " + ns + " : " + name)
	appsAPI := s.clientset.AppsV1()
	apiDeployments := appsAPI.Deployments(ns)

	// deployment, err := apiDeployments.Get(ctx, name+"."+tier, metav1.GetOptions{})
	deployment, err := apiDeployments.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		switch t := err.(type) {
		case *errors.StatusError:
			{
				statusCode := t.Status().Code
				if statusCode == 404 {
					return nil, nil
				}
				return nil, fmt.Errorf("could not get deployment `%s`, got error '%v' with status %d", name, err, statusCode)
			}
		}
		return nil, fmt.Errorf("could not get deployment `%s`, got error '%v'", name, err)
	}
	return deployment, nil
}

// RemoveDeployment remove deployment from k8s
func (s *deploymentServer) removeDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	appsAPI := s.clientset.AppsV1()
	apiDeployments := appsAPI.Deployments(deployment.Namespace)

	if err := apiDeployments.Delete(ctx, deployment.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("deployment delete error `%v`", err)
	}

	return nil
}

// CreateDeployment create new deployment from manifest
func (s *deploymentServer) createDeployment(ctx context.Context, manifest []byte, env []EnvVar, initVariables []EnvVar) error {
	decoder := k8sYaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 1000)

	d := &appsv1.Deployment{}

	if err := decoder.Decode(&d); err != nil {
		return err
	}

	if len(env) > 0 {
		containers := d.Spec.Template.Spec.Containers
		applyEnvironment(containers, env)
	}

	initContainers := d.Spec.Template.Spec.InitContainers
	if len(initContainers) > 0 {
		fmt.Println("deployment " + d.Namespace + "." + d.Name + " has initContainers")
		applyEnvironment(initContainers, initVariables)
	} else {
		fmt.Println("deployment " + d.Namespace + "." + d.Name + " has not initContainers; bug in config")
	}

	appsAPI := s.clientset.AppsV1()
	apiDeployments := appsAPI.Deployments(d.Namespace)

	if _, err := apiDeployments.Create(ctx, d, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("deployment create error '%s'", err.Error())
	}

	return nil
}

func applyEnvironment(containers []apiv1.Container, env []EnvVar) {
	if len(containers) == 0 {
		return
	}

	var cenv = make([]apiv1.EnvVar, len(env))

	for i, e := range env {
		// fmt.Printf("env name: %s  ", e.Name)
		if e.ValueFrom == nil && len(e.Value) > 0 {
			var envvar = apiv1.EnvVar{
				Name:  e.Name,
				Value: e.Value,
			}
			cenv[i] = envvar
			continue
		}

		var src *apiv1.EnvVarSource

		if e.ValueFrom.SecretKeyRef != nil {
			var sec = e.ValueFrom.SecretKeyRef

			// fmt.Printf("SecretKeyRef{ key:%s, name %s}\n", sec.Key, sec.Name)

			src = &apiv1.EnvVarSource{
				SecretKeyRef: &apiv1.SecretKeySelector{
					Key: sec.Key,
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: sec.Name,
					},
				},
			}
		} else if e.ValueFrom.ConfigMapKeyRef != nil {
			var sec = e.ValueFrom.ConfigMapKeyRef

			// fmt.Printf("ConfigMapKeyRef{ key:%s, name %s}\n", sec.Key, sec.Name)

			src = &apiv1.EnvVarSource{
				ConfigMapKeyRef: &apiv1.ConfigMapKeySelector{
					Key: sec.Key,
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: sec.Name,
					},
				},
			}
		}

		var envvar = apiv1.EnvVar{
			Name:      e.Name,
			ValueFrom: src,
		}
		cenv[i] = envvar
	}

	containers[0].Env = append(containers[0].Env, cenv...)
}

// TODO: UpdateDeployment via remove and create new pods !!!
// UpdateDeployment update allready existed deployment with new image
func (s *deploymentServer) updateDeployment(ctx context.Context, deployment *appsv1.Deployment, repo *Repository) (bool, error) {
	containers := deployment.Spec.Template.Spec.InitContainers

	if len(containers) > 0 {
		fmt.Println("deployment " + deployment.Namespace + "." + deployment.Name + " has initContainers")
	} else {
		fmt.Println("deployment " + deployment.Namespace + "." + deployment.Name + " has not initContainers; can not update")
	}

	var grace int64 = 5
	podsAPI := s.clientset.CoreV1().Pods(deployment.Namespace)
	if err := podsAPI.DeleteCollection(
		ctx,
		metav1.DeleteOptions{GracePeriodSeconds: &grace},
		metav1.ListOptions{LabelSelector: "sia-app=" + deployment.Name}); err != nil {
		return false, fmt.Errorf("could not find and delete pods for restart: %v", err)
	}

	return true, nil
}

// ApplyService create new service if not exists
func (s *deploymentServer) applyService(ctx context.Context, manifest []byte) error {
	decoder := k8sYaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 1000)

	srv := &apiv1.Service{}

	if err := decoder.Decode(&srv); err != nil {
		return err
	}

	println("     applyService " + srv.Namespace + ":" + srv.Name)

	api := s.clientset.CoreV1()
	apiServices := api.Services(srv.Namespace)
	if _, err := apiServices.Get(ctx, srv.Name, metav1.GetOptions{}); err != nil {
		// create service
		log.Printf("Error getting service: %v\n", err)
		if _, err := apiServices.Create(ctx, srv, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("service create error '%s'", err.Error())
		}
	}
	return nil
}
