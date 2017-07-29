package main

import (
	"flag"
	"fmt"
	"os"

	//	"github.com/Sirupsen/logrus"

	//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	kwatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	//	v1beta1extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
	//	"net/http"
	//"k8s.io/client-go/rest"
	"github.com/soggiest/quayctl-controller/pkg/controller"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	internal  bool
	namespace string
)

func init() {
	flag.BoolVar(&internal, "Internal or External to a cluster?", false, "Used to determine whether or not we're running internal to a cluster or external")
	flag.Parse()
	namespace = os.Getenv("MY_POD_NAMESPACE")
	if len(namespace) == 0 {
		namespace = "default"
	}
}

func newControllerConfig(kubecli kubernetes.Interface, namespace string) controller.Config {
	// kubecli = k8sutil.MustNewKubeClient()

	//        serviceAccount, err := getMyPodServiceAccount(kubecli)
	//if err != nil {
	//	logrus.Fatalf("fail to get my pod's service account: %v", err)
	//}

	cfg := controller.Config{
		Namespace: namespace,
		//                ServiceAccount: serviceAccount,
		//                PVProvisioner:  pvProvisioner,
		//                S3Context: s3config.S3Context{
		//                        AWSSecret: awsSecret,
		//                        AWSConfig: awsConfig,
		//                        S3Bucket:  s3Bucket,
		//                },
		KubeCli: kubecli,
	}

	return cfg
}

func main() {
	//	if internal {
	//		fmt.Println("test")
	//		*config, err = rest.InClusterConfig()
	//		if err != nil {
	//			panic(err.Error())
	//		}
	//	} else {

	kubeconfig := "/home/soggy/concur/t16"
	//kubeconfig := "/tmp/qa16"
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	//	}
	// create the clientset
	k8sclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	contCFG := newControllerConfig(k8sclient, namespace)
	for {
		c := controller.New(contCFG)
		err := c.Run()
		switch err {
		default:
			fmt.Println("Controller Run() ended with failure: %v", err)
		}
	}
}
