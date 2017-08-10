package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"

	"github.com/soggiest/quayctl-controller/pkg/controller"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	//k8slogsutil "k8s.io/kubernetes/pkg/util/logs"
)

var (
	internal  bool
	namespace string
)

func init() {
	flag.BoolVar(&internal, "internal", false, "Used to determine whether or not we're running internal to a cluster or external")
	flag.Parse()
	namespace = os.Getenv("MY_POD_NAMESPACE")
	if len(namespace) == 0 {
		namespace = "default"
	}
}

func main() {

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	stop := make(chan struct{})
	c := controller.New(namespace)
	go c.Start(stop)

	<-signals

	close(stop)
	glog.V(2).Infof("Shutting down QuayCTL Controller")
}
