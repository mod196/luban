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
	"github.com/dnsjia/luban/controller"
	"github.com/dnsjia/luban/controller/response"
	"github.com/dnsjia/luban/models/k8s"
	"github.com/dnsjia/luban/pkg/k8s/Init"
	"github.com/dnsjia/luban/pkg/k8s/job"
	"github.com/dnsjia/luban/pkg/k8s/parser"
	"github.com/gin-gonic/gin"
)

func GetJobListController(c *gin.Context) {
	dataSelect, treeFilter := parseTreeAwareDataSelect(c)
	nsQuery := parser.ParseNamespacePathParameter(c)

	var data *job.JobList
	channels, ok, err := cachedWorkloadChannels(c, nsQuery)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	if ok {
		data, err = job.GetJobListFromChannels(channels, dataSelect)
	} else {
		client, clientErr := Init.ClusterID(c)
		if clientErr != nil {
			response.FailWithMessage(response.InternalServerError, clientErr.Error(), c)
			return
		}
		data, err = job.GetJobList(client, nsQuery, dataSelect)
	}
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	if treeFilter {
		if err := filterJobListByTree(c, data); err != nil {
			response.FailWithMessage(response.Forbidden, err.Error(), c)
			return
		}
	}

	response.OkWithData(data, c)
	return
}

func DeleteJobController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	namespace := parser.ParseNamespaceParameter(c)
	name := parser.ParseNameParameter(c)
	if !authorizeK8sJob(c, namespace, name, k8s.ServiceTreeActionDelete) {
		return
	}

	err = job.DeleteJob(client, namespace, name)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.Ok(c)
	return
}

func DeleteCollectionJobController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	var jobList []k8s.JobData

	err = controller.CheckParams(c, &jobList)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	for _, item := range jobList {
		if !authorizeK8sJob(c, item.Namespace, item.Name, k8s.ServiceTreeActionDelete) {
			return
		}
	}

	err = job.DeleteCollectionJob(client, jobList)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.Ok(c)
	return
}

func ScaleJobController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	var scaleData k8s.ScaleJob
	err = controller.CheckParams(c, &scaleData)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	if !authorizeK8sJob(c, scaleData.Namespace, scaleData.Name, k8s.ServiceTreeActionScale) {
		return
	}

	err = job.ScaleJob(client, scaleData.Namespace, scaleData.Name, scaleData.Number)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.Ok(c)
	return
}

func DetailJobController(c *gin.Context) {

	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	namespace := parser.ParseNamespaceParameter(c)
	name := parser.ParseNameParameter(c)
	if !authorizeK8sJob(c, namespace, name, k8s.ServiceTreeActionView) {
		return
	}

	result, err := job.GetJobDetail(client, namespace, name)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.OkWithData(result, c)
	return
}
