package operator

import (
	"context"
	"fmt"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	"github.com/openshift/library-go/pkg/operator/events"

	operatorconfigclientv1 "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned/typed/climanager/v1"
	operatorclientinformers "github.com/openshift/cli-manager-operator/pkg/generated/informers/externalversions/climanager/v1"
	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
)

const DefaultImage = "quay.io/openshift/origin-cli-manager:latest"

type TargetConfigReconciler struct {
	ctx                 context.Context
	targetImagePullSpec string
	operatorClient      operatorconfigclientv1.ClimanagersV1Interface
	cliManagerClient    *operatorclient.CLIManagerClient
	kubeClient          kubernetes.Interface
	dynamicClient       dynamic.Interface
	eventRecorder       events.Recorder
	queue               workqueue.RateLimitingInterface
}

func NewTargetConfigReconciler(
	ctx context.Context,
	targetImagePullSpec string,
	operatorConfigClient operatorconfigclientv1.ClimanagersV1Interface,
	operatorClientInformer operatorclientinformers.CLIManagerInformer,
	cliManagerClient *operatorclient.CLIManagerClient,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	configInformer configinformers.SharedInformerFactory,
	eventRecorder events.Recorder,
) *TargetConfigReconciler {

	return nil
}

func (c TargetConfigReconciler) sync() error {
	return nil
}

