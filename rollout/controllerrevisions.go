package rollout

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/controller"
)

func (c *Controller) getControllerRevisionsForRollouts(r *v1alpha1.Rollout) ([]*appsv1.ControllerRevision, error) {
	ctx := context.TODO()
	// List all ControllerRevisions to find those we own but that no longer match our
	// selector. They will be orphaned by ClaimControllerRevisions().
	crList, err := c.controllerRevisionLister.ControllerRevisions(r.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	controllerRevisionSelector, err := metav1.LabelSelectorAsSelector(r.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("rollout %s/%s has invalid label selector: %v", r.Namespace, r.Name, err)
	}

	canAdoptFunc := controller.RecheckDeletionTimestamp(func(ctx context.Context) (metav1.Object, error) {
		fresh, err := c.argoprojclientset.ArgoprojV1alpha1().Rollouts(r.Namespace).Get(ctx, r.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.UID != r.UID {
			return nil, fmt.Errorf("original Rollout %v/%v is gone: got uid %v, wanted %v", r.Namespace, r.Name, fresh.UID, r.UID)
		}
		return fresh, nil
	})
	cm := controller.NewControllerRevisionControllerRefManager(c.controllerRevsionControl, r, controllerRevisionSelector, controllerKind, canAdoptFunc)
	return cm.ClaimControllerRevisions(ctx, crList)
}
