package controller

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/golang/glog"
	"os"
	"strconv"
	"time"

	"k8s.io/client-go/tools/cache"

	"github.com/soggiest/quayctl-controller/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	batchv1 "k8s.io/client-go/pkg/apis/batch/v1"
	"k8s.io/client-go/rest"
)

var (
	jobAnnotation = map[string]string{"quay-piece": "puller-job"}
)

type Controller struct {
	logger    *logrus.Entry
	Namespace string
	KubeCli   kubernetes.Interface

	informer *ControllerInformer
	nodelist []map[string]string
	joblist  []*batchv1.Job
}

type ControllerInformer struct {
	// Store & controller for Job resources
	jobStore      cache.Store
	jobController cache.Controller
}

func New(namespace string) *Controller {

	var nodelist []map[string]string
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	k8sclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	//Get the list of Nodes we will use for this Controller.
	getNodes, err := k8sclient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	for _, node := range getNodes.Items {
		if len(node.Spec.Taints) == 0 || !utils.NoSchedule(node.Spec.Taints) {
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
	}

	c.informer = c.newControllerInformer()

	return c
}

func (c *Controller) newControllerInformer() *ControllerInformer {
	jobStore, jobController := c.newPodInformer()

	return &ControllerInformer{
		jobStore:      jobStore,
		jobController: jobController,
	}
}

func (c *Controller) newPodInformer() (cache.Store, cache.Controller) {
	return cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return c.KubeCli.Batch().Jobs(c.Namespace).List(metav1.ListOptions{LabelSelector: "quay-piece=quayctl-puller"})
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				//options := metav1.ListOptions{LabelSelector: "quay-piece=quayctl-puller"}
				return c.KubeCli.Batch().Jobs(c.Namespace).Watch(metav1.ListOptions{LabelSelector: "quay-piece=quayctl-puller"})
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
		glog.V(2).Infof("Failed to copy Job object: %v", err)
		return
	}

	if len(job.Status.Conditions) > 0 {
		if job.Status.Conditions[0].Type == "Complete" {
			glog.V(2).Infof("Deleting Completed Pod %+v\n", job.Name)
			errRem := c.removeJob(job.Name)
			if errRem != nil {
				panic(errRem)
			}
		} else if job.Status.Conditions[0].Type == "Failed" {
			glog.V(2).Infof("Job %+v Failed, cleaning it up", job.Name)
			errRem := c.removeJob(job.Name)
			if errRem != nil {
				panic(errRem)
			}
		}
	} else {
		glog.V(2).Infof("Job %+v Incompleted. Status: %+v\n", job.Name, job.Status)
	}
}

func (c *Controller) Start(stop <-chan struct{}) {
	//In Mike's operator he has a piece to handle panics. I may want to incorporate this
	//defer itulruntim.HandleCrash()

	glog.V(2).Infof("Starting QuayCTL Controller")
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
			glog.V(2).Infof("Shutting down QuayCTL Controller")
			return
		default:
			c.run()
			time.Sleep(2 * time.Second)
		}
	}
}

func (c *Controller) run() {

	var (
		runningJobs   []*batchv1.Job
		completedJobs []*batchv1.Job
	)

	c.pullJobs()
	runningJobs = runningJobs[:0]
	completedJobs = completedJobs[:0]

	for _, job := range c.joblist {
		if job.Status.Active == 1 {
			//if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
			runningJobs = append(runningJobs, job)
		}
		if len(job.Status.Conditions) > 0 {
			if job.Status.Conditions[0].Type == "Completed" {
				completedJobs = append(completedJobs, job)
			}
		}
	}

	if len(completedJobs) == 0 && len(runningJobs) == 0 {
		glog.V(2).Infof("Jobs have either completed or failed, exiting QuayCTL Controller")
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
	var (
		err  error
		job  *batchv1.Job
		jobs []*batchv1.Job
	)

	for index, selectLabel := range c.nodelist {
		jobName := fmt.Sprintf("quayctl-puller-job-%d", index)

		job, err = c.KubeCli.Batch().Jobs("default").Get(jobName, metav1.GetOptions{})
		if err != nil {
			job, err = c.createJob(jobName, selectLabel)
			if err != nil {
				panic(err)
			}

			jobNotReady := true
			glog.V(2).Infof("Waiting Until Job is Running")
			for jobNotReady == true {
				job, err := c.KubeCli.Batch().Jobs("default").Get(jobName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if job.Status.Active > 0 {
					jobNotReady = false
				}
			}

			if err != nil {
				panic(err)
			}
		}
		jobs = append(jobs, job)
	}
	c.joblist = jobs

	return nil
}

func (c *Controller) createJob(jobName string, selector map[string]string) (*batchv1.Job, error) {
	labels := map[string]string{"quay-piece": "quayctl-puller"}
	seconds, serr := strconv.ParseInt(os.Getenv("DEADLINE_SECONDS"), 10, 64)
	if serr != nil {
		panic(serr)
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      jobName,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: &seconds,
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
							Env: []v1.EnvVar{
								{
									Name:  "QUAY_IMAGES",
									Value: os.Getenv("QUAY_IMAGES"),
								},
							},
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
					HostNetwork:   true,
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
