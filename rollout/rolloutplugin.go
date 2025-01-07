package rollout

import (
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/rollout/steps/plugin"
	log "github.com/sirupsen/logrus"
)

type rolloutPlugin struct {
	resolver plugin.Resolver
	log      *log.Entry

	rolloutPluginStatus []v1alpha1.RolloutPluginStatus
	hasError            bool
}
