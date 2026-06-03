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
	"github.com/dnsjia/luban/pkg/k8s/parser"
	"github.com/dnsjia/luban/pkg/k8s/pods"
	"github.com/gin-gonic/gin"
)

func GetPodsListController(c *gin.Context) {
	dataSelect, treeFilter := parseTreeAwareDataSelect(c)
	nsQuery := parser.ParseNamespacePathParameter(c)

	var data *pods.PodList
	channels, ok, err := cachedWorkloadChannels(c, nsQuery)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	if ok {
		data, err = pods.GetPodListFromChannels(channels, dataSelect)
	} else {
		client, clientErr := Init.ClusterID(c)
		if clientErr != nil {
			response.FailWithMessage(response.InternalServerError, clientErr.Error(), c)
			return
		}
		data, err = pods.GetPodsList(client, nsQuery, dataSelect)
	}
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	if treeFilter {
		if err := filterPodListByTree(c, data); err != nil {
			response.FailWithMessage(response.Forbidden, err.Error(), c)
			return
		}
	}

	response.OkWithData(data, c)
	return
}

func DeleteCollectionPodsController(c *gin.Context) {

	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	var podsData []k8s.RemovePodsData

	err = controller.CheckParams(c, &podsData)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	for _, item := range podsData {
		if !authorizeK8sPodAction(c, client, item.Namespace, item.PodName, k8s.ServiceTreeActionDelete) {
			return
		}
	}

	err = pods.DeleteCollectionPods(client, podsData)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.Ok(c)
	return
}

func DeletePodController(c *gin.Context) {

	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	namespace := parser.ParseNamespaceParameter(c)
	name := parser.ParseNameParameter(c)
	if !authorizeK8sPodAction(c, client, namespace, name, k8s.ServiceTreeActionDelete) {
		return
	}

	err = pods.DeletePod(client, namespace, name)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.Ok(c)
	return
}

func DetailPodController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	namespace := parser.ParseNamespaceParameter(c)
	name := parser.ParseNameParameter(c)
	if !authorizeK8sPodAction(c, client, namespace, name, k8s.ServiceTreeActionView) {
		return
	}

	podData, err := pods.GetPodDetail(client, namespace, name)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.OkWithData(podData, c)
	return
}
