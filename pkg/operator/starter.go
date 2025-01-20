package operator

import (
	"context"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"os"
	"time"

	"k8s.io/klog/v2"

	operatorconfigclient "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned"
	operatorclientinformers "github.com/openshift/cli-manager-operator/pkg/generated/informers/externalversions"
	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/loglevel"
)

const (
	workQueueKey = "key"
)

var ServeArtifactAsHttp bool

// RunOperator runs the operator
func RunOperator(ctx context.Context, cc *controllercmd.ControllerContext) error {
	kubeClient, err := kubernetes.NewForConfig(cc.ProtoKubeConfig)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(cc.ProtoKubeConfig)
	if err != nil {
		return err
	}

	operatorConfigClient, err := operatorconfigclient.NewForConfig(cc.KubeConfig)
	if err != nil {
		return err
	}
	operatorConfigInformers := operatorclientinformers.NewSharedInformerFactory(operatorConfigClient, 10*time.Minute)
	cliManagerClient := &operatorclient.CLIManagerClient{
		Ctx:            ctx,
		SharedInformer: operatorConfigInformers.Climanagers().V1().CliManagers().Informer(),
		OperatorClient: operatorConfigClient.ClimanagersV1(),
	}

	routeClient, err := routev1client.NewForConfig(cc.KubeConfig)
	if err != nil {
		return err
	}

	targetConfigReconciler := NewTargetConfigReconciler(
		ctx,
		os.Getenv("RELATED_IMAGE_OPERAND_IMAGE"),
		operatorConfigClient.ClimanagersV1(),
		routeClient,
		operatorConfigInformers.Climanagers().V1().CliManagers(),
		cliManagerClient,
		dynamicClient,
		kubeClient,
		ServeArtifactAsHttp,
		cc.EventRecorder,
	)

	logLevelController := loglevel.NewClusterOperatorLoggingController(cliManagerClient, cc.EventRecorder)

	klog.Infof("Starting informers")
	operatorConfigInformers.Start(ctx.Done())

	klog.Infof("Starting log level controller")
	go logLevelController.Run(ctx, 1)
	klog.Infof("Starting target config reconciler")
	go targetConfigReconciler.Run(1, ctx.Done())

	<-ctx.Done()
	return nil
}
