package statefulrollout

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	smiclientset "github.com/servicemeshinterface/smi-sdk-go/pkg/gen/client/split/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubectl/pkg/cmd/util"

	"github.com/argoproj/argo-rollouts/controller/metrics"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	clientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	informers "github.com/argoproj/argo-rollouts/pkg/client/informers/externalversions/rollouts/v1alpha1"
	controllerutil "github.com/argoproj/argo-rollouts/utils/controller"
	ingressutil "github.com/argoproj/argo-rollouts/utils/ingress"
	logutil "github.com/argoproj/argo-rollouts/utils/log"
	"github.com/argoproj/argo-rollouts/utils/record"
	unstructuredutil "github.com/argoproj/argo-rollouts/utils/unstructured"
)

const (
	// statefulRolloutIndexName is the index by which Stateful rollout resources are cached
	statefulRolloutIndexName = "byStatefulRollout"
)

// ControllerConfig describes the data required to instantiate a new ingress controller
type ControllerConfig struct {
	Namespace              string
	KubeClientSet          kubernetes.Interface
	ArgoProjClientset      clientset.Interface
	DynamicClientSet       dynamic.Interface
	StatefulRolloutWrapper StatefulRolloutWrapper
	// RefResolver                     TemplateRefResolver
	SmiClientSet                    smiclientset.Interface
	ExperimentInformer              informers.ExperimentInformer
	AnalysisRunInformer             informers.AnalysisRunInformer
	AnalysisTemplateInformer        informers.AnalysisTemplateInformer
	ClusterAnalysisTemplateInformer informers.ClusterAnalysisTemplateInformer
	ReplicaSetInformer              appsinformers.ReplicaSetInformer
	StatefulSetInformer             appsinformers.StatefulSetInformer
	ControllerRevisionInformer      appsinformers.ControllerRevisionInformer
	ServicesInformer                coreinformers.ServiceInformer
	RolloutsInformer                informers.RolloutInformer
	IstioPrimaryDynamicClient       dynamic.Interface
	IstioVirtualServiceInformer     cache.SharedIndexInformer
	IstioDestinationRuleInformer    cache.SharedIndexInformer
	ResyncPeriod                    time.Duration
	RolloutWorkQueue                workqueue.RateLimitingInterface
	ServiceWorkQueue                workqueue.RateLimitingInterface
	IngressWorkQueue                workqueue.RateLimitingInterface
	MetricsServer                   *metrics.MetricsServer
	Recorder                        record.EventRecorder
}

// Controller describes an stateful rollout controller
type Controller struct {
	client                 kubernetes.Interface
	rolloutsIndexer        cache.Indexer
	statefulRolloutWorqeue workqueue.RateLimitingInterface
	ingressWorkqueue       workqueue.RateLimitingInterface
	statefulRolloutWrapper StatefulRolloutWrapper
	metricServer           *metrics.MetricsServer
	enqueueRollout         func(obj any)
}

type StatefulRolloutWrapper interface {
	GetCached(namespace, name string) (*ingressutil.Ingress, error)
	Update(ctx context.Context, namespace string, ingress *ingressutil.Ingress) (*ingressutil.Ingress, error)
}

// NewController returns a new stateful rollout controller
func NewController(cfg ControllerConfig) *Controller {

	controller := &Controller{
		client:          cfg.KubeClientSet,
		rolloutsIndexer: cfg.RolloutsInformer.Informer().GetIndexer(),

		metricServer: cfg.MetricsServer,
	}

	util.CheckErr(cfg.RolloutsInformer.Informer().AddIndexers(cache.Indexers{
		statefulRolloutIndexName: func(obj any) ([]string, error) {
			if ro := unstructuredutil.ObjectToRollout(obj); ro != nil {
				return ingressutil.GetRolloutIngressKeys(ro), nil
			}
			return []string{}, nil
		},
	}))

	cfg.StatefulRolloutWrapper.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			controllerutil.Enqueue(obj, cfg.IngressWorkQueue)
		},
		UpdateFunc: func(oldObj, newObj any) {
			controllerutil.Enqueue(newObj, cfg.IngressWorkQueue)
		},
		DeleteFunc: func(obj any) {
			controllerutil.Enqueue(obj, cfg.IngressWorkQueue)
		},
	})
	controller.enqueueRollout = func(obj any) {
		controllerutil.EnqueueRateLimited(obj, cfg.RolloutWorkQueue)
	}

	return controller
}

// Run starts the controller threads
func (c *Controller) Run(ctx context.Context, threadiness int) error {
	log.Info("Starting Ingress workers")
	wg := sync.WaitGroup{}
	for i := 0; i < threadiness; i++ {
		wg.Add(1)
		go wait.Until(func() {
			controllerutil.RunWorker(ctx, c.ingressWorkqueue, logutil.IngressKey, c.syncIngress, c.metricServer)
			wg.Done()
			log.Debug("Ingress worker has stopped")
		}, time.Second, ctx.Done())
	}

	log.Info("Started Ingress workers")
	<-ctx.Done()
	wg.Wait()
	log.Info("All ingress workers have stopped")

	return nil
}

// syncIngress queues all rollouts referencing the Ingress for reconciliation
func (c *Controller) syncIngress(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	ingress, err := c.statefulRolloutWrapper.GetCached(namespace, name)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Unknown error occurred
			return err
		}

		if !strings.HasSuffix(name, ingressutil.CanaryIngressSuffix) {
			// a primary ingress was deleted, simply ignore the event
			log.WithField(logutil.IngressKey, key).Warn("primary ingress has been deleted")
		}
		return nil
	}
	rollouts, err := c.getStatefulRolloutsByService(ingress.GetNamespace(), ingress.GetName())
	if err != nil {
		return nil
	}
	fmt.Println(rollouts)

}

func hasClass(classes []string, class string) bool {
	for _, str := range classes {
		if str == class {
			return true
		}
	}
	return false
}

// func (c *Controller) syncNginxIngress(name, namespace string, rollouts []*v1alpha1.Rollout) error {
// 	for i := range rollouts {
// 		// reconciling the Rollout will ensure the canaryIngress is updated or created
// 		c.enqueueRollout(rollouts[i])
// 	}
// 	return nil
// }

// getStatefulRolloutsByService returns all rollouts which are referencing specified ingress
func (c *Controller) getStatefulRolloutsByService(namespace string, ingressName string) ([]*v1alpha1.Rollout, error) {
	objs, err := c.rolloutsIndexer.ByIndex(statefulRolloutIndexName, fmt.Sprintf("%s/%s", namespace, ingressName))
	if err != nil {
		return nil, err
	}
	var rollouts []*v1alpha1.Rollout
	for _, obj := range objs {
		if ro := unstructuredutil.ObjectToRollout(obj); ro != nil {
			rollouts = append(rollouts, ro)
		}
	}
	return rollouts, nil
}
