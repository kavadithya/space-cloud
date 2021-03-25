package plugins

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cuelang.org/go/cue"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/oam-dev/kubevela/apis/types"
	"github.com/oam-dev/kubevela/pkg/utils/common"
)

const (
	TestDir        = "testdata"
	RouteName      = "routes.test"
	DeployName     = "deployments.testapps"
	WebserviceName = "webservice.testapps"
)

var _ = Describe("DefinitionFiles", func() {

	deployment := types.Capability{
		Namespace:   "testdef",
		Name:        DeployName,
		Type:        types.TypeWorkload,
		CrdName:     "deployments.apps",
		Description: "description not defined",
		Parameters: []types.Parameter{
			{
				Type: cue.ListKind,
				Name: "env",
			},
			{
				Name:     "image",
				Type:     cue.StringKind,
				Default:  "",
				Short:    "i",
				Required: true,
				Usage:    "Which image would you like to use for your service",
			},
			{
				Name:    "port",
				Type:    cue.IntKind,
				Short:   "p",
				Default: int64(8080),
				Usage:   "Which port do you want customer traffic sent to",
			},
		},
		CrdInfo: &types.CRDInfo{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
	}

	websvc := types.Capability{
		Namespace:   "testdef",
		Name:        WebserviceName,
		Type:        types.TypeWorkload,
		Description: "description not defined",
		Parameters: []types.Parameter{{
			Name: "env", Type: cue.ListKind,
		}, {
			Name:     "image",
			Type:     cue.StringKind,
			Default:  "",
			Short:    "i",
			Required: true,
			Usage:    "Which image would you like to use for your service",
		}, {
			Name:    "port",
			Type:    cue.IntKind,
			Short:   "p",
			Default: int64(6379),
			Usage:   "Which port do you want customer traffic sent to",
		}},
		CrdName: "deployments.apps",
		CrdInfo: &types.CRDInfo{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
	}

	req, _ := labels.NewRequirement("usecase", selection.Equals, []string{"forplugintest"})
	selector := labels.NewSelector().Add(*req)

	// Notice!!  DefinitionPath Object is Cluster Scope object
	// which means objects created in other DefinitionNamespace will also affect here.
	It("getworkload", func() {
		workloadDefs, _, err := GetWorkloadsFromCluster(context.Background(), DefinitionNamespace, common.Args{Config: cfg, Schema: scheme}, selector)
		Expect(err).Should(BeNil())
		logf.Log.Info(fmt.Sprintf("Getting workload definitions  %v", workloadDefs))
		for i := range workloadDefs {
			// CueTemplate should always be fulfilled, even those whose CueTemplateURI is assigend,
			By("check CueTemplate is fulfilled")
			Expect(workloadDefs[i].CueTemplate).ShouldNot(BeEmpty())
			workloadDefs[i].CueTemplate = ""
		}
		Expect(cmp.Diff(workloadDefs, []types.Capability{deployment, websvc})).Should(BeEquivalentTo(""))
	})
	It("getall", func() {
		alldef, err := GetCapabilitiesFromCluster(context.Background(), DefinitionNamespace, common.Args{Config: cfg, Schema: scheme}, selector)
		Expect(err).Should(BeNil())
		logf.Log.Info(fmt.Sprintf("Getting all definitions %v", alldef))
		for i := range alldef {
			alldef[i].CueTemplate = ""
		}
		Expect(cmp.Diff(alldef, []types.Capability{deployment, websvc})).Should(BeEquivalentTo(""))
	})
})