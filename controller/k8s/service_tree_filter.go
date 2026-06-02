/*
Copyright 2021 The DnsJia Authors.
WebSite:  https://github.com/dnsjia/luban
Email:    OpenSource@dnsjia.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8s

import (
	"strconv"

	k8smodel "github.com/dnsjia/luban/models/k8s"
	"github.com/dnsjia/luban/pkg/k8s/cronjob"
	"github.com/dnsjia/luban/pkg/k8s/daemonset"
	"github.com/dnsjia/luban/pkg/k8s/dataselect"
	"github.com/dnsjia/luban/pkg/k8s/deployment"
	"github.com/dnsjia/luban/pkg/k8s/job"
	"github.com/dnsjia/luban/pkg/k8s/parser"
	"github.com/dnsjia/luban/pkg/k8s/pods"
	"github.com/dnsjia/luban/pkg/k8s/statefulset"
	"github.com/dnsjia/luban/services"
	"github.com/gin-gonic/gin"
)

func parseTreeAwareDataSelect(c *gin.Context) (*dataselect.DataSelectQuery, bool) {
	dsQuery := parser.ParseDataSelectPathParameter(c)
	if c.Query("treeNodeId") == "" {
		if user := currentUser(c); user != nil && !user.Role.IsSuperAdmin() {
			dsQuery.PaginationQuery = dataselect.NoPagination
			return dsQuery, true
		}
		return dsQuery, false
	}
	dsQuery.PaginationQuery = dataselect.NoPagination
	return dsQuery, true
}

func serviceTreePageWindow(c *gin.Context, total int) (int, int) {
	itemsPerPage, err := strconv.Atoi(c.DefaultQuery("itemsPerPage", "10"))
	if err != nil || itemsPerPage <= 0 {
		return 0, total
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		page = 1
	}
	start := (page - 1) * itemsPerPage
	if start >= total {
		return total, total
	}
	end := start + itemsPerPage
	if end > total {
		end = total
	}
	return start, end
}

func workloadFilter(c *gin.Context, kind string) (*services.WorkloadResourceFilter, error) {
	treeNodeID := services.ParseTreeNodeID(c.Query("treeNodeId"))
	return services.BuildWorkloadResourceFilter(currentUser(c), c.Query("clusterId"), treeNodeID, kind)
}

func filterDeploymentListByTree(c *gin.Context, data *deployment.DeploymentList) error {
	filter, err := workloadFilter(c, k8smodel.ResourceKindDeployment)
	if err != nil || filter == nil || !filter.Enabled {
		return err
	}
	items := make([]deployment.Deployment, 0)
	for _, item := range data.Deployments {
		if filter.Match(item.ObjectMeta.Namespace, k8smodel.ResourceKindDeployment, item.ObjectMeta.Name, item.ObjectMeta.Labels) {
			items = append(items, item)
		}
	}
	data.ListMeta.TotalItems = len(items)
	start, end := serviceTreePageWindow(c, len(items))
	data.Deployments = items[start:end]
	return nil
}

func filterStatefulSetListByTree(c *gin.Context, data *statefulset.StatefulSetList) error {
	filter, err := workloadFilter(c, k8smodel.ResourceKindStatefulSet)
	if err != nil || filter == nil || !filter.Enabled {
		return err
	}
	items := make([]statefulset.StatefulSet, 0)
	for _, item := range data.StatefulSets {
		if filter.Match(item.ObjectMeta.Namespace, k8smodel.ResourceKindStatefulSet, item.ObjectMeta.Name, item.ObjectMeta.Labels) {
			items = append(items, item)
		}
	}
	data.ListMeta.TotalItems = len(items)
	start, end := serviceTreePageWindow(c, len(items))
	data.StatefulSets = items[start:end]
	return nil
}

func filterDaemonSetListByTree(c *gin.Context, data *daemonset.DaemonSetList) error {
	filter, err := workloadFilter(c, k8smodel.ResourceKindDaemonSet)
	if err != nil || filter == nil || !filter.Enabled {
		return err
	}
	items := make([]daemonset.DaemonSet, 0)
	for _, item := range data.DaemonSets {
		if filter.Match(item.ObjectMeta.Namespace, k8smodel.ResourceKindDaemonSet, item.ObjectMeta.Name, item.ObjectMeta.Labels) {
			items = append(items, item)
		}
	}
	data.ListMeta.TotalItems = len(items)
	start, end := serviceTreePageWindow(c, len(items))
	data.DaemonSets = items[start:end]
	return nil
}

func filterJobListByTree(c *gin.Context, data *job.JobList) error {
	filter, err := workloadFilter(c, k8smodel.ResourceKindJob)
	if err != nil || filter == nil || !filter.Enabled {
		return err
	}
	items := make([]job.Job, 0)
	for _, item := range data.Jobs {
		if filter.Match(item.ObjectMeta.Namespace, k8smodel.ResourceKindJob, item.ObjectMeta.Name, item.ObjectMeta.Labels) {
			items = append(items, item)
		}
	}
	data.ListMeta.TotalItems = len(items)
	start, end := serviceTreePageWindow(c, len(items))
	data.Jobs = items[start:end]
	return nil
}

func filterCronJobListByTree(c *gin.Context, data *cronjob.CronJobList) error {
	filter, err := workloadFilter(c, k8smodel.ResourceKindCronJob)
	if err != nil || filter == nil || !filter.Enabled {
		return err
	}
	items := make([]cronjob.CronJob, 0)
	for _, item := range data.Items {
		if filter.Match(item.ObjectMeta.Namespace, k8smodel.ResourceKindCronJob, item.ObjectMeta.Name, item.ObjectMeta.Labels) {
			items = append(items, item)
		}
	}
	data.ListMeta.TotalItems = len(items)
	start, end := serviceTreePageWindow(c, len(items))
	data.Items = items[start:end]
	return nil
}

func filterPodListByTree(c *gin.Context, data *pods.PodList) error {
	filter, err := workloadFilter(c, k8smodel.ResourceKindPod)
	if err != nil || filter == nil || !filter.Enabled {
		return err
	}
	items := make([]pods.Pod, 0)
	for _, item := range data.Pods {
		if filter.Match(item.ObjectMeta.Namespace, k8smodel.ResourceKindPod, item.ObjectMeta.Name, item.ObjectMeta.Labels) {
			items = append(items, item)
		}
	}
	data.ListMeta.TotalItems = len(items)
	start, end := serviceTreePageWindow(c, len(items))
	data.Pods = items[start:end]
	return nil
}
