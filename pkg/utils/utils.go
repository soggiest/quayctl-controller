package utils

import (
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	batchv1 "k8s.io/client-go/pkg/apis/batch/v1"
)

func CopyObjToJob(obj interface{}) (*batchv1.Job, error) {
	objCopy, err := api.Scheme.Copy(obj.(*batchv1.Job))
	if err != nil {
		return nil, err
	}

	//fmt.Println(objCopy)
	job := objCopy.(*batchv1.Job)
	if job.ObjectMeta.Annotations == nil {
		job.ObjectMeta.Annotations = make(map[string]string)
	}
	return job, nil
}

//Determine whether a Node's Taint makes it unschedulable
func NoSchedule(taints []v1.Taint) bool {
	for _, taint := range taints {
		if taint.Effect == v1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}
