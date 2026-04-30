package nomad

import (
	"testing"

	nomadapi "github.com/hashicorp/nomad/api"
)

func TestNomadImport(t *testing.T) {
	var client *nomadapi.Client
	_ = client

	var job nomadapi.Job
	_ = job

	var alloc nomadapi.Allocation
	_ = alloc
}
