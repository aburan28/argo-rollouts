package rollout

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
