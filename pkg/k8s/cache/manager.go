package cache

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dnsjia/luban/common"
	"github.com/dnsjia/luban/models"
	"go.uber.org/zap"
	informers "k8s.io/client-go/informers"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	networkingv1informers "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	k8stoolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var Global = NewManager()

type Manager struct {
	mu       sync.RWMutex
	clusters map[string]*ClusterCache
	ctx      context.Context
	enabled  bool
}

type ClusterCache struct {
	ClusterID   string
	ClusterName string

	client  kubernetes.Interface
	factory informers.SharedInformerFactory
	stopCh  chan struct{}

	deploymentLister  appsv1listers.DeploymentLister
	statefulSetLister appsv1listers.StatefulSetLister
	daemonSetLister   appsv1listers.DaemonSetLister
	replicaSetLister  appsv1listers.ReplicaSetLister
	jobLister         batchv1listers.JobLister
	cronJobLister     batchv1listers.CronJobLister
	podLister         corev1listers.PodLister
	serviceLister     corev1listers.ServiceLister
	eventLister       corev1listers.EventLister
	namespaceLister   corev1listers.NamespaceLister
	nodeLister        corev1listers.NodeLister
	ingressLister     networkingv1listers.IngressLister

	synced []k8stoolscache.InformerSynced

	mu           sync.RWMutex
	ready        bool
	stopped      bool
	lastSyncTime time.Time
	lastError    string
}

type Status struct {
	Enabled      bool   `json:"enabled"`
	ClusterID    string `json:"clusterId"`
	ClusterName  string `json:"clusterName"`
	Ready        bool   `json:"ready"`
	Stopped      bool   `json:"stopped"`
	LastSyncTime string `json:"lastSyncTime"`
	LastError    string `json:"lastError"`
}

func NewManager() *Manager {
	return &Manager{
		clusters: map[string]*ClusterCache{},
		enabled:  true,
	}
}

func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	m.ctx = ctx
	m.enabled = common.CONFIG.K8sCache.Enabled
	m.mu.Unlock()
	if !common.CONFIG.K8sCache.Enabled {
		logInfo("k8s informer cache disabled")
		return
	}

	var clusters []models.K8SCluster
	if err := common.DB.Find(&clusters).Error; err != nil {
		logError("list k8s clusters for cache failed", err)
		return
	}
	for _, cluster := range clusters {
		if err := m.StartCluster(cluster); err != nil {
			logError("start k8s cluster cache failed", err, zap.Uint("clusterId", cluster.ID), zap.String("clusterName", cluster.ClusterName))
		}
	}
}

func (m *Manager) StartCluster(cluster models.K8SCluster) error {
	if !common.CONFIG.K8sCache.Enabled {
		return nil
	}
	clusterID := strconv.FormatUint(uint64(cluster.ID), 10)
	if clusterID == "0" || cluster.KubeConfig == "" {
		return fmt.Errorf("invalid k8s cluster cache config: id=%s", clusterID)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.KubeConfig))
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	resyncPeriod := parseDuration(common.CONFIG.K8sCache.ResyncPeriod, 10*time.Hour)
	cc := newClusterCache(clusterID, cluster.ClusterName, client, resyncPeriod)

	m.mu.Lock()
	if old := m.clusters[clusterID]; old != nil {
		old.Stop()
	}
	m.clusters[clusterID] = cc
	ctx := m.ctx
	m.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	cc.Start(ctx, parseDuration(common.CONFIG.K8sCache.StartupTimeout, 30*time.Second))
	return nil
}

func (m *Manager) StopCluster(clusterID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cc := m.clusters[clusterID]; cc != nil {
		cc.Stop()
		delete(m.clusters, clusterID)
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cc := range m.clusters {
		cc.Stop()
		delete(m.clusters, id)
	}
}

func (m *Manager) GetCluster(clusterID string) (*ClusterCache, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cc, ok := m.clusters[clusterID]
	if !ok || cc == nil || !cc.IsReady() {
		return nil, false
	}
	return cc, true
}

func (m *Manager) Status(clusterID string) []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if clusterID != "" {
		if cc := m.clusters[clusterID]; cc != nil {
			return []Status{cc.Status(m.enabled)}
		}
		return []Status{{Enabled: m.enabled, ClusterID: clusterID, Ready: false, LastError: "cache not started"}}
	}
	items := make([]Status, 0, len(m.clusters))
	for _, cc := range m.clusters {
		items = append(items, cc.Status(m.enabled))
	}
	return items
}

func newClusterCache(clusterID, clusterName string, client kubernetes.Interface, resyncPeriod time.Duration) *ClusterCache {
	factory := informers.NewSharedInformerFactory(client, resyncPeriod)
	cc := &ClusterCache{
		ClusterID:   clusterID,
		ClusterName: clusterName,
		client:      client,
		factory:     factory,
		stopCh:      make(chan struct{}),
	}
	cc.register(factory)
	return cc
}

