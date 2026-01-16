package e2e

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	g "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"

	"github.com/openshift/cli-manager/api/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"

	climanagerv1 "github.com/openshift/cli-manager-operator/pkg/apis/climanager/v1"
	climanagerscheme "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned/scheme"
	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
	"github.com/openshift/cli-manager-operator/test/e2e/bindata"
)

var _ = g.Describe("[Operator][Serial] CLI Manager Operator", func() {
	g.BeforeEach(func() {
		setupOperator(g.GinkgoTB())
	})

	g.It("should successfully deploy and serve plugins via krew", func() {
		testCLIManager(g.GinkgoTB())
	})
})

// setupOperator sets up the operator environment and waits for it to be ready.
func setupOperator(t testing.TB) {
	kubeClient := GetKubeClient()
	apiExtClient := GetApiExtensionClient()
	cliManagerClient := GetCLIManagerClient()
	routeClient := GetRouteClient()

	eventRecorder := events.NewKubeRecorder(kubeClient.CoreV1().Events("default"), "test-e2e", &corev1.ObjectReference{}, clock.RealClock{})

	ctx, cancelFnc := context.WithCancel(context.TODO())
	defer cancelFnc()

	// Get cluster version
	cmd := exec.Command("oc", "get", "clusterversion", "-o", "jsonpath={.items[0].status.desired.version}")
	versionOutput, err := cmd.Output()
	if err != nil {
		t.Fatalf("Unable to get cluster version: %v", err)
	}
	versionParts := strings.Split(string(versionOutput), ".")
	if len(versionParts) < 2 {
		t.Fatalf("Unable to parse cluster version: %s", string(versionOutput))
	}
	ocpVersion := fmt.Sprintf("%s.%s", versionParts[0], versionParts[1])
	klog.Infof("Detected OCP version: %s", ocpVersion)

	assets := []struct {
		path           string
		readerAndApply func(objBytes []byte) error
	}{
		{
			path: "assets/00_cli-manager-operator.crd.yaml",
			readerAndApply: func(objBytes []byte) error {
				_, _, err := resourceapply.ApplyCustomResourceDefinitionV1(ctx, apiExtClient.ApiextensionsV1(), eventRecorder, resourceread.ReadCustomResourceDefinitionV1OrDie(objBytes))
				return err
			},
		},
		{
			path: "assets/01_namespace.yaml",
			readerAndApply: func(objBytes []byte) error {
				_, _, err := resourceapply.ApplyNamespace(ctx, kubeClient.CoreV1(), eventRecorder, resourceread.ReadNamespaceV1OrDie(objBytes))
				return err
			},
		},
		{
			path: "assets/02_config.openshift.io_plugins.yaml",
			readerAndApply: func(objBytes []byte) error {
				_, _, err := resourceapply.ApplyCustomResourceDefinitionV1(ctx, apiExtClient.ApiextensionsV1(), eventRecorder, resourceread.ReadCustomResourceDefinitionV1OrDie(objBytes))
				return err
			},
		},
		{
			path: "assets/03_clusterrole.yaml",
			readerAndApply: func(objBytes []byte) error {
				_, _, err := resourceapply.ApplyClusterRole(ctx, kubeClient.RbacV1(), eventRecorder, resourceread.ReadClusterRoleV1OrDie(objBytes))
				return err
			},
		},
		{
			path: "assets/04_clusterrolebinding.yaml",
			readerAndApply: func(objBytes []byte) error {
				_, _, err := resourceapply.ApplyClusterRoleBinding(ctx, kubeClient.RbacV1(), eventRecorder, resourceread.ReadClusterRoleBindingV1OrDie(objBytes))
				return err
			},
		},
		{
			path: "assets/05_serviceaccount.yaml",
			readerAndApply: func(objBytes []byte) error {
				_, _, err := resourceapply.ApplyServiceAccount(ctx, kubeClient.CoreV1(), eventRecorder, resourceread.ReadServiceAccountV1OrDie(objBytes))
				return err
			},
		},
		{
			path: "assets/06_deployment.yaml",
			readerAndApply: func(objBytes []byte) error {
				required := resourceread.ReadDeploymentV1OrDie(objBytes)
				// override the operator image with the one built in the CI

				operatorImage := fmt.Sprintf("registry.ci.openshift.org/ocp/%s:cli-manager-operator", ocpVersion)
				required.Spec.Template.Spec.Containers[0].Image = operatorImage
				klog.Infof("Using operator image: %s", operatorImage)

				// RELATED_IMAGE_OPERAND_IMAGE env
				operandImage := fmt.Sprintf("registry.ci.openshift.org/ocp/%s:cli-manager", ocpVersion)
				for i, env := range required.Spec.Template.Spec.Containers[0].Env {
					if env.Name == "RELATED_IMAGE_OPERAND_IMAGE" {
						required.Spec.Template.Spec.Containers[0].Env[i].Value = operandImage
						klog.Infof("Using operand image: %s", operandImage)
						break
					}
				}

				_, _, err := resourceapply.ApplyDeployment(
					ctx,
					kubeClient.AppsV1(),
					eventRecorder,
					required,
					1000, // any random high number
				)
				return err
			},
		},
		{
			path: "assets/07_cli-manager-operator-cr.yaml",
			readerAndApply: func(objBytes []byte) error {
				requiredObj, err := apiruntime.Decode(climanagerscheme.Codecs.UniversalDecoder(climanagerv1.SchemeGroupVersion), objBytes)
				if err != nil {
					klog.Errorf("Unable to decode assets/07_cli-manager-operator-cr.yaml: %v", err)
					return err
				}
				requiredCLI := requiredObj.(*climanagerv1.CliManager)

				_, err = cliManagerClient.ClimanagersV1().CliManagers(requiredCLI.Namespace).Create(ctx, requiredCLI, metav1.CreateOptions{})
				return err
			},
		},
	}

	// create required resources, e.g. namespace, crd, roles
	if err := wait.PollUntilContextTimeout(context.TODO(), 1*time.Second, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		for _, asset := range assets {
			klog.Infof("Creating %v", asset.path)
			if err := asset.readerAndApply(bindata.MustAsset(asset.path)); err != nil {
				klog.Errorf("Unable to create %v: %v", asset.path, err)
				return false, nil
			}
		}

		return true, nil
	}); err != nil {
		t.Fatalf("Unable to create CLIO resources: %v", err)
	}

	var cliManagerOperatorPod *corev1.Pod
	// Wait until the CLI Manager Operator pod is running
	if err := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		klog.Infof("Listing pods...")
		podItems, err := kubeClient.CoreV1().Pods(operatorclient.OperatorNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Unable to list pods: %v", err)
			return false, nil
		}
		for _, pod := range podItems.Items {
			if !strings.HasPrefix(pod.Name, operatorclient.OperandName+"-operator") {
				continue
			}
			klog.Infof("Checking pod: %v, phase: %v, deletionTS: %v\n", pod.Name, pod.Status.Phase, pod.GetDeletionTimestamp())
			if pod.Status.Phase == corev1.PodRunning && pod.GetDeletionTimestamp() == nil {
				cliManagerOperatorPod = pod.DeepCopy()
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		t.Fatalf("Unable to wait for the CLIO pod to run")
	}

	klog.Infof("CLI Manager Operator running in %v", cliManagerOperatorPod.Name)

	var cliManagerPod *corev1.Pod
	// Wait until the CLI Manager pod is running
	if err := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 2*time.Minute, false, func(ctx context.Context) (bool, error) {
		klog.Infof("Listing pods...")
		podItems, err := kubeClient.CoreV1().Pods(operatorclient.OperatorNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Unable to list pods: %v", err)
			return false, nil
		}
		for _, pod := range podItems.Items {
			// skip if pod.Name _doesn't_ have operatorclient.OperandName (operand should have this)
			// or if it _has_ operatorclient.OperandName + '-operator'
			if !strings.HasPrefix(pod.Name, operatorclient.OperandName) || strings.HasPrefix(pod.Name, operatorclient.OperandName+"-operator") {
				continue
			}
			klog.Infof("Checking pod: %v, phase: %v, deletionTS: %v\n", pod.Name, pod.Status.Phase, pod.GetDeletionTimestamp())
			if pod.Status.Phase == corev1.PodRunning && pod.GetDeletionTimestamp() == nil {
				cliManagerPod = pod.DeepCopy()
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		t.Fatalf("Unable to wait for the CLI Manager (operand) pod to run")
	}

	klog.Infof("CLI Manager (operand) pod running in %v", cliManagerPod.Name)

	r, err := routeClient.Routes("openshift-cli-manager-operator").Get(context.TODO(), "openshift-cli-manager", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unable to get route host: %v", err)
	}

	krewUrl := fmt.Sprintf("https://%s/cli-manager", r.Spec.Host)

	if err := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 5*time.Minute, false, func(ctx context.Context) (bool, error) {
		klog.Infof("checking the route is alive")

		tr := &http.Transport{
			// Just a simple get request to check the route is up and running.
			// So that we can use skip tls verification.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err := client.Get(krewUrl)
		if err != nil {
			klog.Errorf("Unable to send request to %s: %v", krewUrl, err)
			return false, nil
		}
		defer resp.Body.Close()
		// we are checking notfound because basically our custom git server
		// does not serve anything and it is legit to get this error in GET request.
		// Whereas, by getting NotFound error proves that route configuration is up.
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
			return true, nil
		}
		klog.Infof("still not alive, status code %d", resp.StatusCode)
		return false, nil
	}); err != nil {
		t.Fatalf("Unable to wait for CLI Manager route")
	}
}

// testCLIManager tests the CLI Manager functionality.
func testCLIManager(t testing.TB) {
	customKrewIndexName := "test-e2e"
	routeClient := GetRouteClient()

	r, err := routeClient.Routes("openshift-cli-manager-operator").Get(context.TODO(), "openshift-cli-manager", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("route get error %v", err)
	}
	krewUrl := fmt.Sprintf("https://%s/cli-manager", r.Spec.Host)

	currentPath := homedir.HomeDir() + "/.krew"
	cmd := exec.Command("oc", "krew", "index", "add", customKrewIndexName, krewUrl)
	cmd.Env = []string{
		"GIT_SSL_NO_VERIFY=true",
		"KREW_ROOT=" + currentPath,
		"KREW_OS=" + runtime.GOOS,
		"KREW_ARCH=" + runtime.GOARCH,
	}
	cmd.Env = append(cmd.Env, "PATH="+currentPath+"/bin"+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oc krew index add operation failed %v output: %s", err, string(out))
	}

	dynamicClient := GetApiDynamicClient()
	plugin := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "config.openshift.io/v1alpha1",
			"kind":       "Plugin",
			"metadata": map[string]any{
				"name": "oc",
			},
			"spec": map[string]any{
				"homepage":         "https://github.com/openshift/oc/",
				"shortDescription": "Binary for oc",
				"description":      "this is a test plugin to deliver oc",
				"version":          "v4.15.0",
				"platforms": []map[string]any{
					{
						"platform": "linux/amd64",
						"image":    "quay.io/openshift/origin-cli",
						"bin":      "oc",
						"files": []map[string]any{
							{
								"from": "/usr/bin/oc",
								"to":   ".",
							},
						},
					},
				},
			},
		},
	}

	_, err = dynamicClient.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Version: "v1alpha1", Resource: "plugins"}).Create(context.TODO(), plugin, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("test plugin creation error %v", err)
	}

	err = wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		pluginName := fmt.Sprintf("%s/oc", customKrewIndexName)
		cmd := exec.Command("oc", "krew", "update")
		cmd.Env = []string{
			"GIT_SSL_NO_VERIFY=true",
			"KREW_ROOT=" + currentPath,
			"KREW_OS=" + runtime.GOOS,
			"KREW_ARCH=" + runtime.GOARCH,
		}
		cmd.Env = append(cmd.Env, "PATH="+currentPath+"/bin"+string(os.PathListSeparator)+os.Getenv("PATH"))
		err := cmd.Run()
		if err != nil {
			t.Fatalf("oc krew update operation failed %v", err)
		}

		cmd = exec.Command("oc", "krew", "search", pluginName)
		cmd.Env = []string{
			"GIT_SSL_NO_VERIFY=true",
			"KREW_ROOT=" + currentPath,
			"KREW_OS=" + runtime.GOOS,
			"KREW_ARCH=" + runtime.GOARCH,
		}
		cmd.Env = append(cmd.Env, "PATH="+currentPath+"/bin"+string(os.PathListSeparator)+os.Getenv("PATH"))
		res, err := cmd.Output()
		if err != nil {
			return false, err
		}
		if strings.Contains(string(res), pluginName) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("plugin search failed %v", err)
	}

	cmd = exec.Command("oc", "krew", "install", fmt.Sprintf("%s/%s", customKrewIndexName, "oc"))
	cmd.Env = []string{
		"GIT_SSL_NO_VERIFY=true",
		"KREW_ROOT=" + currentPath,
		"KREW_OS=" + runtime.GOOS,
		"KREW_ARCH=" + runtime.GOARCH,
	}
	cmd.Env = append(cmd.Env, "PATH="+currentPath+"/bin"+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin installation failure %s output: %s", err, string(out))
	}

	cmd = exec.Command("oc", "oc", "version")
	cmd.Env = []string{
		"GIT_SSL_NO_VERIFY=true",
		"KREW_ROOT=" + currentPath,
		"KREW_OS=" + runtime.GOOS,
		"KREW_ARCH=" + runtime.GOARCH,
	}
	cmd.Env = append(cmd.Env, "PATH="+currentPath+"/bin"+string(os.PathListSeparator)+os.Getenv("PATH"))
	ver, err := cmd.Output()
	if err != nil {
		t.Fatalf("plugin execution failure response %s err %v", string(ver), err)
	}
	klog.Infof("plugin oc execution result \n %s", string(ver))
	if !strings.Contains(string(ver), "Client Version:") {
		t.Fatalf("unexpected output of plugin execution %s", string(ver))
	}

	unstrctrd, err := dynamicClient.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Version: "v1alpha1", Resource: "plugins"}).Get(context.TODO(), "oc", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("test plugin retrieval error %v", err)
	}

	latestPlugin := &v1alpha1.Plugin{}
	err = machineryruntime.DefaultUnstructuredConverter.FromUnstructured(unstrctrd.UnstructuredContent(), latestPlugin)
	if err != nil {
		t.Fatalf("test plugin conversion error %v", err)
	}

	if len(latestPlugin.Status.Conditions) == 0 {
		t.Fatalf("unexpected empty condition of plugin oc")
	}

	if latestPlugin.Status.Conditions[0].Status != metav1.ConditionTrue || latestPlugin.Status.Conditions[0].Reason != "Installed" {
		t.Fatalf("unexpected condition of plugin %s reason %s", latestPlugin.Status.Conditions[0].Status, latestPlugin.Status.Conditions[0].Reason)
	}
}
