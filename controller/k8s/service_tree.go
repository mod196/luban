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
	"strings"

	"github.com/dnsjia/luban/controller"
	"github.com/dnsjia/luban/controller/response"
	"github.com/dnsjia/luban/models"
	"github.com/dnsjia/luban/pkg/k8s/Init"
	k8scache "github.com/dnsjia/luban/pkg/k8s/cache"
	"github.com/dnsjia/luban/services"
	"github.com/gin-gonic/gin"
)

func GetServiceTreeController(c *gin.Context) {
	user := currentUser(c)
	data, err := services.ListK8sServiceTree(user, c.Query("clusterId"))
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	response.OkWithData(data, c)
}

func GetWorkloadsController(c *gin.Context) {
	switch strings.ToLower(c.DefaultQuery("kind", "deployment")) {
	case "deployment", "deployments", "无状态":
		GetDeploymentList(c)
	case "statefulset", "statefulsets", "有状态":
		GetStatefulSetListController(c)
	case "daemonset", "daemonsets", "守护进程集":
		GetDaemonSetListController(c)
	case "job", "jobs", "任务":
		GetJobListController(c)
	case "cronjob", "cronjobs", "定时任务":
		GetCronJobListController(c)
	case "pod", "pods", "容器组":
		GetPodsListController(c)
	default:
		response.FailWithMessage(response.ParamError, "不支持的工作负载类型", c)
	}
}

func CreateServiceTreeNodeController(c *gin.Context) {
	var req services.ServiceTreeNodeCreateRequest
	if err := controller.CheckParams(c, &req); err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	node, err := services.CreateK8sServiceTreeNode(req)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	response.OkWithData(node, c)
}

func CreateServiceTreeBindingController(c *gin.Context) {
	var req services.ServiceTreeBindingRequest
	if err := controller.CheckParams(c, &req); err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	if req.CreatedBy == "" {
		if user := currentUser(c); user != nil {
			req.CreatedBy = user.UserName
		}
	}
	if err := services.UpsertK8sServiceTreeBinding(req); err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	response.Ok(c)
}

func GetServiceTreeBindingsController(c *gin.Context) {
	data, err := services.ListK8sServiceTreeBindings(
		c.Query("clusterId"),
		services.ParseTreeNodeID(c.Query("treeNodeId")),
	)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	response.OkWithData(data, c)
}

func GetServiceTreeUnclassifiedController(c *gin.Context) {
	data, err := services.ListK8sServiceTreeUnclassified(c.Query("clusterId"))
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	response.OkWithData(data, c)
}

func CreateServiceTreeBindingRuleController(c *gin.Context) {
	var req services.ServiceTreeBindingRuleRequest
	if err := controller.CheckParams(c, &req); err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	rule, err := services.CreateK8sServiceTreeBindingRule(req)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	response.OkWithData(rule, c)
}

func GetServiceTreeBindingRulesController(c *gin.Context) {
	data, err := services.ListK8sServiceTreeBindingRules(c.Query("clusterId"), c.Query("namespace"))
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	response.OkWithData(data, c)
}

func CreateServiceTreePolicyController(c *gin.Context) {
	var req services.TreePolicyRequest
	if err := controller.CheckParams(c, &req); err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	createdBy := ""
	if user := currentUser(c); user != nil {
		createdBy = user.UserName
	}
	policy, err := services.CreateK8sTreePolicy(req, createdBy)
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	response.OkWithData(policy, c)
}

func GetServiceTreePoliciesController(c *gin.Context) {
	data, err := services.ListK8sTreePolicies()
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	response.OkWithData(data, c)
}

func GetServiceTreePrincipalsController(c *gin.Context) {
	data, err := services.ListK8sTreePrincipals(c.DefaultQuery("principalType", "user"), c.Query("keyword"))
	if err != nil {
		response.FailWithMessage(response.ParamError, err.Error(), c)
		return
	}
	response.OkWithData(data, c)
}

func GetServiceTreeAppLabelsController(c *gin.Context) {
	data, err := services.ListK8sAppLabelOptionsFromInventory(c.DefaultQuery("clusterId", "1"), c.Query("namespace"), c.Query("kind"))
	if err == nil && len(data) > 0 {
		response.OkWithData(data, c)
		return
	}

	client, err := Init.ClusterID(c)
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	data, err = services.ListK8sAppLabelOptions(client, c.Query("namespace"), c.Query("kind"))
	if err != nil {
		response.FailWithMessage(response.InternalServerError, err.Error(), c)
		return
	}
	response.OkWithData(data, c)
}

func GetK8sCacheStatusController(c *gin.Context) {
	response.OkWithData(k8scache.Global.Status(c.Query("clusterId")), c)
}

func currentUser(c *gin.Context) *models.User {
	v, ok := c.Get("user")
	if !ok {
		return nil
	}
	user, ok := v.(models.User)
	if !ok {
		return nil
	}
	return &user
}
