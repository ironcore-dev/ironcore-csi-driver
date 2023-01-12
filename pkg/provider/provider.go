package provider

import (
	"github.com/dell/gocsi"
	"github.com/go-logr/logr"
	"github.com/onmetal/onmetal-csi-driver/pkg/driver"
)

func New(config map[string]string, log logr.Logger) gocsi.StoragePluginProvider {
	srvc := driver.New(config, log)
	return &gocsi.StoragePlugin{
		Controller:  srvc,
		Node:        srvc,
		Identity:    srvc,
		BeforeServe: srvc.BeforeServe,
		EnvVars: []string{
			// Enable request validation
			gocsi.EnvVarSpecReqValidation + "=true",

			// Enable serial volume access
			gocsi.EnvVarSerialVolAccess + "=true",
		},
	}
}