func (cc *ClusterCache) register(factory informers.SharedInformerFactory) {
	deployments := factory.Apps().V1().Deployments()
	statefulSets := factory.Apps().V1().StatefulSets()
	daemonSets := factory.Apps().V1().DaemonSets()
	replicaSets := factory.Apps().V1().ReplicaSets()
	jobs := factory.Batch().V1().Jobs()
	cronJobs := factory.Batch().V1().CronJobs()
	pods := factory.Core().V1().Pods()
	services := factory.Core().V1().Services()
	events := factory.Core().V1().Events()
	namespaces := factory.Core().V1().Namespaces()
	nodes := factory.Core().V1().Nodes()
	ingresses := factory.Networking().V1().Ingresses()

	cc.deploymentLister = deployments.Lister()
	cc.statefulSetLister = statefulSets.Lister()
	cc.daemonSetLister = daemonSets.Lister()
	cc.replicaSetLister = replicaSets.Lister()
	cc.jobLister = jobs.Lister()
	cc.cronJobLister = cronJobs.Lister()
	cc.podLister = pods.Lister()
	cc.serviceLister = services.Lister()
	cc.eventLister = events.Lister()
	cc.namespaceLister = namespaces.Lister()
	cc.nodeLister = nodes.Lister()
	cc.ingressLister = ingresses.Lister()

	cc.addHandler(deployments, "apps/v1", "deployment")
	cc.addHandler(statefulSets, "apps/v1", "statefulset")
	cc.addHandler(daemonSets, "apps/v1", "daemonset")
	cc.addHandler(jobs, "batch/v1", "job")
	cc.addHandler(cronJobs, "batch/v1", "cronjob")
	cc.addHandler(pods, "v1", "pod")
	cc.addHandler(services, "v1", "service")
	cc.addHandler(namespaces, "v1", "namespace")
	cc.addHandler(nodes, "v1", "node")
	cc.addHandler(ingresses, "networking.k8s.io/v1", "ingress")

	cc.synced = []k8stoolscache.InformerSynced{
		deployments.Informer().HasSynced,
		statefulSets.Informer().HasSynced,
		daemonSets.Informer().HasSynced,
		replicaSets.Informer().HasSynced,
		jobs.Informer().HasSynced,
		cronJobs.Informer().HasSynced,
		pods.Informer().HasSynced,
		services.Informer().HasSynced,
		events.Informer().HasSynced,
		namespaces.Informer().HasSynced,
		nodes.Informer().HasSynced,
		ingresses.Informer().HasSynced,
	}
}

type informerWithHandler interface {
	Informer() k8stoolscache.SharedIndexInformer
}

func (cc *ClusterCache) addHandler(informer informerWithHandler, apiVersion, kind string) {
	informer.Informer().AddEventHandler(k8stoolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cc.upsertObject(apiVersion, kind, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			cc.upsertObject(apiVersion, kind, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			cc.deleteObject(obj)
		},
	})
}

func (cc *ClusterCache) Start(ctx context.Context, startupTimeout time.Duration) {
	cc.factory.Start(cc.stopCh)
	done := make(chan bool, 1)
	go func() {
		done <- k8stoolscache.WaitForCacheSync(cc.stopCh, cc.synced...)
	}()
	go func() {
		timer := time.NewTimer(startupTimeout)
		defer timer.Stop()
		select {
		case ok := <-done:
			cc.setReady(ok, "")
		case <-timer.C:
			cc.setReady(false, "cache sync timeout")
			ok := <-done
			cc.setReady(ok, "")
		case <-ctx.Done():
			cc.Stop()
		}
	}()
}

func (cc *ClusterCache) Stop() {
	cc.mu.Lock()
	if !cc.stopped {
		close(cc.stopCh)
		cc.stopped = true
		cc.ready = false
	}
	cc.mu.Unlock()
}

func (cc *ClusterCache) IsReady() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.ready && !cc.stopped
}

func (cc *ClusterCache) Status(enabled bool) Status {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	lastSync := ""
	if !cc.lastSyncTime.IsZero() {
		lastSync = cc.lastSyncTime.Format(time.RFC3339)
	}
	return Status{
		Enabled:      enabled,
		ClusterID:    cc.ClusterID,
		ClusterName:  cc.ClusterName,
		Ready:        cc.ready,
		Stopped:      cc.stopped,
		LastSyncTime: lastSync,
		LastError:    cc.lastError,
	}
}

func (cc *ClusterCache) setReady(ready bool, errMsg string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.ready = ready
	if ready {
		cc.lastSyncTime = time.Now()
		cc.lastError = ""
		return
	}
	if errMsg != "" {
		cc.lastError = errMsg
	}
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}

func logInfo(msg string, fields ...zap.Field) {
	if common.LOG != nil {
		common.LOG.Info(msg, fields...)
	}
}

func logError(msg string, err error, fields ...zap.Field) {
	if common.LOG != nil {
		fields = append(fields, zap.Any("err", err))
		common.LOG.Error(msg, fields...)
	}
}

var (
	_ informerWithHandler = corev1informers.PodInformer(nil)
	_ informerWithHandler = corev1informers.ServiceInformer(nil)
	_ informerWithHandler = corev1informers.EventInformer(nil)
	_ informerWithHandler = corev1informers.NamespaceInformer(nil)
	_ informerWithHandler = corev1informers.NodeInformer(nil)
	_ informerWithHandler = appsv1informers.DeploymentInformer(nil)
	_ informerWithHandler = appsv1informers.StatefulSetInformer(nil)
	_ informerWithHandler = appsv1informers.DaemonSetInformer(nil)
	_ informerWithHandler = appsv1informers.ReplicaSetInformer(nil)
	_ informerWithHandler = batchv1informers.JobInformer(nil)
	_ informerWithHandler = batchv1informers.CronJobInformer(nil)
	_ informerWithHandler = networkingv1informers.IngressInformer(nil)
)
