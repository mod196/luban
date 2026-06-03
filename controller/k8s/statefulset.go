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
	"github.com/dnsjia/luban/pkg/k8s/statefulset"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetStatefulSetListController(c *gin.Context) {
	dataSelect, treeFilter := parseTreeAwareDataSelect(c)
	nsQuery := parser.ParseNamespacePathParameter(c)

	var data *statefulset.StatefulSetList
	channels, ok, err := cachedWorkloadChannels(c, nsQuery)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	if ok {
		data, err = statefulset.GetStatefulSetListFromChannels(channels, dataSelect)
	} else {
		client, clientErr := Init.ClusterID(c)
		if clientErr != nil {
			response.FailWithMessage(response.InternalServerError, clientErr.Error(), c)
			return
		}
		data, err = statefulset.GetStatefulSetList(client, nsQuery, dataSelect)
	}
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	if treeFilter {
		if err := filterStatefulSetListByTree(c, data); err != nil {
			response.FailWithMessage(response.Forbidden, err.Error(), c)
			return
		}
	}

	response.OkWithData(data, c)
	return
}

func DeleteCollectionStatefulSetController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	var statefulSetList []k8s.StatefulSetData

	err = controller.CheckParams(c, &statefulSetList)
	if err != nil {
		response.FailWithMessage(http.StatusNotFound, err.Error(), c)
		return
	}
	for _, item := range statefulSetList {
		if !authorizeK8sStatefulSet(c, item.Namespace, item.Name, k8s.ServiceTreeActionDelete) {
			return
		}
	}

	err = statefulset.DeleteCollectionStatefulSet(client, statefulSetList)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.Ok(c)
	return
}

func DeleteStatefulSetController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	namespace := parser.ParseNamespaceParameter(c)
	name := parser.ParseNameParameter(c)
	if !authorizeK8sStatefulSet(c, namespace, name, k8s.ServiceTreeActionDelete) {
		return
	}

	err = statefulset.DeleteStatefulSet(client, namespace, name)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.Ok(c)
	return
}

func RestartStatefulSetController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	var statefulSetData k8s.StatefulSetData
	err2 := controller.CheckParams(c, &statefulSetData)
	if err2 != nil {
		response.FailWithMessage(response.ParamError, err2.Error(), c)
		return
	}
	if !authorizeK8sStatefulSet(c, statefulSetData.Namespace, statefulSetData.Name, k8s.ServiceTreeActionRestart) {
		return
	}
	err3 := statefulset.RestartStatefulSet(client, statefulSetData.Name, statefulSetData.Namespace)
	if err3 != nil {
		response.FailWithMessage(response.InternalServerError, err3.Error(), c)
		return
	}
	response.Ok(c)
	return
}

func ScaleStatefulSetController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	var scaleData k8s.ScaleStatefulSet

	err2 := controller.CheckParams(c, &scaleData)
	if err2 != nil {
		response.FailWithMessage(response.ParamError, err2.Error(), c)
		return
	}
	if !authorizeK8sStatefulSet(c, scaleData.Namespace, scaleData.Name, k8s.ServiceTreeActionScale) {
		return
	}

	err = statefulset.ScaleStatefulSet(client, scaleData.Namespace, scaleData.Name, *scaleData.ScaleNumber)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.Ok(c)
	return
}

func DetailStatefulSetController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	namespace := parser.ParseNamespaceParameter(c)
	name := parser.ParseNameParameter(c)
	dataSelect := parser.ParseDataSelectPathParameter(c)
	if !authorizeK8sStatefulSet(c, namespace, name, k8s.ServiceTreeActionView) {
		return
	}

	data, err := statefulset.GetStatefulSetDetail(client, dataSelect, namespace, name)

	if err != nil {
		response.FailWithMessage(response.ERROR, err.Error(), c)
		return
	}
	response.OkWithData(data, c)
}
