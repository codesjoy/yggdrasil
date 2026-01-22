package polaris

import (
	polaris "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

type providerAPI interface {
	RegisterInstance(
		instance *polaris.InstanceRegisterRequest,
	) (*model.InstanceRegisterResponse, error)
	Deregister(instance *polaris.InstanceDeRegisterRequest) error
}

type consumerAPI interface {
	GetInstances(req *polaris.GetInstancesRequest) (*model.InstancesResponse, error)
}

type configAPI interface {
	FetchConfigFile(*polaris.GetConfigFileRequest) (model.ConfigFile, error)
}

type limitAPI interface {
	GetQuota(request polaris.QuotaRequest) (polaris.QuotaFuture, error)
	Destroy()
}

type circuitBreakerAPI interface {
	Check(model.Resource) (*model.CheckResult, error)
	Report(*model.ResourceStat) error
}

type routerAPI interface {
	ProcessRouters(*polaris.ProcessRoutersRequest) (*model.InstancesResponse, error)
	ProcessLoadBalance(*polaris.ProcessLoadBalanceRequest) (*model.OneInstanceResponse, error)
}
