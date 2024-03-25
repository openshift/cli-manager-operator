package operator

import (
	"context"
	"time"

	"k8s.io/klog/v2"

	"k8s.io/client-go/kubernetes"

	operatorconfigclient "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned"
	operatorclientinformers "github.com/openshift/cli-manager-operator/pkg/generated/informers/externalversions"
	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/loglevel"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	workQueueKey = "key"
)

func RunOperator(ctx context.Context, cc *controllercmd.ControllerContext) error {
	kubeClient, err := kubernetes.NewForConfig(cc.ProtoKubeConfig)
	if err != nil {
		return err
	}

	/*dynamicClient, err := dynamic.NewForConfig(cc.ProtoKubeConfig)
	if err != nil {
		return err
	}*/

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

	/*targetConfigReconciler, err := NewTargetConfigReconciler(
		ctx,
		operatorConfigClient.ClimanagersV1(),
		operatorConfigInformers.Climanagers().V1().CLIManagers(),
		kubeInformersForNamespaces,
		cliManagerClient,
		kubeClient,
		dynamicClient,
		cc.EventRecorder,
	)
	if err != nil {
		return err
	}*/

	logLevelController := loglevel.NewClusterOperatorLoggingController(cliManagerClient, cc.EventRecorder)

	klog.Infof("Starting informers")
	operatorConfigInformers.Start(ctx.Done())
	kubeInformersForNamespaces.Start(ctx.Done())

	klog.Infof("Starting log level controller")
	go logLevelController.Run(ctx, 1)
	klog.Infof("Starting target config reconciler")
	//go targetConfigReconciler.Run(1, ctx.Done())

	<-ctx.Done()
	return nil
}
