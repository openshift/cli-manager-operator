package operator

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/library-go/pkg/controller"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/cli-manager-operator/bindata"
	climanagerv1 "github.com/openshift/cli-manager-operator/pkg/apis/climanager/v1"
	operatorconfigclientv1 "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned/typed/climanager/v1"
	operatorclientinformers "github.com/openshift/cli-manager-operator/pkg/generated/informers/externalversions/climanager/v1"
	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
)

type TargetConfigReconciler struct {
	ctx              context.Context
	targetImage      string
	operatorClient   operatorconfigclientv1.ClimanagersV1Interface
	dynamicClient    dynamic.Interface
	routeCLient      routev1client.RouteV1Interface
	cliManagerClient *operatorclient.CLIManagerClient
	kubeClient       kubernetes.Interface
	eventRecorder    events.Recorder
	queue            workqueue.RateLimitingInterface

	insecureHTTP bool
}

func NewTargetConfigReconciler(
	ctx context.Context,
	targetImage string,
	operatorConfigClient operatorconfigclientv1.ClimanagersV1Interface,
	routeCLient routev1client.RouteV1Interface,
	operatorClientInformer operatorclientinformers.CliManagerInformer,
	kubeInformersForNamespaces v1helpers.KubeInformersForNamespaces,
	cliManagerClient *operatorclient.CLIManagerClient,
	dynamicClient dynamic.Interface,
	kubeClient kubernetes.Interface,
	insecureHTTP bool,
	eventRecorder events.Recorder,
) (*TargetConfigReconciler, error) {
	c := &TargetConfigReconciler{
		ctx:              ctx,
		operatorClient:   operatorConfigClient,
		routeCLient:      routeCLient,
		cliManagerClient: cliManagerClient,
		dynamicClient:    dynamicClient,
		kubeClient:       kubeClient,
		eventRecorder:    eventRecorder,
		queue:            workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{Name: "TargetConfigReconciler"}),
		targetImage:      targetImage,
		insecureHTTP:     insecureHTTP,
	}

	operatorClientInformer.Informer().AddEventHandler(c.eventHandler())

	// Watch NetworkPolicies in operator namespace for immediate reconciliation on deletion or modification.
	// Only watches operator namespace because all operand resources (including NetworkPolicies)
	// are created in the same namespace as the CliManager CR (always operator namespace).
	_, err := kubeInformersForNamespaces.InformersFor(operatorclient.OperatorNamespace).Networking().V1().NetworkPolicies().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.queue.Add(workQueueKey)
		},
		DeleteFunc: func(obj interface{}) {
			c.queue.Add(workQueueKey)
		},
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *TargetConfigReconciler) sync() error {
	cliManager, err := c.operatorClient.CliManagers(operatorclient.OperatorNamespace).Get(c.ctx, operatorclient.OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		klog.ErrorS(err, "unable to get operator configuration", "namespace", operatorclient.OperatorNamespace, "openshift-cli-manager", operatorclient.OperatorConfigName)
		return err
	}

	// Create allow policy first
	_, _, err = c.manageOperandNetworkPolicyAllow(cliManager)
	if err != nil {
		klog.Errorf("unable to manage operand allow network policy err: %v", err)
		return err
	}

	// Handle default-deny policy: only create if allow policy exists.
	// This prevents traffic blocking if the allow policy is accidentally deleted.
	// If allow policy is missing, delete default-deny so traffic continues.
	allowExists := c.checkNetworkPolicyExists(cliManager.Namespace, "allow-all-egress-and-metrics-ingress-operand")
	if allowExists {
		_, _, err = c.manageOperandDefaultDenyNetworkPolicy(cliManager)
		if err != nil {
			klog.Errorf("unable to manage operand default-deny network policy err: %v", err)
			return err
		}
	} else {
		// Allow policy doesn't exist (creation failed or was deleted), ensure default-deny is also gone
		if err := c.deleteOperandDefaultDenyNetworkPolicy(cliManager); err != nil {
			klog.ErrorS(err, "failed to delete operand default-deny policy")
			// Don't return error - this is cleanup, not critical
		}
	}

	_, _, err = c.manageClusterRole(cliManager)
	if err != nil {
		klog.Errorf("unable to manage cluster role err: %v", err)
		return err
	}

	_, _, err = c.manageClusterRoleBinding(cliManager)
	if err != nil {
		klog.Errorf("unable to manage cluster role binding err: %v", err)
		return err
	}

	_, _, err = c.manageRole(cliManager)
	if err != nil {
		klog.Errorf("unable to manage cluster role err: %v", err)
		return err
	}

	_, _, err = c.manageRoleBinding(cliManager)
	if err != nil {
		klog.Errorf("unable to manage cluster role binding err: %v", err)
		return err
	}

	deployment, _, err := c.manageDeployments(cliManager)
	if err != nil {
		klog.Errorf("unable to manage deployment err: %v", err)
		return err
	}

	_, _, err = c.manageRoute(cliManager)
	if err != nil {
		klog.Errorf("unable to manage route err: %v", err)
		return err
	}

	_, _, err = c.manageService(cliManager)
	if err != nil {
		klog.Errorf("unable to manage service err: %v", err)
		return err
	}

	_, _, err = c.manageServiceMetrics(cliManager)
	if err != nil {
		klog.Errorf("unable to manage service metrics err: %v", err)
		return err
	}

	_, _, err = c.manageServiceAccount(cliManager)
	if err != nil {
		klog.Errorf("unable to manage service account err: %v", err)
		return err
	}

	_, err = c.manageServiceMonitor(cliManager)
	if err != nil {
		klog.Errorf("unable to manage service monitor err: %v", err)
		return err
	}

	_, _, err = v1helpers.UpdateStatus(c.ctx, c.cliManagerClient, func(status *operatorv1.OperatorStatus) error {
		resourcemerge.SetDeploymentGeneration(&status.Generations, deployment)
		return nil
	})

	return err
}

