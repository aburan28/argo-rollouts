package plugin

import "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"

type RolloutPlugin interface {
	Reconcile(r *v1alpha1.Rollout) error
	Rollback(r *v1alpha1.Rollout) error
	Abort(r *v1alpha1.Rollout) error
}
