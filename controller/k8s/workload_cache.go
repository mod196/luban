package k8s

import (
	"errors"

	"github.com/dnsjia/luban/common"
	k8scache "github.com/dnsjia/luban/pkg/k8s/cache"
	k8scommon "github.com/dnsjia/luban/pkg/k8s/common"
	"github.com/gin-gonic/gin"
)

var errK8sCacheNotReady = errors.New("k8s informer cache 未就绪")

func cachedWorkloadChannels(c *gin.Context, nsQuery *k8scommon.NamespaceQuery) (*k8scommon.ResourceChannels, bool, error) {
	if !common.CONFIG.K8sCache.Enabled {
		return nil, false, nil
	}
	channels, ok := k8scache.ResourceChannels(c.DefaultQuery("clusterId", "1"), nsQuery, 1)
	if ok {
		return channels, true, nil
	}
	if common.CONFIG.K8sCache.FallbackDirect {
		return nil, false, nil
	}
	return nil, false, errK8sCacheNotReady
}
