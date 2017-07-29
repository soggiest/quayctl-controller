package controller

import (
	//	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	//	"github.com/soggiest/quayctl-controller/pkg/spec"

	//        apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/client-go/tools/cache"

	"github.com/soggiest/quay-controller/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	batchv1 "k8s.io/client-go/pkg/apis/batch/v1"
	//v1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

type Controller struct {
	logger *logrus.Entry
	Config

	informer *ControllerInformer

	// TODO: combine the three cluster map.
	//        clusters map[string]*cluster.Cluster
	// Kubernetes resource version of the clusters
	//       clusterRVs map[string]string
	//        stopChMap  map[string]chan struct{}

	//        waitCluster sync.WaitGroup
}

type ControllerInformer struct {
	// Store & controller for Pod resources
	jobStore      cache.Store
	jobController cache.Controller

	// Store & controller for AppMonitor resources
	//	appMonitorStore      cache.Store
	//	appMonitorController cache.Controller
}

type Config struct {
	Namespace string
	//ServiceAccount string
	KubeCli kubernetes.Interface
}

func New(cfg Config) *Controller {

	cInformer := c.newControllerInformer()

	return &Controller{
		logger: logrus.WithField("pkg", "controller"),

		Config: cfg,
		//                clusters:   make(map[string]*cluster.Cluster),
		//                clusterRVs: make(map[string]string),
		//                stopChMap:  map[string]chan struct{}{},
	}
}

func noSchedule(taints []v1.Taint) bool {
	for _, taint := range taints {
		if taint.Effect == v1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}

func (c *Controller) Start(stop <-chan struct{}) {
	//In Mike's operator he has a piece to handle panics. I may want to incorporate this
	//defer itulruntim.HandleCrash()

	fmt.Println("Starting QuayCTL Controller")
	amc.start(stop)

	<-stop
}

func (c *Controller) start(stop <-chan struct{}) {
	go c.informer.podController.Run(stop)

	go c.Run(stop)
}

func (c *Controller) Run(stop <-chan struct{}) error {
	for {
		select {
		case <-stop:
			fmt.Println("Shutting down QuayCTL Controller")
			return
		default:
			c.run()
			time.Sleep(2 * time.Second)
		}
	}
}

func (c *Controller) run() {
	var (
		jobs []*batchv1.Job
	)

	jobs, err := c.initResources()
	if err != nil {
		fmt.Printf("Failed to initialize QuayCTL resources: %+v\n", err)
	}

	lo := metav1.ListOptions{
		Watch: true,
	}

	for _, job := range jobs {
		fmt.Printf("Status: %+v\n", job.Status)
	}

	panic("Node Listing ATM")

	//	for {
	//	watchVer, err = c.initResources()
	if err != nil {
		fmt.Printf("Err: %+v", err)
		panic(err)
	}
	fmt.Println(watchVer)
	return nil
	//	}
}

func (c *Controller) initResources() (string, error) {
	var (
		watchVer    string
		err         error
		jobs        []*batchv1.Job
		selectLabel string
	)

	nodeList, err := c.KubeCli.CoreV1().Nodes().List(metav1.ListOptions{})

	for index, node := range nodeList.Items {
		for k, v := range node.Labels {
			if k == "kubernetes.io/hostname" {
				selectLabel = v
			}
		}
		if len(node.Spec.Taints) == 0 || !noSchedule(node.Spec.Taints) {
			job, err := c.createJob(&node, index, selectLabel)
			if err != nil {
				panic(err)
			}
			jobs = append(jobs, job)
		}
	}

}

func (c *Controller) newControllerInformer() *ControllerInformer {
	podStore, podController := c.newPodInformer()
	//	appMonitorStore, appMonitorController := amc.newAppMonitorInformer()

	return &ControllerInformer{
		podStore:      podStore,
		podController: podController,
		//		appMonitorStore:      appMonitorStore,
		//		appMonitorController: appMonitorController,
	}
}

func (c *Controller) newPodInformer() (cache.Store, cache.Controller) {
	return cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return c.KubeCli.CoreV1().Jobs(c.namespace).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return c.KubeCli.CoreV1().Jobs(c.namespace).Watch(options)
			},
		},
		// The resource that the informer returns
		&v1.Pod{},
		// The sync interval of the informer
		5*time.Second,
		// Callback functions for add, delete & update events
		cache.ResourceEventHandlerFuncs{
			// AddFunc: func(o interface{}) {}
			UpdateFunc: c.handleJobsUpdate,
			// DeleteFunc: func(o interface{}) {}
		},
	)
}

