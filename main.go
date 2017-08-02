package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	//	"github.com/Sirupsen/logrus"

	//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	kwatch "k8s.io/apimachinery/pkg/watch"
	//"k8s.io/client-go/kubernetes"
	//	v1beta1extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
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

	//	}
	// create the clientset
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	stop := make(chan struct{})
	c := controller.New(kubeconfig, namespace)
	go c.Start(stop)

	<-signals

	close(stop)
	fmt.Println("Shutting down QuayCTL Controller")
}
