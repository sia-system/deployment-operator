package service

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"demius.md/deployment-operator/api"
	"demius.md/deployment-operator/gitclient"
)

// MaxServicesCount is maximum number of services in one deployment call
const MaxServicesCount = 32

type deploymentServer struct {
	clientset *kubernetes.Clientset

	templates      Templates
	kustomizations string
	providers      map[string]ProviderConfig

	gitclients map[string]gitclient.GitClient
}

// NewServer create new grpc server
func NewServer(deployTemplates, kustomizations string, providers map[string]ProviderConfig) api.DeploymentServer {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	templates, err := LoadTemplates(deployTemplates)
	if err != nil {
		panic(err.Error())
	}

	gitclients := make(map[string]gitclient.GitClient)

	for provider, providerConf := range providers {
		var gitcli gitclient.GitClient

		if val, ok := gitclients[provider]; ok {
			gitcli = val
		} else {
			if providerConf.Type == "gitlab" {
				println("   connect to gitlab provider " + provider)
				gitcli = gitclient.ConnectGitlab(provider, providerConf.Secret)
			} else if providerConf.Type == "github" {
				println("   connect to github provider " + provider)
				gitcli = gitclient.ConnectGithub(provider, providerConf.Secret)
			} else {
				println("Unknwn provider type: " + providerConf.Type)
				continue
			}

			gitclients[provider] = gitcli
		}

	}

	println("deployment-server-impl created")
	s := &deploymentServer{clientset, templates, kustomizations, providers, gitclients}
	return s
}

func (s *deploymentServer) Deploy(ctx context.Context, request *api.Request) (*api.Response, error) {
	println("deploymentServer.Deploy")

	source := s.kustomizations
	prefixLen := len(source) + 1

	if len(request.Path) > 0 {
		source = filepath.Join(source, request.Path)
	}

	log.Printf("request %s %v\n", source, request.Recreate)

	return s.walkApplications(ctx, prefixLen, source, request.Recreate, request.Mode)
}

func respError(errorDesc string) *api.Response {
	return &api.Response{
		ResponseVariants: &api.Response_ErrorDescription{
			ErrorDescription: errorDesc,
		},
	}
}

func (s *deploymentServer) walkApplications(ctx context.Context, prefixLen int, source string, recreate bool, serverMode api.ServerMode) (*api.Response, error) {
	services := make([]*api.ServiceInfo, MaxServicesCount)
	idx := 0
	err := filepath.Walk(source, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walk dir%s: %v", path, err)
		}
		if f.IsDir() {
			return nil
		}

		services[idx] = s.handleKustomization(ctx, prefixLen, path, recreate, serverMode)
		idx++

		if idx >= MaxServicesCount {
			return fmt.Errorf("Maximum number of services in one deployment call exceeded: %v", MaxServicesCount)
		}

		return err
	})
	var response = &api.Response{
		ResponseVariants: &api.Response_ServicesResponse{
			ServicesResponse: &api.ServicesResponse{Services: services[:idx]},
		},
	}
	return response, err
}

func (s *deploymentServer) handleKustomization(ctx context.Context, prefixLen int, path string, recreate bool, serverMode api.ServerMode) *api.ServiceInfo {
	filename := filepath.Base(path)

	serviceInfo := &api.ServiceInfo{
		Path: extrtactArtifactPath(prefixLen, path, filename),
	}
	if filename != "kustomization.yaml" {
		return serviceInfoWithError(serviceInfo, "file with customization must be `kustomization.yaml`, actual: "+filename)
	}

	filedata, err := ioutil.ReadFile(path)
	if err != nil {
		return serviceInfoWithError(serviceInfo, "can not read `kustomization.yaml`: "+err.Error())
	}

	kustomization, err := ParseKustomization(filedata)
	if err != nil {
		return serviceInfoWithError(serviceInfo, "can not parse `kustomization.yaml`: "+err.Error())
	}

	serviceInfo.ServiceId = &api.ServiceID{
		Group:   kustomization.Repository.Group,
		Package: kustomization.Name,
		Kind:    kustomization.Kind,
	}

	gitclient := s.gitclients[kustomization.Repository.Provider]

	serviceInfo.Provider = gitclient.ProviderName()

	var srvMode string

	if serverMode == api.ServerMode_Development {
		srvMode = "devel"
	} else {
		srvMode = "prod"
	}

	disabled := !(kustomization.OnlyFor == "" || kustomization.OnlyFor == "all" || kustomization.OnlyFor == srvMode)

	releaseInfo, err := gitclient.LoadImageTag(kustomization.Repository.Group, kustomization.Repository.Project, srvMode)
	if err != nil {
		return serviceInfoWithError(serviceInfo, "can not load image tag from git: "+err.Error())
	}

	if releaseInfo == nil {
		return serviceInfoWithError(serviceInfo, "not found group or project in git")
	}

	if len(releaseInfo.ImageTag) == 0 {
		return serviceInfoWithError(serviceInfo, "not found image tag in git")
	}

	serviceInfo.Release = releaseInfo

	log.Printf("srv: %s/%s - %s:%s\n", kustomization.Repository.Group, kustomization.Name, kustomization.Kind, releaseInfo.ImageTag)

	initVariables := []EnvVar{{Name: "APP_SERVER_MODE", Value: srvMode}}

	if kustomization.Kind == "cronjob" {
		action, err := s.handleCronjob(ctx, kustomization, recreate, disabled, initVariables)
		if err != nil {
			return serviceInfoWithError(serviceInfo, err.Error())
		}
		return serviceInfoWithAction(serviceInfo, action)
	} else if kustomization.Kind == "deployment" {
		action, err := s.handleDeployment(ctx, kustomization, recreate, disabled, initVariables)
		if err != nil {
			return serviceInfoWithError(serviceInfo, err.Error())
		}

		if kustomization.Service != nil {
			if err = s.handleService(ctx, kustomization, recreate); err != nil {
				return serviceInfoWithError(serviceInfo, err.Error())
			}
		}
		return serviceInfoWithAction(serviceInfo, action)
	} else {
		return serviceInfoWithError(serviceInfo, "unknown kind of kustomization")
	}
}

