package operator

import (
	"context"
	"os"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/loglevel"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	operatorconfigclient "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned"
	operatorclientinformers "github.com/openshift/cli-manager-operator/pkg/generated/informers/externalversions"
	"github.com/openshift/cli-manager-operator/pkg/operator/configobservation/configobservercontroller"
	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
	"github.com/openshift/cli-manager-operator/pkg/operator/resourcesynccontroller"
)

const (
	workQueueKey = "key"
)

func RunOperator(ctx context.Context, cc *controllercmd.ControllerContext) error {
	kubeClient, err := kubernetes.NewForConfig(cc.ProtoKubeConfig)
	if err != nil {
		return err
	}

	openshiftConfigClient, err := configv1client.NewForConfig(cc.KubeConfig)
	if err != nil {
		return err
	}

	configInformers := configv1informers.NewSharedInformerFactory(openshiftConfigClient, 10*time.Minute)

	dynamicClient, err := dynamic.NewForConfig(cc.ProtoKubeConfig)
	if err != nil {
		return err
	}

	kubeInformersForNamespaces := v1helpers.NewKubeInformersForNamespaces(kubeClient,
		"",
		operatorclient.OperatorNamespace,
	)

	operatorConfigClient, err := operatorconfigclient.NewForConfig(cc.KubeConfig)
	if err != nil {
		return err
	}
	operatorConfigInformers := operatorclientinformers.NewSharedInformerFactory(operatorConfigClient, 10*time.Minute)
	cliManagerClient := &operatorclient.CLIManagerClient{
		Ctx:            ctx,
		SharedInformer: operatorConfigInformers.Climanagers().V1().CLIManagers().Informer(),
		OperatorClient: operatorConfigClient.ClimanagersV1(),
	}

	resourceSyncController, err := resourcesynccontroller.NewResourceSyncController(
		cliManagerClient,
		kubeInformersForNamespaces,
		kubeClient,
		cc.EventRecorder,
	)
	if err != nil {
		return err
	}

	configObserver := configobservercontroller.NewConfigObserver(
		cliManagerClient,
		kubeInformersForNamespaces,
		configInformers,
		resourceSyncController,
		cc.EventRecorder,
	)

	targetConfigReconciler := NewTargetConfigReconciler(
		ctx,
		os.Getenv("RELATED_IMAGE_OPERAND_IMAGE"),
		operatorConfigClient.ClimanagersV1(),
		operatorConfigInformers.Climanagers().V1().CLIManagers(),
		cliManagerClient,
		kubeClient,
		dynamicClient,
		configInformers,
		cc.EventRecorder,
	)

	logLevelController := loglevel.NewClusterOperatorLoggingController(cliManagerClient, cc.EventRecorder)

	klog.Infof("Starting informers")
	operatorConfigInformers.Start(ctx.Done())
	configInformers.Start(ctx.Done())
	kubeInformersForNamespaces.Start(ctx.Done())

	klog.Infof("Starting log level controller")
	go logLevelController.Run(ctx, 1)
	klog.Infof("Starting target config reconciler")
	go targetConfigReconciler.Run(1, ctx.Done())

	go resourceSyncController.Run(ctx, 1)
	go configObserver.Run(ctx, 1)

	<-ctx.Done()
	return nil
}
