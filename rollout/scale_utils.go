package rollout

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *rolloutContext) scaleDeployment(targetScale *int32) error {
	deploymentName := c.rollout.Spec.WorkloadRef.Name
	namespace := c.rollout.Namespace
	deployment, err := c.kubeclientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		c.log.Warnf("Failed to fetch deployment %s: %s", deploymentName, err.Error())
		return err
	}

	var newReplicasCount int32
	if *targetScale < 0 {
		newReplicasCount = 0
	} else {
		newReplicasCount = *targetScale
	}
	if newReplicasCount == *deployment.Spec.Replicas {
		return nil
	}
	c.log.Infof("Scaling deployment %s to %d replicas", deploymentName, newReplicasCount)
	*deployment.Spec.Replicas = newReplicasCount

	_, err = c.kubeclientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		c.log.Warnf("Failed to update deployment %s: %s", deploymentName, err.Error())
		return err
	}
	return nil
}

func (c *rolloutContext) scaleStatefulSet(targetScale *int32) error {
	// this scales the statefulset to the target scale
	statefulSetName := c.rollout.Spec.WorkloadRef.Name
	namespace := c.rollout.Namespace
	statefulSet, err := c.kubeclientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		c.log.Warnf("Failed to fetch statefulset %s: %s", statefulSetName, err.Error())
		return err
	}

	var newReplicasCount int32
	if *targetScale < 0 {
		newReplicasCount = 0
	} else {
		newReplicasCount = *targetScale
	}
	if newReplicasCount == *statefulSet.Spec.Replicas {
		return nil
	}

	c.log.Infof("Scaling statefulset %s to %d replicas", statefulSetName, newReplicasCount)
	*statefulSet.Spec.Replicas = newReplicasCount

	_, err = c.kubeclientset.AppsV1().StatefulSets(namespace).Update(context.TODO(), statefulSet, metav1.UpdateOptions{})
	if err != nil {
		c.log.Warnf("Failed to update statefulset %s: %s", statefulSetName, err.Error())
		return err
	}

	return nil

}
