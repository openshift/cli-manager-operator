package operator

import (
	"context"
	"fmt"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/operator/events"

	operatorconfigclientv1 "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned/typed/climanager/v1"
	operatorclientinformers "github.com/openshift/cli-manager-operator/pkg/generated/informers/externalversions/climanager/v1"
	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
)

type TargetConfigReconciler struct {
	ctx              context.Context
	targetImage      string
	operatorClient   operatorconfigclientv1.ClimanagersV1Interface
	cliManagerClient *operatorclient.CLIManagerClient
	kubeClient       kubernetes.Interface
	eventRecorder    events.Recorder
	queue            workqueue.RateLimitingInterface
}

func NewTargetConfigReconciler(
	ctx context.Context,
	targetImage string,
	operatorConfigClient operatorconfigclientv1.ClimanagersV1Interface,
	operatorClientInformer operatorclientinformers.CLIManagerInformer,
	cliManagerClient *operatorclient.CLIManagerClient,
	kubeClient kubernetes.Interface,
	eventRecorder events.Recorder,
) *TargetConfigReconciler {
	c := &TargetConfigReconciler{
		ctx:              ctx,
		operatorClient:   operatorConfigClient,
		cliManagerClient: cliManagerClient,
		kubeClient:       kubeClient,
		eventRecorder:    eventRecorder,
		queue:            workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{Name: "TargetConfigReconciler"}),
		targetImage:      targetImage,
	}

	operatorClientInformer.Informer().AddEventHandler(c.eventHandler())

	return c
}

func (c *TargetConfigReconciler) sync() error {
	return nil
}

// Run starts the kube-scheduler and blocks until stopCh is closed.
func (c *TargetConfigReconciler) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting TargetConfigReconciler")
	defer klog.Infof("Shutting down TargetConfigReconciler")

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *TargetConfigReconciler) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *TargetConfigReconciler) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

// eventHandler queues the operator to check spec and status
func (c *TargetConfigReconciler) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}
