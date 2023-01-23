// Copyright 2023 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"github.com/dell/gocsi"
	"github.com/go-logr/logr"
	"github.com/onmetal/onmetal-csi-driver/pkg/driver"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(config map[string]string, targetClient, onMetalClient client.Client, log logr.Logger) gocsi.StoragePluginProvider {
	drv := driver.New(config, targetClient, onMetalClient, log)
	return &gocsi.StoragePlugin{
		Controller:  drv,
		Node:        drv,
		Identity:    drv,
		BeforeServe: drv.BeforeServe,
		EnvVars: []string{
			// Enable request validation
			gocsi.EnvVarSpecReqValidation + "=true",

			// Enable serial volume access
			gocsi.EnvVarSerialVolAccess + "=true",
		},
	}
}