func (c *TargetConfigReconciler) manageRole(cliManager *climanagerv1.CliManager) (*rbacv1.Role, bool, error) {
	required := resourceread.ReadRoleV1OrDie(bindata.MustAsset("assets/cli-manager/role.yaml"))
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyRole(c.ctx, c.kubeClient.RbacV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageRoleBinding(cliManager *climanagerv1.CliManager) (*rbacv1.RoleBinding, bool, error) {
	required := resourceread.ReadRoleBindingV1OrDie(bindata.MustAsset("assets/cli-manager/rolebinding.yaml"))
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyRoleBinding(c.ctx, c.kubeClient.RbacV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageClusterRole(cliManager *climanagerv1.CliManager) (*rbacv1.ClusterRole, bool, error) {
	required := resourceread.ReadClusterRoleV1OrDie(bindata.MustAsset("assets/cli-manager/clusterrole.yaml"))
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyClusterRole(c.ctx, c.kubeClient.RbacV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageClusterRoleBinding(cliManager *climanagerv1.CliManager) (*rbacv1.ClusterRoleBinding, bool, error) {
	required := resourceread.ReadClusterRoleBindingV1OrDie(bindata.MustAsset("assets/cli-manager/clusterrolebinding.yaml"))
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyClusterRoleBinding(c.ctx, c.kubeClient.RbacV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageRoute(cliManager *climanagerv1.CliManager) (*routev1.Route, bool, error) {
	required := resourceread.ReadRouteV1OrDie(bindata.MustAsset("assets/cli-manager/route.yaml"))
	required.Namespace = cliManager.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}

	if c.insecureHTTP {
		required.Spec.TLS.InsecureEdgeTerminationPolicy = routev1.InsecureEdgeTerminationPolicyAllow
	}

	controller.EnsureOwnerRef(required, ownerReference)

	return applyRoute(c.ctx, c.routeCLient, c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageService(cliManager *climanagerv1.CliManager) (*v1.Service, bool, error) {
	required := resourceread.ReadServiceV1OrDie(bindata.MustAsset("assets/cli-manager/service.yaml"))
	required.Namespace = cliManager.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyService(c.ctx, c.kubeClient.CoreV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageServiceMetrics(cliManager *climanagerv1.CliManager) (*v1.Service, bool, error) {
	required := resourceread.ReadServiceV1OrDie(bindata.MustAsset("assets/cli-manager/servicemetrics.yaml"))
	required.Namespace = cliManager.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyService(c.ctx, c.kubeClient.CoreV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageServiceAccount(cliManager *climanagerv1.CliManager) (*v1.ServiceAccount, bool, error) {
	required := resourceread.ReadServiceAccountV1OrDie(bindata.MustAsset("assets/cli-manager/serviceaccount.yaml"))
	required.Namespace = cliManager.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyServiceAccount(c.ctx, c.kubeClient.CoreV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageServiceMonitor(cliManager *climanagerv1.CliManager) (bool, error) {
	required := resourceread.ReadUnstructuredOrDie(bindata.MustAsset("assets/cli-manager/servicemonitor.yaml"))
	required.SetNamespace(cliManager.Namespace)

	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	// Bindata has a hardcoded namespace in serverName. SetNestedField cannot address
	// slice indexes (spec.endpoints[0]), so update tlsConfig.serverName via NestedSlice.
	serverName := fmt.Sprintf("metrics.%s.svc", cliManager.Namespace)
	if err := setServiceMonitorServerName(required, serverName); err != nil {
		return false, err
	}

	_, changed, err := resourceapply.ApplyKnownUnstructured(c.ctx, c.dynamicClient, c.eventRecorder, required)
	return changed, err
}

func setServiceMonitorServerName(sm *unstructured.Unstructured, serverName string) error {
	endpoints, found, err := unstructured.NestedSlice(sm.Object, "spec", "endpoints")
	if err != nil {
		return fmt.Errorf("failed to read ServiceMonitor endpoints: %w", err)
	}
	if !found || len(endpoints) == 0 {
		return fmt.Errorf("ServiceMonitor spec.endpoints is empty")
	}
	ep, ok := endpoints[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("ServiceMonitor spec.endpoints[0] is not an object")
	}
	tlsConfig, ok := ep["tlsConfig"].(map[string]interface{})
	if !ok || tlsConfig == nil {
		return fmt.Errorf("ServiceMonitor spec.endpoints[0].tlsConfig is missing or not an object")
	}
	tlsConfig["serverName"] = serverName
	ep["tlsConfig"] = tlsConfig
	endpoints[0] = ep
	if err := unstructured.SetNestedSlice(sm.Object, endpoints, "spec", "endpoints"); err != nil {
		return fmt.Errorf("failed to set ServiceMonitor serverName %q: %w", serverName, err)
	}
	return nil
}

func (c *TargetConfigReconciler) manageDeployments(cliManager *climanagerv1.CliManager) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentV1OrDie(bindata.MustAsset("assets/cli-manager/deployment.yaml"))
	required.Name = operatorclient.OperandName
	required.Namespace = cliManager.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	if c.targetImage != "" {
		images := map[string]string{
			"${IMAGE}": c.targetImage,
		}

		for i := range required.Spec.Template.Spec.Containers {
			for pat, img := range images {
				if required.Spec.Template.Spec.Containers[i].Image == pat {
					required.Spec.Template.Spec.Containers[i].Image = img
					break
				}
			}
		}
	}

	switch cliManager.Spec.LogLevel {
	case operatorv1.Normal:
		required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", 2))
	case operatorv1.Debug:
		required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", 4))
	case operatorv1.Trace:
		required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", 6))
	case operatorv1.TraceAll:
		required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", 8))
	default:
		required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", 2))
	}

	if c.insecureHTTP {
		required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--serve-artifacts-in-http"))
	}

	return resourceapply.ApplyDeployment(
		c.ctx,
		c.kubeClient.AppsV1(),
		c.eventRecorder,
		required,
		resourcemerge.ExpectedDeploymentGeneration(required, cliManager.Status.Generations))
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

// manageOperandNetworkPolicyAllow manages the allow network policy for the operand pods
func (c *TargetConfigReconciler) manageOperandNetworkPolicyAllow(cliManager *climanagerv1.CliManager) (*networkingv1.NetworkPolicy, bool, error) {
	required := resourceread.ReadNetworkPolicyV1OrDie(bindata.MustAsset("assets/cli-manager/networkpolicy-operand-allow.yaml"))
	required.Namespace = cliManager.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyNetworkPolicy(c.ctx, c.kubeClient.NetworkingV1(), c.eventRecorder, required, resourceapply.NewResourceCache())
}

// manageOperandDefaultDenyNetworkPolicy manages the default-deny network policy for operand pods only
func (c *TargetConfigReconciler) manageOperandDefaultDenyNetworkPolicy(cliManager *climanagerv1.CliManager) (*networkingv1.NetworkPolicy, bool, error) {
	required := resourceread.ReadNetworkPolicyV1OrDie(bindata.MustAsset("assets/cli-manager/networkpolicy-operand-default-deny.yaml"))
	required.Namespace = cliManager.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "CliManager",
		Name:       cliManager.Name,
		UID:        cliManager.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyNetworkPolicy(c.ctx, c.kubeClient.NetworkingV1(), c.eventRecorder, required, resourceapply.NewResourceCache())
}

// checkNetworkPolicyExists checks if a network policy exists in a given namespace
func (c *TargetConfigReconciler) checkNetworkPolicyExists(namespace, policyName string) bool {
	_, err := c.kubeClient.NetworkingV1().NetworkPolicies(namespace).Get(c.ctx, policyName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		klog.V(4).InfoS("unexpected error checking network policy existence", "namespace", namespace, "name", policyName, "error", err)
	}
	return err == nil
}

// deleteOperandDefaultDenyNetworkPolicy deletes the operand default-deny network policy
func (c *TargetConfigReconciler) deleteOperandDefaultDenyNetworkPolicy(cliManager *climanagerv1.CliManager) error {
	required := resourceread.ReadNetworkPolicyV1OrDie(bindata.MustAsset("assets/cli-manager/networkpolicy-operand-default-deny.yaml"))
	required.Namespace = cliManager.Namespace
	_, _, err := resourceapply.DeleteNetworkPolicy(c.ctx, c.kubeClient.NetworkingV1(), c.eventRecorder, required)
	return err
}

// eventHandler queues the operator to check spec and status
func (c *TargetConfigReconciler) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}

func applyRoute(ctx context.Context, client routev1client.RoutesGetter, recorder events.Recorder, required *routev1.Route) (*routev1.Route, bool, error) {
	existing, err := client.Routes(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.Routes(requiredCopy.Namespace).Create(ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*routev1.Route), metav1.CreateOptions{})
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	existingCopy := existing.DeepCopy()
	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	specSame := equality.Semantic.DeepEqual(existingCopy.Spec, required.Spec)

	if specSame && !*modified {
		klog.V(4).Infof("%s route exists and is in the correct state", existingCopy.ObjectMeta.Name)
		return existingCopy, false, nil
	}

	existingCopy.Spec = required.Spec
	actual, err := client.Routes(required.Namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})
	return actual, true, err
}
