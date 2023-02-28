package e2e_test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/operator-framework/operator-registry/pkg/configmap"
	unstructuredlib "github.com/operator-framework/operator-registry/pkg/lib/unstructured"
	"github.com/operator-framework/operator-registry/test/e2e/ctx"
)

var builderCmd string

const (
	imageDirectory = "testdata/bundles/"
)

func Logf(format string, a ...interface{}) {
	fmt.Fprintf(GinkgoWriter, "INFO: "+format+"\n", a...)
}

// checks command that it exists in $PATH, isn't a directory, and has executable permissions set
func checkCommand(filename string) string {
	path, err := exec.LookPath(filename)
	if err != nil {
		Logf("LookPath error: %v", err)
		return ""
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		Logf("IsNotExist error: %v", err)
		return ""
	}
	if info.IsDir() {
		return ""
	}
	perm := info.Mode()
	if perm&0111 != 0x49 { // 000100100100 - execute bits set for user/group/everybody
		Logf("permissions failure: %#v %#v", perm, perm&0111)
		return ""
	}

	Logf("Using builder at '%v'\n", path)
	return path
}

func init() {
	logrus.SetOutput(GinkgoWriter)

	if builderCmd = checkCommand("docker"); builderCmd != "" {
		return
	}
	if builderCmd = checkCommand("podman"); builderCmd != "" {
		return
	}
}

func buildContainer(tag, dockerfilePath, dockerContext string, w io.Writer) {
	cmd := exec.Command(builderCmd, "build", "-t", tag, "-f", dockerfilePath, dockerContext)
	cmd.Stderr = w
	cmd.Stdout = w
	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("Launch bundle", func() {
	namespace := "default"
	initImage := dockerHost + "/olmtest/init-operator-manifest:test"

	Context("Deploy bundle job", func() {
		DescribeTable("should populate specified configmap", func(bundleName, bundleDirectory string, gzip bool) {
			// these permissions are only necessary for the e2e (and not OLM using the feature)
			By("configuring configmap service account")
			kubeclient, err := kubernetes.NewForConfig(ctx.Ctx().RESTConfig())
			Expect(err).NotTo(HaveOccurred())

			roleName := "olm-dev-configmap-access"
			roleBindingName := "olm-dev-configmap-access-binding"

			_, err = kubeclient.RbacV1().Roles(namespace).Create(context.TODO(), &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: namespace,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"configmaps"},
						Verbs:     []string{"create", "get", "update"},
					},
				},
			}, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			_, err = kubeclient.RbacV1().RoleBindings(namespace).Create(context.TODO(), &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleBindingName,
					Namespace: namespace,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      "default",
						Namespace: namespace,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Role",
					Name:     roleName,
				},
			}, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("building required images")
			bundleImage := dockerHost + "/olmtest/" + bundleName + ":test"
			buildContainer(initImage, imageDirectory+"serve.Dockerfile", "../../bin", GinkgoWriter)
			buildContainer(bundleImage, imageDirectory+"bundle.Dockerfile", bundleDirectory, GinkgoWriter)

			err = pushLoadImages(kubeclient, GinkgoWriter, initImage, bundleImage)
			Expect(err).ToNot(HaveOccurred(), "error loading required images into cluster")

			By("creating a batch job")
			bundleDataConfigMap, job, err := configmap.LaunchBundleImage(kubeclient, bundleImage, initImage, namespace, gzip)
			Expect(err).NotTo(HaveOccurred())

			// wait for job to complete
			jobWatcher, err := kubeclient.BatchV1().Jobs(namespace).Watch(context.TODO(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			done := make(chan struct{})
			quit := make(chan struct{})
			defer close(quit)
			go func() {
				for {
					select {
					case <-quit:
						return
					case evt, ok := <-jobWatcher.ResultChan():
						if !ok {
							Logf("watch channel closed unexpectedly")
							return
						}
						if evt.Type == watch.Modified {
							job, ok := evt.Object.(*batchv1.Job)
							if !ok {
								continue
							}
							for _, condition := range job.Status.Conditions {
								if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
									Logf("Job complete")
									done <- struct{}{}
								}
							}
						}
					case <-time.After(120 * time.Second):
						Logf("Timeout waiting for job to complete")
						done <- struct{}{}
					}
				}
			}()

			Logf("Waiting on job to update status")
			<-done

			pl, err := kubeclient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(job.Spec.Selector)})
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range pl.Items {
				logs, err := kubeclient.CoreV1().Pods(namespace).GetLogs(pod.GetName(), &corev1.PodLogOptions{}).Stream(context.Background())
				Expect(err).NotTo(HaveOccurred())
				logData, err := ioutil.ReadAll(logs)
				Expect(err).NotTo(HaveOccurred())
				Logf("Pod logs for unpack job pod %q:\n%s", pod.GetName(), string(logData))
			}

			bundleDataConfigMap, err = kubeclient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), bundleDataConfigMap.GetName(), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			bundle, err := configmap.NewBundleLoader().Load(bundleDataConfigMap)
			Expect(err).NotTo(HaveOccurred())

			expectedObjects, err := unstructuredlib.FromDir(bundleDirectory + "/manifests")
			Expect(err).NotTo(HaveOccurred())

			configMapObjects, err := unstructuredlib.FromBundle(bundle)
			Expect(err).NotTo(HaveOccurred())

			Expect(configMapObjects).To(ConsistOf(expectedObjects))

			// clean up, perhaps better handled elsewhere
			err = kubeclient.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), bundleDataConfigMap.GetName(), metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			// job deletion does not clean up underlying pods (but using kubectl will do the clean up)
			pods, err := kubeclient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", job.GetName())})
			Expect(err).NotTo(HaveOccurred())
			err = kubeclient.CoreV1().Pods(namespace).Delete(context.TODO(), pods.Items[0].GetName(), metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = kubeclient.BatchV1().Jobs(namespace).Delete(context.TODO(), job.GetName(), metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = kubeclient.RbacV1().Roles(namespace).Delete(context.TODO(), roleName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = kubeclient.RbacV1().RoleBindings(namespace).Delete(context.TODO(), roleBindingName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		},

			Entry("Small bundle, uncompressed", "kiali.1.4.2", "testdata/bundles/kiali.1.4.2", false),
			Entry("Large bundle, compressed", "redis.0.4.0", "testdata/bundles/redis.0.4.0", true),
		)
	})
})

