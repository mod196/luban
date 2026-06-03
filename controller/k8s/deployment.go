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
	"fmt"
	"github.com/dnsjia/luban/common"
	"github.com/dnsjia/luban/controller"
	"github.com/dnsjia/luban/controller/response"
	"github.com/dnsjia/luban/models/k8s"
	"github.com/dnsjia/luban/pkg/k8s/Init"
	"github.com/dnsjia/luban/pkg/k8s/deployment"
	"github.com/dnsjia/luban/pkg/k8s/parser"
	"github.com/dnsjia/luban/pkg/k8s/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
)

func GetDeploymentList(c *gin.Context) {
	dataSelect, treeFilter := parseTreeAwareDataSelect(c)
	nsQuery := parser.ParseNamespacePathParameter(c)

	var data *deployment.DeploymentList
	channels, ok, err := cachedWorkloadChannels(c, nsQuery)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	if ok {
		data, err = deployment.GetDeploymentListFromChannels(channels, dataSelect)
	} else {
		client, clientErr := Init.ClusterID(c)
		if clientErr != nil {
			response.FailWithMessage(response.InternalServerError, clientErr.Error(), c)
			return
		}
		data, err = deployment.GetDeploymentList(client, nsQuery, dataSelect)
	}
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	if treeFilter {
		if err := filterDeploymentListByTree(c, data); err != nil {
			response.FailWithMessage(response.Forbidden, err.Error(), c)
			return
		}
	}

	response.OkWithData(data, c)
	return
}

func DeleteCollectionDeployment(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	var deploymentList []k8s.RemoveDeploymentData

	err = controller.CheckParams(c, &deploymentList)
	if err != nil {
		response.FailWithMessage(http.StatusNotFound, err.Error(), c)
		return
	}
	for _, item := range deploymentList {
		if !authorizeK8sDeployment(c, item.Namespace, item.DeploymentName, k8s.ServiceTreeActionDelete) {
			return
		}
	}

	err = deployment.DeleteCollectionDeployment(client, deploymentList)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.Ok(c)
	return
}

func DeleteDeployment(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	var deploymentData k8s.RemoveDeploymentToServiceData

	err2 := controller.CheckParams(c, &deploymentData)
	if err2 != nil {
		response.FailWithMessage(http.StatusNotFound, err2.Error(), c)
		return
	}
	if !authorizeK8sDeployment(c, deploymentData.Namespace, deploymentData.DeploymentName, k8s.ServiceTreeActionDelete) {
		return
	}
	if deploymentData.IsDeleteService && deploymentData.ServiceName != "" {
		if !authorizeK8sService(c, deploymentData.Namespace, deploymentData.ServiceName, k8s.ServiceTreeActionDelete) {
			return
		}
	}

	err = deployment.DeleteDeployment(client, deploymentData.Namespace, deploymentData.DeploymentName)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	common.LOG.Info(fmt.Sprintf("deployment：%v, 已删除", deploymentData.DeploymentName))

	if deploymentData.IsDeleteService {
		serviceErr := service.DeleteService(client, deploymentData.Namespace, deploymentData.ServiceName)

		if serviceErr != nil {
			common.LOG.Error("删除相关Service出错", zap.Any("err: ", serviceErr))
			response.FailWithMessage(response.InternalServerError, err.Error(), c)
			return
		}
	}
	response.Ok(c)
	return
}

func ScaleDeployment(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}

	var scaleData k8s.ScaleDeployment

	err2 := controller.CheckParams(c, &scaleData)
	if err2 != nil {
		response.FailWithMessage(http.StatusNotFound, err2.Error(), c)
		return
	}
	if !authorizeK8sDeployment(c, scaleData.Namespace, scaleData.DeploymentName, k8s.ServiceTreeActionScale) {
		return
	}

	err = deployment.ScaleDeployment(client, scaleData.Namespace, scaleData.DeploymentName, *scaleData.ScaleNumber)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}

	response.Ok(c)
	return
}

func RestartDeploymentController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	var restartDeployment k8s.RestartDeployment
	err2 := controller.CheckParams(c, &restartDeployment)
	if err2 != nil {
		response.FailWithMessage(response.ParamError, err2.Error(), c)
		return
	}
	if !authorizeK8sDeployment(c, restartDeployment.Namespace, restartDeployment.DeploymentName, k8s.ServiceTreeActionRestart) {
		return
	}
	err3 := deployment.RestartDeployment(client, restartDeployment.DeploymentName, restartDeployment.Namespace)
	if err3 != nil {
		response.FailWithMessage(response.InternalServerError, err3.Error(), c)
		return
	}
	response.Ok(c)
	return

}

func GetDeploymentToServiceController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}

	var Deployment k8s.RestartDeployment
	err2 := controller.CheckParams(c, &Deployment)
	if err2 != nil {
		response.FailWithMessage(response.ParamError, err2.Error(), c)
		return
	}
	if !authorizeK8sDeployment(c, Deployment.Namespace, Deployment.DeploymentName, k8s.ServiceTreeActionView) {
		return
	}

	data, err := service.GetToService(client, Deployment.Namespace, Deployment.DeploymentName)
	if err != nil {
		response.FailWithMessage(response.ERROR, err.Error(), c)
		return
	}
	response.OkWithData(data, c)
	return
}

func DetailDeploymentController(c *gin.Context) {

	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	namespace := parser.ParseNamespaceParameter(c)
	name := parser.ParseNameParameter(c)
	if !authorizeK8sDeployment(c, namespace, name, k8s.ServiceTreeActionView) {
		return
	}

	data, err := deployment.GetDeploymentDetail(client, namespace, name)

	if err != nil {
		response.FailWithMessage(response.ERROR, err.Error(), c)
		return
	}
	response.OkWithData(data, c)
}

func RollBackDeploymentController(c *gin.Context) {
	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	var rollback k8s.RollbackDeployment

	rollbackParamsErr := controller.CheckParams(c, &rollback)
	if rollbackParamsErr != nil {
		response.FailWithMessage(response.ParamError, rollbackParamsErr.Error(), c)
		return
	}
	if !authorizeK8sDeployment(c, rollback.Namespace, rollback.DeploymentName, k8s.ServiceTreeActionYamlEdit) {
		return
	}
	rollbackErr := deployment.RollbackDeployment(client, rollback.DeploymentName, rollback.Namespace, *rollback.ReVersion)

	if rollbackErr != nil {
		response.FailWithMessage(response.ERROR, rollbackErr.Error(), c)
		return
	}
	response.Ok(c)
}
