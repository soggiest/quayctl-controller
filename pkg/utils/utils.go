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

	job := objCopy.(*batchv1.Job)
	if job.ObjectMeta.Status == nil {
		pod.ObjectMeta.Annotations = make(map[string]string)
	}
	return job, nil
}
