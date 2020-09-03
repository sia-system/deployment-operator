package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"demius.md/deployment-operator/api"
	"demius.md/deployment-operator/service"
	"demius.md/deployment-operator/utils"
)

// MaxMessageSize maximum message size of GRPC
const MaxMessageSize = 1024 * 1024

func main() {
	executablePath := utils.ExecutableDir()
	homePath := utils.UserHomeDir()
	fmt.Println("executable path: " + executablePath)
	fmt.Println("      home path: " + homePath)

	// disable check of http cert because wrong acc cert and Marina gitlab
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	config := service.LoadDeployConfig("/etc/deploy/config.yaml")

	listenURL := fmt.Sprintf("0.0.0.0:%v", config.ServerPort)
	listener, err := net.Listen("tcp", listenURL)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	certs := config.Certs

	if certs.CertFile == "" || certs.KeyFile == "" {
		log.Fatalln("No key & crt files are specified in the environment")
	} else {
		log.Println("Key-file: " + certs.KeyFile + "; certs-file: " + certs.CertFile)
	}

	creds, err := credentials.NewServerTLSFromFile(certs.CertFile, certs.KeyFile)
	if err != nil {
		log.Fatalf("Failed to generate credentials %v", err)
	}
	opts = []grpc.ServerOption{grpc.Creds(creds), grpc.MaxRecvMsgSize(MaxMessageSize)}

	grpcServer := grpc.NewServer(opts...)
	api.RegisterDeploymentServer(grpcServer, service.NewServer(config.DeployTemplates, config.Kustomizations, config.Providers))

	log.Printf("Starting deploy-operator at `%s`\n", listenURL)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	log.Println("grpc server stopped")
}