func serviceInfoWithError(info *api.ServiceInfo, errorDescription string) *api.ServiceInfo {
	info.ActionVariants = &api.ServiceInfo_ErrorDescription{
		ErrorDescription: errorDescription,
	}
	return info
}

func serviceInfoWithAction(info *api.ServiceInfo, action api.Action) *api.ServiceInfo {
	info.ActionVariants = &api.ServiceInfo_Action{
		Action: action,
	}
	return info
}

func extrtactArtifactPath(prefixLen int, path, filename string) string {
	if len(path) > (prefixLen + len(filename)) {
		return path[prefixLen : len(path)-len(filename)-1]
	}
	return path
}

func (s *deploymentServer) handleCronjob(ctx context.Context, kustomization *Kustomization, recreate, disabled bool, initVariables []EnvVar) (api.Action, error) {
	tmpl := s.templates[CronJobKind][""]
	bh := createBaseHandler(ctx, s, tmpl, kustomization, initVariables)
	handler := createCronjobHandler(bh)
	return handleArtifact(handler, recreate, disabled)
}

func (s *deploymentServer) handleDeployment(ctx context.Context, kustomization *Kustomization, recreate, disabled bool, initVariables []EnvVar) (api.Action, error) {
	tmpl := s.templates[DeploymentKind][kustomization.Tier]
	bh := createBaseHandler(ctx, s, tmpl, kustomization, initVariables)
	handler := createDeploymentHandler(bh)
	return handleArtifact(handler, recreate, disabled)
}

func handleArtifact(handler artifactHandler, recreate, disabled bool) (api.Action, error) {
	found, err := handler.Find()
	if err != nil {
		return api.Action_NotChanged, err
	}

	needRemove := found && (recreate || disabled)
	needUpdate := found && !recreate && !disabled
	needCreate := (!found || recreate) && !disabled

	if needRemove {
		println("     remove")
		if err = handler.Remove(); err != nil {
			return api.Action_NotChanged, err
		}
		time.Sleep(2 * time.Second)
	} else if needUpdate {
		println("     update")
		updated, err := handler.Update()
		if err != nil {
			return api.Action_NotChanged, err
		}
		if updated {
			return api.Action_Updated, nil
		}
		return api.Action_NotChanged, nil
	}

	if needCreate {
		println("     create")
		if err = handler.Kustomize(); err != nil {
			return api.Action_NotChanged, err
		}
		if err := handler.Create(); err != nil {
			return api.Action_NotChanged, err
		}
		if recreate {
			return api.Action_Recreated, nil
		}
		return api.Action_Created, nil
	} else if needRemove {
		return api.Action_Removed, handler.Remove()
	}

	return api.Action_NotChanged, nil
}

func (s *deploymentServer) handleService(ctx context.Context, kustomization *Kustomization, recreate bool) error {
	template := s.templates[ServiceKind][kustomization.Service.Template]

	if template == nil {
		fmt.Printf("kustomize service %s.%s - %s not found template %s\n", kustomization.Ns, kustomization.Name, kustomization.Tier, kustomization.Service.Template)
		return fmt.Errorf("kustomize service %s.%s - %s not found template %s", kustomization.Ns, kustomization.Name, kustomization.Tier, kustomization.Service.Template)
	}

	manifest, err := KustomizeService(kustomization, template)
	if err != nil {
		return err
	}

	fmt.Printf("kustomize service %s.%s - %s with\n%v\n", kustomization.Ns, kustomization.Name, kustomization.Tier, string(manifest))

	if err = s.applyService(ctx, manifest); err != nil {
		return err
	}

	return nil
}