var kindControlPlaneNodeNameRegex = regexp.MustCompile("^kind-.*-control-plane$|^kind-control-plane$")

func isKindCluster(client *kubernetes.Clientset) (bool, string, error) {
	nodes, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		// transient error accessing nodes in cluster
		// return an error, failing the test
		return false, "", fmt.Errorf("accessing nodes in cluster")
	}

	var kindNode *corev1.Node
	for _, node := range nodes.Items {
		if kindControlPlaneNodeNameRegex.MatchString(node.Name) {
			kindNode = &node
		}
	}

	if kindNode == nil {
		return false, "", nil
	}
	// found a match... strip off -control-plane from name and return to caller
	return true, strings.TrimSuffix(kindNode.Name, "-control-plane"), nil
}

// pushLoadImages loads the built image onto the target cluster, either by "kind load docker-image",
// or by pushing to a registry.
func pushLoadImages(client *kubernetes.Clientset, w io.Writer, images ...string) error {
	kind, kindServerName, err := isKindCluster(client)
	if err != nil {
		return err
	}

	if kind {
		for _, image := range images {
			cmd := exec.Command("kind", "load", "docker-image", image, "--name", kindServerName)
			cmd.Stderr = w
			cmd.Stdout = w
			err := cmd.Run()
			if err != nil {
				return err
			}
		}
	} else {
		for _, image := range images {
			pushWith("docker", image)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
