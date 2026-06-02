package cache

import (
	k8scommon "github.com/dnsjia/luban/pkg/k8s/common"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func ResourceChannels(clusterID string, nsQuery *k8scommon.NamespaceQuery, numReads int) (*k8scommon.ResourceChannels, bool) {
	cc, ok := Global.GetCluster(clusterID)
	if !ok {
		return nil, false
	}
	channels, err := cc.resourceChannels(nsQuery, numReads)
	if err != nil {
		logError("build k8s cache resource channels failed", err)
		return nil, false
	}
	return channels, true
}

func (cc *ClusterCache) resourceChannels(nsQuery *k8scommon.NamespaceQuery, numReads int) (*k8scommon.ResourceChannels, error) {
	if nsQuery == nil {
		nsQuery = k8scommon.NewNamespaceQuery(nil)
	}
	deployments, err := cc.listDeployments(nsQuery)
	if err != nil {
		return nil, err
	}
	statefulSets, err := cc.listStatefulSets(nsQuery)
	if err != nil {
		return nil, err
	}
	daemonSets, err := cc.listDaemonSets(nsQuery)
	if err != nil {
		return nil, err
	}
	replicaSets, err := cc.listReplicaSets(nsQuery)
	if err != nil {
		return nil, err
	}
	jobs, err := cc.listJobs(nsQuery)
	if err != nil {
		return nil, err
	}
	cronJobs, err := cc.listCronJobs(nsQuery)
	if err != nil {
		return nil, err
	}
	pods, err := cc.listPods(nsQuery)
	if err != nil {
		return nil, err
	}
	events, err := cc.listEvents(nsQuery)
	if err != nil {
		return nil, err
	}
	return &k8scommon.ResourceChannels{
		DeploymentList:  deploymentChannel(deployments, nil, numReads),
		StatefulSetList: statefulSetChannel(statefulSets, nil, numReads),
		DaemonSetList:   daemonSetChannel(daemonSets, nil, numReads),
		ReplicaSetList:  replicaSetChannel(replicaSets, nil, numReads),
		JobList:         jobChannel(jobs, nil, numReads),
		CronJobList:     cronJobChannel(cronJobs, nil, numReads),
		PodList:         podChannel(pods, nil, numReads),
		EventList:       eventChannel(events, nil, numReads),
	}, nil
}

func (cc *ClusterCache) listDeployments(nsQuery *k8scommon.NamespaceQuery) (*appsv1.DeploymentList, error) {
	items, err := cc.deploymentLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	result := &appsv1.DeploymentList{}
	for _, item := range items {
		if nsQuery.Matches(item.Namespace) {
			result.Items = append(result.Items, *item.DeepCopy())
		}
	}
	return result, nil
}

func (cc *ClusterCache) listStatefulSets(nsQuery *k8scommon.NamespaceQuery) (*appsv1.StatefulSetList, error) {
	items, err := cc.statefulSetLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	result := &appsv1.StatefulSetList{}
	for _, item := range items {
		if nsQuery.Matches(item.Namespace) {
			result.Items = append(result.Items, *item.DeepCopy())
		}
	}
	return result, nil
}

func (cc *ClusterCache) listDaemonSets(nsQuery *k8scommon.NamespaceQuery) (*appsv1.DaemonSetList, error) {
	items, err := cc.daemonSetLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	result := &appsv1.DaemonSetList{}
	for _, item := range items {
		if nsQuery.Matches(item.Namespace) {
			result.Items = append(result.Items, *item.DeepCopy())
		}
	}
	return result, nil
}

func (cc *ClusterCache) listReplicaSets(nsQuery *k8scommon.NamespaceQuery) (*appsv1.ReplicaSetList, error) {
	items, err := cc.replicaSetLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	result := &appsv1.ReplicaSetList{}
	for _, item := range items {
		if nsQuery.Matches(item.Namespace) {
			result.Items = append(result.Items, *item.DeepCopy())
		}
	}
	return result, nil
}

func (cc *ClusterCache) listJobs(nsQuery *k8scommon.NamespaceQuery) (*batchv1.JobList, error) {
	items, err := cc.jobLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	result := &batchv1.JobList{}
	for _, item := range items {
		if nsQuery.Matches(item.Namespace) {
			result.Items = append(result.Items, *item.DeepCopy())
		}
	}
	return result, nil
}

func (cc *ClusterCache) listCronJobs(nsQuery *k8scommon.NamespaceQuery) (*batchv1beta1.CronJobList, error) {
	items, err := cc.cronJobLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	result := &batchv1beta1.CronJobList{}
	for _, item := range items {
		if nsQuery.Matches(item.Namespace) {
			result.Items = append(result.Items, convertCronJobV1ToV1beta1(item))
		}
	}
	return result, nil
}

func convertCronJobV1ToV1beta1(item *batchv1.CronJob) batchv1beta1.CronJob {
	if item == nil {
		return batchv1beta1.CronJob{}
	}
	return batchv1beta1.CronJob{
		TypeMeta:   item.TypeMeta,
		ObjectMeta: *item.ObjectMeta.DeepCopy(),
		Spec: batchv1beta1.CronJobSpec{
			Schedule:                item.Spec.Schedule,
			StartingDeadlineSeconds: item.Spec.StartingDeadlineSeconds,
			ConcurrencyPolicy:       batchv1beta1.ConcurrencyPolicy(item.Spec.ConcurrencyPolicy),
			Suspend:                 item.Spec.Suspend,
			JobTemplate: batchv1beta1.JobTemplateSpec{
				ObjectMeta: item.Spec.JobTemplate.ObjectMeta,
				Spec:       item.Spec.JobTemplate.Spec,
			},
			SuccessfulJobsHistoryLimit: item.Spec.SuccessfulJobsHistoryLimit,
			FailedJobsHistoryLimit:     item.Spec.FailedJobsHistoryLimit,
		},
		Status: batchv1beta1.CronJobStatus{
			Active:             item.Status.Active,
			LastScheduleTime:   item.Status.LastScheduleTime,
			LastSuccessfulTime: item.Status.LastSuccessfulTime,
		},
	}
}

func (cc *ClusterCache) listPods(nsQuery *k8scommon.NamespaceQuery) (*corev1.PodList, error) {
	items, err := cc.podLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	result := &corev1.PodList{}
	for _, item := range items {
		if nsQuery.Matches(item.Namespace) {
			result.Items = append(result.Items, *item.DeepCopy())
		}
	}
	return result, nil
}

func (cc *ClusterCache) listEvents(nsQuery *k8scommon.NamespaceQuery) (*corev1.EventList, error) {
	items, err := cc.eventLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	result := &corev1.EventList{}
	for _, item := range items {
		if nsQuery.Matches(item.Namespace) {
			result.Items = append(result.Items, *item.DeepCopy())
		}
	}
	return result, nil
}

func deploymentChannel(list *appsv1.DeploymentList, err error, numReads int) k8scommon.DeploymentListChannel {
	ch := k8scommon.DeploymentListChannel{List: make(chan *appsv1.DeploymentList, numReads), Error: make(chan error, numReads)}
	for i := 0; i < numReads; i++ {
		ch.List <- list
		ch.Error <- err
	}
	return ch
}

func statefulSetChannel(list *appsv1.StatefulSetList, err error, numReads int) k8scommon.StatefulSetListChannel {
	ch := k8scommon.StatefulSetListChannel{List: make(chan *appsv1.StatefulSetList, numReads), Error: make(chan error, numReads)}
	for i := 0; i < numReads; i++ {
		ch.List <- list
		ch.Error <- err
	}
	return ch
}

func daemonSetChannel(list *appsv1.DaemonSetList, err error, numReads int) k8scommon.DaemonSetListChannel {
	ch := k8scommon.DaemonSetListChannel{List: make(chan *appsv1.DaemonSetList, numReads), Error: make(chan error, numReads)}
	for i := 0; i < numReads; i++ {
		ch.List <- list
		ch.Error <- err
	}
	return ch
}

func replicaSetChannel(list *appsv1.ReplicaSetList, err error, numReads int) k8scommon.ReplicaSetListChannel {
	ch := k8scommon.ReplicaSetListChannel{List: make(chan *appsv1.ReplicaSetList, numReads), Error: make(chan error, numReads)}
	for i := 0; i < numReads; i++ {
		ch.List <- list
		ch.Error <- err
	}
	return ch
}

func jobChannel(list *batchv1.JobList, err error, numReads int) k8scommon.JobListChannel {
	ch := k8scommon.JobListChannel{List: make(chan *batchv1.JobList, numReads), Error: make(chan error, numReads)}
	for i := 0; i < numReads; i++ {
		ch.List <- list
		ch.Error <- err
	}
	return ch
}

func cronJobChannel(list *batchv1beta1.CronJobList, err error, numReads int) k8scommon.CronJobListChannel {
	ch := k8scommon.CronJobListChannel{List: make(chan *batchv1beta1.CronJobList, numReads), Error: make(chan error, numReads)}
	for i := 0; i < numReads; i++ {
		ch.List <- list
		ch.Error <- err
	}
	return ch
}

func podChannel(list *corev1.PodList, err error, numReads int) k8scommon.PodListChannel {
	ch := k8scommon.PodListChannel{List: make(chan *corev1.PodList, numReads), Error: make(chan error, numReads)}
	for i := 0; i < numReads; i++ {
		ch.List <- list
		ch.Error <- err
	}
	return ch
}

func eventChannel(list *corev1.EventList, err error, numReads int) k8scommon.EventListChannel {
	ch := k8scommon.EventListChannel{List: make(chan *corev1.EventList, numReads), Error: make(chan error, numReads)}
	for i := 0; i < numReads; i++ {
		ch.List <- list
		ch.Error <- err
	}
	return ch
}
