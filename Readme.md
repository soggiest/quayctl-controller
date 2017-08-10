# QuayCTL Controller

The `QuayCTL Controller` deploys [QuayCTL](https://github.com/coreos/quayctl) images as jobs across all schedulable nodes in a Kubernetes cluster. This controller was created to fulfill a need to have necessary images available as soon as possible on Kubernetes nodes in clusters that are geographically seperated from the central image repository. In order to make this happen there needed to be a way to periodically pull necessary images to all schedulable nodes in a cluster, while taking up as little resources as possible.

# Why a Controller?

Originally I investigated using a Cronjob object to deploy the containers that will actually do the pulling, but quickly discovered that CronJobs couldn't deploy DaemonSets. So, since this functionality was lacking I decided to write a controller that will select the nodes to deploy the puller images to, create Jobs for each of those nodes, and either wait for the Jobs to finish.

A CronJob is used to deploy the controller so the pulling of images to the remote clusters can happen at regular and controllable intervals.

# Prerequisites

* A Quay Enterprise Image Repository v2.4.0 or greater, using the `BitTorrent Downloads`
* A BitTorrent Tracker, such as Chihaya: [https://github.com/chihaya/chihaya]
* Port 6881 open from the K8S nodes to the Bittorrent tracker

# Deploying

To deploy the `QuayCTL Controller`:
* Grab the CronJob manifest `k8s/deploy/quayctl-controller-cronjob.yaml` and open it in a text editor
* Modify the value for the `QUAY_IMAGES` environmental value to include semicolon (;) delineated list of images you want to pull regularly.
  * EXAMPLE: `quay.io/nicholas_lane/chihaya:latest;quay.io/nicholas_lane/kibana:5.4.1-official`

NOTE: The `DEADLINE_SECONDS` environmental value is set to 5 minutes (300 seconds) by default, feel free to change this if you feel this is too short. This value determines how long it takes for the `QuayCTL Puller` jobs to fail. If the number of pulls you are executing or the latency from the target cluster to the central image repository is significant it would be wise to increase this value otherwise Jobs that would've succeeded would fail instead.

* Save the QuayCTL cronjob manifest locally once all changes have been made.
* Deploy the Controller:
  * `kubectl --kubeconfig <your kubeconfig file> create -f <path and filename of quayclt controller manifest file`

# Building

```
glide up -v
make quayctl-controller
```

# Thanks

* [Jimmy Zelinskie](https://github.com/jzelinskie) for all his help troubleshotting my numerous BitTorrent downloading problems.
* [Mike Metral](https://github.com/metral) who helped me wrap my head around writing controllers, who's [MemHog Operator](https://github.com/metral/memhog-operator) this controller borrows from heavily.