/*func (c *TargetConfigReconciler) manageClusterRole(descheduler *deschedulerv1.KubeDescheduler) (*rbacv1.ClusterRole, bool, error) {
	required := resourceread.ReadClusterRoleV1OrDie(bindata.MustAsset("assets/kube-descheduler/operandclusterrole.yaml"))
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "KubeDescheduler",
		Name:       descheduler.Name,
		UID:        descheduler.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyClusterRole(c.ctx, c.kubeClient.RbacV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageClusterRoleBinding(descheduler *deschedulerv1.KubeDescheduler) (*rbacv1.ClusterRoleBinding, bool, error) {
	required := resourceread.ReadClusterRoleBindingV1OrDie(bindata.MustAsset("assets/kube-descheduler/operandclusterrolebinding.yaml"))
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "KubeDescheduler",
		Name:       descheduler.Name,
		UID:        descheduler.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyClusterRoleBinding(c.ctx, c.kubeClient.RbacV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageRole(descheduler *deschedulerv1.KubeDescheduler) (*rbacv1.Role, bool, error) {
	required := resourceread.ReadRoleV1OrDie(bindata.MustAsset("assets/kube-descheduler/role.yaml"))
	required.Namespace = descheduler.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "KubeDescheduler",
		Name:       descheduler.Name,
		UID:        descheduler.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyRole(c.ctx, c.kubeClient.RbacV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageRoleBinding(descheduler *deschedulerv1.KubeDescheduler) (*rbacv1.RoleBinding, bool, error) {
	required := resourceread.ReadRoleBindingV1OrDie(bindata.MustAsset("assets/kube-descheduler/rolebinding.yaml"))
	required.Namespace = descheduler.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "KubeDescheduler",
		Name:       descheduler.Name,
		UID:        descheduler.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyRoleBinding(c.ctx, c.kubeClient.RbacV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageServiceAccount(descheduler *deschedulerv1.KubeDescheduler) (*v1.ServiceAccount, bool, error) {
	required := resourceread.ReadServiceAccountV1OrDie(bindata.MustAsset("assets/kube-descheduler/operandserviceaccount.yaml"))
	required.Namespace = descheduler.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "KubeDescheduler",
		Name:       descheduler.Name,
		UID:        descheduler.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyServiceAccount(c.ctx, c.kubeClient.CoreV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageService(descheduler *deschedulerv1.KubeDescheduler) (*v1.Service, bool, error) {
	required := resourceread.ReadServiceV1OrDie(bindata.MustAsset("assets/kube-descheduler/service.yaml"))
	required.Namespace = descheduler.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "KubeDescheduler",
		Name:       descheduler.Name,
		UID:        descheduler.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	return resourceapply.ApplyService(c.ctx, c.kubeClient.CoreV1(), c.eventRecorder, required)
}

func (c *TargetConfigReconciler) manageServiceMonitor(descheduler *deschedulerv1.KubeDescheduler) (bool, error) {
	required := resourceread.ReadUnstructuredOrDie(bindata.MustAsset("assets/kube-descheduler/servicemonitor.yaml"))
	_, changed, err := resourceapply.ApplyKnownUnstructured(c.ctx, c.dynamicClient, c.eventRecorder, required)
	return changed, err
}

func (c *TargetConfigReconciler) manageConfigMap(descheduler *deschedulerv1.KubeDescheduler) (*v1.ConfigMap, bool, error) {
	required := resourceread.ReadConfigMapV1OrDie(bindata.MustAsset("assets/kube-descheduler/configmap.yaml"))
	required.Name = descheduler.Name
	required.Namespace = descheduler.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "KubeDescheduler",
		Name:       descheduler.Name,
		UID:        descheduler.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)

	scheduler, err := c.configSchedulerLister.Get("cluster")
	if err != nil {
		return nil, false, err
	}

	// override the included/excluded namespace
	excludedNamespaces := c.protectedNamespaces
	protectedNamespacesSet := sets.NewString(c.protectedNamespaces...)
	includedNamespaces := []string{}
	if descheduler.Spec.ProfileCustomizations != nil {
		if len(descheduler.Spec.ProfileCustomizations.Namespaces.Excluded) > 0 && len(descheduler.Spec.ProfileCustomizations.Namespaces.Included) > 0 {
			return nil, false, fmt.Errorf("It is forbidden to combine both included and excluded namespaces")
		}
		if len(descheduler.Spec.ProfileCustomizations.Namespaces.Included) > 0 {
			for _, ns := range descheduler.Spec.ProfileCustomizations.Namespaces.Included {
				if protectedNamespacesSet.Has(ns) {
					return nil, false, fmt.Errorf("Protected namespace %v included. It is forbidden to include any of the protected namespaces from %v", ns, c.protectedNamespaces)
				}
				includedNamespaces = append(includedNamespaces, ns)
			}
			excludedNamespaces = []string{}
		}
		for _, ns := range descheduler.Spec.ProfileCustomizations.Namespaces.Excluded {
			excludedNamespaces = append(excludedNamespaces, ns)
		}
	}

	// parse whatever profiles are set into their policy representations then merge them into one file
	if len(descheduler.Spec.Profiles) == 0 {
		return nil, false, fmt.Errorf("descheduler should have at least 1 profile enabled")
	}
	profiles := sets.NewString()
	policy := &deschedulerapi.DeschedulerPolicy{}

	// ignore PVC pods by default
	ignorePVCPods := true
	policy.IgnorePVCPods = &ignorePVCPods
	for _, profileName := range descheduler.Spec.Profiles {
		p := bindata.MustAsset("assets/profiles/" + string(profileName) + ".yaml")
		profile := &deschedulerapi.DeschedulerPolicy{}
		if err := yaml.Unmarshal(p, profile); err != nil {
			return nil, false, err
		}

		if err := checkProfileConflicts(profiles, profileName, scheduler); err != nil {
			klog.ErrorS(err, "Profile conflict")
			continue
		}

		// exclude openshift namespaces from descheduling
		for name, strategy := range profile.Strategies {
			if strategy.Params == nil {
				strategy.Params = &deschedulerapi.StrategyParameters{}
			}
			if name == "LowNodeUtilization" {
				if len(includedNamespaces) > 0 {
					// log a warning if user tries to enable ns inclusion with a profile that activates LowNodeUtilization
					klog.Warning("LowNodeUtilization is enabled, however it does not support namespace inclusion. Namespace inclusion will only be considered by other strategies (like RemovePodsHavingTooManyRestarts and PodLifeTime)")
				}

				if descheduler.Spec.ProfileCustomizations != nil && descheduler.Spec.ProfileCustomizations.DevLowNodeUtilizationThresholds != nil {
					if strategy.Params == nil {
						strategy.Params = &deschedulerapi.StrategyParameters{}
					}
					switch *descheduler.Spec.ProfileCustomizations.DevLowNodeUtilizationThresholds {
					case deschedulerv1.LowThreshold:
						strategy.Params.NodeResourceUtilizationThresholds = &deschedulerapi.NodeResourceUtilizationThresholds{
							Thresholds: map[v1.ResourceName]deschedulerapi.Percentage{
								"cpu":    10,
								"memory": 10,
								"pods":   10,
							},
							TargetThresholds: map[v1.ResourceName]deschedulerapi.Percentage{
								"cpu":    30,
								"memory": 30,
								"pods":   30,
							},
						}
					case deschedulerv1.MediumThreshold:
						strategy.Params.NodeResourceUtilizationThresholds = &deschedulerapi.NodeResourceUtilizationThresholds{
							Thresholds: map[v1.ResourceName]deschedulerapi.Percentage{
								"cpu":    20,
								"memory": 20,
								"pods":   20,
							},
							TargetThresholds: map[v1.ResourceName]deschedulerapi.Percentage{
								"cpu":    50,
								"memory": 50,
								"pods":   50,
							},
						}
					case deschedulerv1.HighThreshold:
						strategy.Params.NodeResourceUtilizationThresholds = &deschedulerapi.NodeResourceUtilizationThresholds{
							Thresholds: map[v1.ResourceName]deschedulerapi.Percentage{
								"cpu":    40,
								"memory": 40,
								"pods":   40,
							},
							TargetThresholds: map[v1.ResourceName]deschedulerapi.Percentage{
								"cpu":    70,
								"memory": 70,
								"pods":   70,
							},
						}
					default:
						return nil, false, fmt.Errorf("unknown Descheduler LowNodeUtilization threshold %v, only 'Low', 'Medium' and 'High' are supported", *descheduler.Spec.ProfileCustomizations.DevLowNodeUtilizationThresholds)
					}
				}
			}
			if len(excludedNamespaces) > 0 || len(includedNamespaces) > 0 {
				// LowNodeUtilization can only support namespace exclusion
				if name == "LowNodeUtilization" {
					strategy.Params.Namespaces = &deschedulerapi.Namespaces{
						Exclude: excludedNamespaces,
					}
					// Other strategies support both exclusions and inclusions
				} else {
					strategy.Params.Namespaces = &deschedulerapi.Namespaces{
						Exclude: excludedNamespaces,
						Include: includedNamespaces,
					}
				}
			}

			profile.Strategies[name] = strategy
		}
		mergo.Merge(policy, profile, mergo.WithAppendSlice, mergo.WithOverride)
	}

	// Check for conflicting kube-scheduler config
	if scheduler.Spec.Profile == configv1.HighNodeUtilization &&
		(profiles.Has(string(deschedulerv1.LifecycleAndUtilization)) || profiles.Has(string(deschedulerv1.DevPreviewLongLifecycle))) {
		// force a new deployment so we can scale it to 0
		return nil, true, fmt.Errorf("enabling Descheduler LowNodeUtilization with Scheduler HighNodeUtilization may cause an eviction/scheduling hot loop")
	}

	if descheduler.Spec.ProfileCustomizations != nil {
		// set PodLifetime if non-default
		if descheduler.Spec.ProfileCustomizations.PodLifetime != nil {
			seconds := uint(descheduler.Spec.ProfileCustomizations.PodLifetime.Seconds())
			if _, ok := policy.Strategies["PodLifeTime"]; ok {
				policy.Strategies["PodLifeTime"].Params.PodLifeTime.MaxPodLifeTimeSeconds = &seconds
			}
		}

		// set priority class threshold if customized
		if descheduler.Spec.ProfileCustomizations.ThresholdPriority != nil && descheduler.Spec.ProfileCustomizations.ThresholdPriorityClassName != "" {
			return nil, false, fmt.Errorf("It is invalid to set both .spec.profileCustomizations.thresholdPriority and .spec.profileCustomizations.ThresholdPriorityClassName fields")
		}

		if descheduler.Spec.ProfileCustomizations.ThresholdPriority != nil || descheduler.Spec.ProfileCustomizations.ThresholdPriorityClassName != "" {
			for name, strategy := range policy.Strategies {
				if strategy.Params == nil {
					strategy.Params = &deschedulerapi.StrategyParameters{}
				}
				if descheduler.Spec.ProfileCustomizations.ThresholdPriority != nil {
					priority := *descheduler.Spec.ProfileCustomizations.ThresholdPriority
					policy.Strategies[name].Params.ThresholdPriority = &priority
				}
				if descheduler.Spec.ProfileCustomizations.ThresholdPriorityClassName != "" {
					policy.Strategies[name].Params.ThresholdPriorityClassName = descheduler.Spec.ProfileCustomizations.ThresholdPriorityClassName
				}
			}
		}
	}

	policyBytes, err := yaml.Marshal(policy)
	if err != nil {
		return nil, false, err
	}
	required.Data = map[string]string{"policy.yaml": string(policyBytes)}
	return resourceapply.ApplyConfigMap(c.ctx, c.kubeClient.CoreV1(), c.eventRecorder, required)
}

// checkProfileConflicts ensures that multiple profiles aren't redeclared
// it also checks for various inter-profile conflicts (profiles which should not be enabled simultaneously)
func checkProfileConflicts(profiles sets.String, profileName deschedulerv1.DeschedulerProfile, scheduler *configv1.Scheduler) error {
	if profiles.Has(string(profileName)) {
		return fmt.Errorf("profile %s already declared, ignoring", profileName)
	} else {
		profiles.Insert(string(profileName))
	}

	if profiles.Has(string(deschedulerv1.DevPreviewLongLifecycle)) && profiles.Has(string(deschedulerv1.LifecycleAndUtilization)) {
		return fmt.Errorf("cannot declare %s and %s profiles simultaneously, ignoring", deschedulerv1.DevPreviewLongLifecycle, deschedulerv1.LifecycleAndUtilization)
	}

	if profiles.Has(string(deschedulerv1.SoftTopologyAndDuplicates)) && profiles.Has(string(deschedulerv1.TopologyAndDuplicates)) {
		return fmt.Errorf("cannot declare %s and %s profiles simultaneously, ignoring", deschedulerv1.TopologyAndDuplicates, deschedulerv1.SoftTopologyAndDuplicates)
	}
	return nil
}

func (c *TargetConfigReconciler) manageDeployment(descheduler *deschedulerv1.KubeDescheduler, specAnnotations map[string]string) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentV1OrDie(bindata.MustAsset("assets/kube-descheduler/deployment.yaml"))
	required.Name = operatorclient.OperandName
	required.Namespace = descheduler.Namespace
	ownerReference := metav1.OwnerReference{
		APIVersion: "operator.openshift.io/v1",
		Kind:       "KubeDescheduler",
		Name:       descheduler.Name,
		UID:        descheduler.UID,
	}
	required.OwnerReferences = []metav1.OwnerReference{
		ownerReference,
	}
	controller.EnsureOwnerRef(required, ownerReference)
	replicas := int32(1)
	required.Spec.Replicas = &replicas
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args,
		fmt.Sprintf("--descheduling-interval=%ss", strconv.Itoa(int(*descheduler.Spec.DeschedulingIntervalSeconds))))

	var observedConfig map[string]interface{}
	if err := yaml.Unmarshal(descheduler.Spec.ObservedConfig.Raw, &observedConfig); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal the observedConfig: %v", err)
	}

	cipherSuites, cipherSuitesFound, err := unstructured.NestedStringSlice(observedConfig, "servingInfo", "cipherSuites")
	if err != nil {
		return nil, false, fmt.Errorf("couldn't get the servingInfo.cipherSuites config from observedConfig: %v", err)
	}

	minTLSVersion, minTLSVersionFound, err := unstructured.NestedString(observedConfig, "servingInfo", "minTLSVersion")
	if err != nil {
		return nil, false, fmt.Errorf("couldn't get the servingInfo.minTLSVersion config from observedConfig: %v", err)
	}

	if cipherSuitesFound && len(cipherSuites) > 0 {
		required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--tls-cipher-suites=%s", strings.Join(cipherSuites, ",")))
	}

	if minTLSVersionFound && len(minTLSVersion) > 0 {
		required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--tls-min-version=%s", minTLSVersion))
	}

	required.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name = descheduler.Name

	if len(descheduler.Spec.Mode) > 0 {
		switch descheduler.Spec.Mode {
		case deschedulerv1.Automatic:
			// No additional flags/configuration for now
		case deschedulerv1.Predictive:
			// Run the simulator in the dry mode (metrics are enabled by default)
			required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, "--dry-run=true")
		default:
			return nil, false, fmt.Errorf("descheduler mode %v not recognized", descheduler.Spec.Mode)
		}
	}

	images := map[string]string{
		"${IMAGE}": c.targetImagePullSpec,
	}
	for i := range required.Spec.Template.Spec.Containers {
		for pat, img := range images {
			if required.Spec.Template.Spec.Containers[i].Image == pat {
				required.Spec.Template.Spec.Containers[i].Image = img
				break
			}
		}
	}

	switch descheduler.Spec.LogLevel {
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

	resourcemerge.MergeMap(resourcemerge.BoolPtr(false), &required.Spec.Template.Annotations, specAnnotations)

	return resourceapply.ApplyDeployment(
		c.ctx,
		c.kubeClient.AppsV1(),
		c.eventRecorder,
		required,
		resourcemerge.ExpectedDeploymentGeneration(required, descheduler.Status.Generations))
}*/

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
