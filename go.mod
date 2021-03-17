module demius.md/deployment-operator

go 1.16

// https://github.com/kubernetes/client-go/blob/master/INSTALL.md#go-modules

require (
	github.com/golang/protobuf v1.4.3
	github.com/google/go-github/v31 v31.0.0
	github.com/xanzy/go-gitlab v0.47.0
	golang.org/x/oauth2 v0.0.0-20210313182246-cd4f82c27b84
	google.golang.org/grpc v1.36.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.4
)
