package rollout

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/utils/hash"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// func (c *Controller) getStatefulSetsForRollouts(r *v1alpha1.Rollout) ([]*appsv1.StatefulSet, error) {
// 	// ctx := context.TODO()
// 	// List all StatefulSets to find those we own but that no longer match our
// 	// selector. They will be orphaned by ClaimStatefulSets().
// 	ssList, err := c.statefulSetLister.StatefulSets(r.Namespace).List(labels.Everything())
// 	if err != nil {
// 		return nil, err
// 	}

// 	statefulSetSelector, err := metav1.LabelSelectorAsSelector(r.Spec.Selector)
// 	if err != nil {
// 		return nil, fmt.Errorf("rollout %s/%s has invalid label selector: %v", r.Namespace, r.Name, err)
// 	}

// }

func (c *rolloutContext) ComputePodHash(rollout *v1alpha1.Rollout) (string, error) {
	podHash := hash.ComputePodTemplateHash(&rollout.Spec.Template, rollout.Status.CollisionCount)
	return podHash, nil
}

func (c *rolloutContext) findMatchingStatefulSets(rollout *v1alpha1.Rollout) ([]*appsv1.StatefulSet, error) {
	// List all StatefulSets to find those we own but that no longer match our
	// selector. They will be orphaned by ClaimStatefulSets().
	ssList, err := c.statefulSetLister.StatefulSets(rollout.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	statefulSetSelector, err := metav1.LabelSelectorAsSelector(rollout.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("rollout %s/%s has invalid label selector: %v", rollout.Namespace, rollout.Name, err)
	}

	var matchingStatefulSets []*appsv1.StatefulSet
	for i := range ssList {
		ss := ssList[i]
		if metav1.IsControlledBy(ss, rollout) &&
			statefulSetSelector.Matches(labels.Set(ss.Spec.Selector.MatchLabels)) {
			matchingStatefulSets = append(matchingStatefulSets, ss)
		}
	}
	c.statefulsetRolloutContext.matchedStatefulsets = matchingStatefulSets
	return matchingStatefulSets, nil

}

func (c *rolloutContext) checkIfStatefulSetExists(rollout *v1alpha1.Rollout) bool {
	ctx := context.TODO()
	statefulSetName := rollout.Status.StatefulSetStatus.Name
	_, err := c.kubeclientset.AppsV1().StatefulSets(rollout.Namespace).Get(ctx, statefulSetName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

func (c *rolloutContext) CreateStatefulSet(rollout *v1alpha1.Rollout) error {
	if c.checkIfStatefulSetExists(rollout) {
		return nil
	}

	ctx := context.TODO()

	// Create a new StatefulSet for the Rollout.
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rollout.Status.StatefulSetStatus.Name,
			Namespace: rollout.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(rollout, v1alpha1.SchemeGroupVersion.WithKind("Rollout")),
			},
		},

		Spec: appsv1.StatefulSetSpec{
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
			Replicas: rollout.Spec.Replicas,
			Selector: rollout.Spec.Selector,
			Template: rollout.Spec.Template,
		},
	}
	ss, err := c.kubeclientset.AppsV1().StatefulSets(rollout.Namespace).Create(ctx, statefulSet, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	rollout.Status.StatefulSetStatus.Name = ss.Name
	rollout.Status.Phase = v1alpha1.RolloutPhaseProgressing

	return nil
}

func (c *rolloutContext) findMatchingControllerRevisions(rollout *v1alpha1.Rollout) ([]*appsv1.ControllerRevision, error) {
	// List all ControllerRevisions to find those we own but that no longer match our
	// selector. They will be orphaned by ClaimControllerRevisions().
	crList, err := c.controllerRevisionLister.ControllerRevisions(rollout.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	controllerRevisionSelector, err := metav1.LabelSelectorAsSelector(rollout.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("rollout %s/%s has invalid label selector: %v", rollout.Namespace, rollout.Name, err)
	}

	var matchingControllerRevisions []*appsv1.ControllerRevision
	for i := range crList {
		cr := crList[i]
		if metav1.IsControlledBy(cr, rollout) &&
			controllerRevisionSelector.Matches(labels.Set(rollout.Spec.Selector.MatchLabels)) {
			matchingControllerRevisions = append(matchingControllerRevisions, cr)
		}
	}
	c.statefulsetRolloutContext.matchedControllerRevisions = matchingControllerRevisions
	return matchingControllerRevisions, nil
}

func (c *rolloutContext) Rollout(rollout *v1alpha1.Rollout) error {
	// check all the conditions of the rollout
	err := c.checkPausedConditions()
	if err != nil {
		return err
	}

	if c.statefulsetRolloutContext == nil {

	}

	ss, err := c.findMatchingStatefulSets(rollout)
	if err != nil {
		return err
	}
	if len(ss) == 0 {
		err := c.CreateStatefulSet(rollout)
		if err != nil {
			return err
		}
	}

	return nil
}
