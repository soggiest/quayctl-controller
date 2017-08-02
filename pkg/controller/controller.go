package controller

import (
	//	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"time"
	//	"github.com/soggiest/quayctl-controller/pkg/spec"

	//        apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/client-go/tools/cache"

	"github.com/soggiest/quayctl-controller/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	//	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	batchv1 "k8s.io/client-go/pkg/apis/batch/v1"
	//v1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

var (
	jobAnnotation = map[string]string{"quay-piece": "puller-job"}
)

type Controller struct {
	logger    *logrus.Entry
	Namespace string
	//ServiceAccount string
	KubeCli kubernetes.Interface

	informer *ControllerInformer
	nodelist []map[string]string
	joblist  []*batchv1.Job
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

func New(kubeconfig string, namespace string) *Controller {

	var nodelist []map[string]string

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	k8sclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	getNodes, err := k8sclient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	for _, node := range getNodes.Items {
		if len(node.Spec.Taints) == 0 || !noSchedule(node.Spec.Taints) {
			selectLabel := ""
			for k, v := range node.Labels {
				if k == "kubernetes.io/hostname" {
					selectLabel = v
				}
			}

			selector := map[string]string{"kubernetes.io/hostname": selectLabel}

			nodelist = append(nodelist, selector)
		}
	}

	c := &Controller{
		logger:    logrus.WithField("pkg", "controller"),
		KubeCli:   k8sclient,
		Namespace: namespace,
		nodelist:  nodelist,
		//                clusters:   make(map[string]*cluster.Cluster),
		//                clusterRVs: make(map[string]string),
		//                stopChMap:  map[string]chan struct{}{},
	}

	c.informer = c.newControllerInformer()

	return c
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
	c.start(stop)

	<-stop
}

func (c *Controller) start(stop <-chan struct{}) {
	err := c.initResources()
	if err != nil {
		panic(err)
	}
	go c.informer.jobController.Run(stop)

	go c.Run(stop)
}

func (c *Controller) Run(stop <-chan struct{}) {
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

	//TODO: FIGURE OUT HOW I WANT TO END THE CONTROLLER. IT SHOULD END: IF THERE ARE NO MORE COMPLETED JOBS LEFT AND REPORT HOW MANY FAILED JOBS REMAIN.

	var (
		runningJobs   []*batchv1.Job
		completedJobs []*batchv1.Job
	)

	//lo := &metav1.ListOptions{

	//	LabelSelector: "quay-piece",
	//}
	//	jobList, err := c.KubeCli.Batch().Jobs("default").List(*lo)
	//	if err != nil {
	//		panic(err)
	//	}

	//	for _, job := range jobList.Items {
	c.pullJobs()
	runningJobs = runningJobs[:0]
	completedJobs = completedJobs[:0]

	for _, job := range c.joblist {
		fmt.Printf("Job List: %+v Active: %+v\n", job.Name, job.Status.Active)
		//if job.Status.Active > 0 {
		if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
			fmt.Printf("Running Job: %+v Active %+v\n", job.Name, job.Status)
			runningJobs = append(runningJobs, job)
		}
		if len(job.Status.Conditions) > 0 {
			if job.Status.Conditions[0].Type == "Completed" {
				completedJobs = append(completedJobs, job)
			}
		}
	}

	if len(completedJobs) == 0 && len(runningJobs) == 0 {
		os.Exit(0)
	}
}

func (c *Controller) pullJobs() {
	var (
		jobs []*batchv1.Job
	)

	lo := &metav1.ListOptions{

		LabelSelector: "quay-piece",
	}
	jobList, err := c.KubeCli.Batch().Jobs("default").List(*lo)

	for _, job := range jobList.Items {
		jobs = append(jobs, &job)
	}

	c.joblist = jobs
	if err != nil {
		panic(err)
	}

}

func (c *Controller) initResources() error {
	//func (c *Controller) initResources() ([]*batchv1.Job, error) {
	var (
		//	watchVer    string
		err  error
		job  *batchv1.Job
		jobs []*batchv1.Job
		//		selectLabel string
	)

	for index, selectLabel := range c.nodelist {
		jobName := fmt.Sprintf("quayctl-puller-job-%d", index)

		//err = c.KubeCli.Batch().Jobs("default").Get(jobName, metav1.Ge    tOptions{})
		//		fmt.Printf("SelectLabel: %+v\n", selectLabel)
		job, err = c.KubeCli.Batch().Jobs("default").Get(jobName, metav1.GetOptions{})
		if err != nil {
			job, err = c.createJob(jobName, selectLabel)
			if err != nil {
				panic(err)
			}

			jobNotReady := true
			fmt.Println("Waiting Until Job is Running")
			for jobNotReady == true {
				job, err := c.KubeCli.Batch().Jobs("default").Get(jobName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				//	fmt.Printf("Pod: %+v Active: %+v\n", job.Name, job.Status.Active)
				if job.Status.Active > 0 {
					jobNotReady = false
				}
			}

			//fmt.Printf("JOB CREATE RESULTS: %+v\n", job.Status)
			if err != nil {
				panic(err)
			}
		}
		jobs = append(jobs, job)
	}
	c.joblist = jobs

	return nil
}

func (c *Controller) newControllerInformer() *ControllerInformer {
	jobStore, jobController := c.newPodInformer()
	//	appMonitorStore, appMonitorController := amc.newAppMonitorInformer()

	return &ControllerInformer{
		jobStore:      jobStore,
		jobController: jobController,
		//		appMonitorStore:      appMonitorStore,
		//		appMonitorController: appMonitorController,
	}
}

func (c *Controller) newPodInformer() (cache.Store, cache.Controller) {
	return cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return c.KubeCli.Batch().Jobs(c.Namespace).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return c.KubeCli.Batch().Jobs(c.Namespace).Watch(options)
			},
		},
		// The resource that the informer returns
		&batchv1.Job{},
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
	//fmt.Printf("NewOBJ: %+v\n", newObj)

	job, err := utils.CopyObjToJob(newObj)
	if err != nil {
		fmt.Printf("Failed to copy Job object: %v", err)
		return
	}

	//if _, ok := job.Annotations["asdfgquay-piece/puller-job"]; ok {
	//	fmt.Printf("Failed to find Jobs with Annotation: %+v\n", job.Annotations)
	//	return
	//}

	//fmt.Printf("Received update for annotated Pod: %s | Annotations: %s", job.Name, job.Annotations)
	if len(job.Status.Conditions) > 0 {
		if job.Status.Conditions[0].Type == "Complete" {
			fmt.Printf("Deleting Completed Pod %+v\n", job.Name)
			errRem := c.removeJob(job.Name)
			if errRem != nil {
				panic(errRem)
			}
		}
	} else {
		fmt.Printf("Job %+v Incompleted. Status: %+v\n", job.Name, job.Status)
	}
}

func (c *Controller) createJob(jobName string, selector map[string]string) (*batchv1.Job, error) {
	fmt.Println("Creating Pod ---")
	//	fmt.Printf("SelectLabel: %+v\n", selectLabel)
	labels := map[string]string{"quay-piece": "quayctl-puller"}

	//annotations := map[string]string{"quay-piece": "puller-job"}
	//	selector := map[string]string{"kubernetes.io/hostname": selectLabel}
	//	jobName := fmt.Sprintf("quayctl-puller-job-%d", indexer)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      jobName,
			Labels:    labels,
			//			Annotations: jobAnnotation,
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
					RestartPolicy: "Never",
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
	return job, nil

}

func (c *Controller) removeJob(jobName string) error {
	err := c.KubeCli.Batch().Jobs("default").Delete(jobName, &metav1.DeleteOptions{})
	return err
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
