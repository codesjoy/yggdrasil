// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
