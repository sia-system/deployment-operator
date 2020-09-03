package service

import (
	"io/ioutil"
	"log"

	yaml "gopkg.in/yaml.v2"
)

// DeployConfig contains all info about deployment in k8s
type DeployConfig struct {
	Certs           CertsConf                 `yaml:"certs"`
	ServerPort      int                       `yaml:"server-port"`
	DeployTemplates string                    `yaml:"templates"`
	Kustomizations  string                    `yaml:"kustomizations"`
	Providers       map[string]ProviderConfig `yaml:"providers"`
}

// CertsConf contains location of key/cert files
type CertsConf struct {
	KeyFile  string `yaml:"key-file"`
	CertFile string `yaml:"cert-file"`
}

// ProviderConfig contains info about git provider
type ProviderConfig struct {
	URL    string `yaml:"url"`          // base url of git provider
	Type   string `yaml:"api-type"`     // may be gitlab or github
	Secret string `yaml:"secret-token"` // api access secret token
}

// LoadDeployConfig load config of deployment
func LoadDeployConfig(path string) *DeployConfig {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("%s get err #%v", path, err)
	}
	var config = DeployConfig{}

	if err = yaml.Unmarshal(file, &config); err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	return &config
}