func (c *Controller) handleJobsUpdate(oldObj, newObj interface{}) {
	// Make a copy of the object, to not mutate the original object from the local
	// cache store
	job, err := utils.CopyObjToJob(newObj)
	if err != nil {
		fmt.Printf("Failed to copy Jod object: %v", err)
		return
	}

	if _, ok := newObj.Status; !ok {
		return
	}

	fmt.Printf("Received update for annotated Pod: %s | Annotations: %s", pod.Name, pod.Annotations)
}

func (c *Controller) createJob(node *v1.Node, indexer int, selectLabel string) (*batchv1.Job, error) {
	labels := map[string]string{"quay-piece": "quayctl-puller"}
	selector := map[string]string{"kubernetes.io/hostname": selectLabel}
	jobName := fmt.Sprintf("quayctl-puller-job-%d", indexer)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      jobName,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					ServiceAccountName: "default",
					ImagePullSecrets: []v1.LocalObjectReference{
						{Name: "coreos-pull-secret"},
					},
					Containers: []v1.Container{
						{
							Name:            "puller",
							Image:           "quay.cnqr.delivery/containerhosting/quayctl-puller:latest",
							ImagePullPolicy: "Always",
							Ports: []v1.ContainerPort{
								{
									ContainerPort: 5000,
									HostPort:      5000,
									Name:          "docker",
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "dockervol",
									MountPath: "/var/run/docker.sock",
								},
							},
						},
					},
					NodeSelector:  selector,
					RestartPolicy: "OnFailure",
					Volumes: []v1.Volume{
						{
							Name: "dockervol",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/run/docker.sock",
								},
							},
						},
					},
				},
			},
		},
	}
	job, err := c.KubeCli.Batch().Jobs("default").Create(job)
	if err != nil {
		return nil, err
	}
	//ds, err := c.KubeCli.Batch().Jobs("default").Get("daemonset-quayctl-puller-1", metav1.GetOptions{})
	//fmt.Printf("JOBS: %+v\n", ds)
	return job, nil

}

//func (c *Controller) waitForDS(ds *v1beta.DaemonSet) error {
//	fmt.Println(ds.ResourceVersion)
//	return nil
//}

//func (c *Controller) createDS() error {
//	labels := map[string]string{"quay-piece": "daemonset-puller"}
//	daemonset := &v1beta1.DaemonSet{
//		ObjectMeta: metav1.ObjectMeta{
//			Namespace: "default",
//			Name:      "daemonset-quayctl-puller",
//			Labels:    labels,
//		},
//		Spec: v1beta1.DaemonSetSpec{
//			Template: v1.PodTemplateSpec{
//				ObjectMeta: metav1.ObjectMeta{
//					Labels: labels,
//				},
//				Spec: v1.PodSpec{
//					ServiceAccountName: "default",
//					ImagePullSecrets: []v1.LocalObjectReference{
//						{Name: "coreos-pull-secret"},
//					},
//					Containers: []v1.Container{
//						{
//							Name:            "puller",
//							Image:           "quay.cnqr.delivery/containerhosting/quayctl-puller:latest",
//							ImagePullPolicy: "Always",
//							Ports: []v1.ContainerPort{
//								{
//									ContainerPort: 5000,
//									HostPort:      5000,
//									Name:          "docker",
//								},
//							},
//							VolumeMounts: []v1.VolumeMount{
//								{
//									Name:      "dockervol",
//									MountPath: "/var/run/docker.sock",
//								},
//							},
//						},
//					},
//					Volumes: []v1.Volume{
//						{
//							Name: "dockervol",
//							VolumeSource: v1.VolumeSource{
//								HostPath: &v1.HostPathVolumeSource{
//									Path: "/var/run/docker.sock",
//								},
//							},
//						},
//					},
//				},
//			},
//		},
//	}
//
//	_, err := c.KubeCli.ExtensionsV1beta1().DaemonSets("default").Create(daemonset)
//	if err != nil {
//		return err
//	}
//	ds, err := c.KubeCli.ExtensionsV1beta1().DaemonSets("default").Get("daemonset-quayctl-puller", metav1.GetOptions{})
//	fmt.Printf("DS: %+v", ds)
//	return err
//}
