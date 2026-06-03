package k8s

import (
	"github.com/dnsjia/luban/controller/response"
	k8smodel "github.com/dnsjia/luban/models/k8s"
	"github.com/dnsjia/luban/services"
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
)

func authorizeK8sResourceAction(c *gin.Context, namespace, kind, name, action string) bool {
	if err := services.AuthorizeK8sResourceAction(currentUser(c), c.Query("clusterId"), namespace, kind, name, action); err != nil {
		response.FailWithMessage(response.Forbidden, err.Error(), c)
		return false
	}
	return true
}

func authorizeK8sPodAction(c *gin.Context, client kubernetes.Interface, namespace, podName, action string) bool {
	if err := services.AuthorizeK8sPodAction(currentUser(c), client, c.Query("clusterId"), namespace, podName, action); err != nil {
		response.FailWithMessage(response.Forbidden, err.Error(), c)
		return false
	}
	return true
}

func authorizeK8sDeployment(c *gin.Context, namespace, name, action string) bool {
	return authorizeK8sResourceAction(c, namespace, k8smodel.ResourceKindDeployment, name, action)
}

func authorizeK8sStatefulSet(c *gin.Context, namespace, name, action string) bool {
	return authorizeK8sResourceAction(c, namespace, k8smodel.ResourceKindStatefulSet, name, action)
}

func authorizeK8sDaemonSet(c *gin.Context, namespace, name, action string) bool {
	return authorizeK8sResourceAction(c, namespace, k8smodel.ResourceKindDaemonSet, name, action)
}

func authorizeK8sJob(c *gin.Context, namespace, name, action string) bool {
	return authorizeK8sResourceAction(c, namespace, k8smodel.ResourceKindJob, name, action)
}

func authorizeK8sCronJob(c *gin.Context, namespace, name, action string) bool {
	return authorizeK8sResourceAction(c, namespace, k8smodel.ResourceKindCronJob, name, action)
}

func authorizeK8sService(c *gin.Context, namespace, name, action string) bool {
	return authorizeK8sResourceAction(c, namespace, k8smodel.ResourceKindService, name, action)
}

func authorizeK8sIngress(c *gin.Context, namespace, name, action string) bool {
	return authorizeK8sResourceAction(c, namespace, k8smodel.ResourceKindIngress, name, action)
}
