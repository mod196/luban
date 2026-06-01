<template>
  <div class="workload-page">
    <div class="workload-header">
      <div>
        <span class="workload-title">工作负载</span>
        <span class="workload-subtitle">Workload</span>
      </div>
      <a-space>
        <a-button type="primary">挂载工作负载</a-button>
        <a-button type="primary">使用镜像创建</a-button>
        <a-button type="primary">使用YAML创建资源</a-button>
        <a-button @click="getServiceTree">刷新</a-button>
      </a-space>
    </div>

    <div class="workload-body">
      <div class="service-tree-panel">
        <div class="service-tree-toolbar">
          <span class="service-tree-title">服务树</span>
          <a-space>
            <a-button size="small">新建节点</a-button>
            <a-button size="small">授权</a-button>
          </a-space>
        </div>
        <a-input-search
            v-model:value="data.treeKeyword"
            placeholder="搜索服务节点"
            style="margin-bottom: 12px"
            @search="getServiceTree"
        />
        <a-spin :spinning="data.treeLoading">
          <a-tree
              v-model:selectedKeys="data.selectedTreeKeys"
              :tree-data="data.serviceTreeData"
              default-expand-all
              @select="onSelectServiceTree"
          />
        </a-spin>
      </div>

      <div class="workload-content">
        <div class="current-tree-path">
          {{ serviceTreeContext.path || '请选择服务树节点' }}
        </div>

  <a-tabs v-model:activeKey="data.workload" @change="callback">

    <a-tab-pane key="1" tab="无状态">
      <Deployment></Deployment>
    </a-tab-pane>

    <a-tab-pane key="2" tab="有状态" force-render>
      <StatefulSet></StatefulSet>
    </a-tab-pane>

    <a-tab-pane key="3" tab="守护进程集">
      <DaemonSet></DaemonSet>
    </a-tab-pane>

    <a-tab-pane key="4" tab="任务">
      <Job></Job>
    </a-tab-pane>

    <a-tab-pane key="5" tab="定时任务">
      <CronJob></CronJob>
    </a-tab-pane>

    <a-tab-pane key="6" tab="容器组">
      <Pods></Pods>
    </a-tab-pane>

  </a-tabs>
      </div>
    </div>
  </div>
</template>

<script>
import Deployment from "./Deployment";
import Pods from "./Pods";
import {onMounted, provide, reactive} from "vue";
import StatefulSet from "./StatefulSet";
import DaemonSet from "./DaemonSet";
import Job from "./Job";
import CronJob from "./CronJob";
import {GetK8sServiceTree} from "../../api/k8s";
import {GetStorage} from "../../plugin/state/stroge";
export default {
  name: "WorkLoad",
  setup() {

    const callback = val => {
      localStorage.setItem("workload", val)
    };
    const data = reactive({
          workload: "",
          treeLoading: false,
          treeKeyword: "",
          selectedTreeKeys: [],
          serviceTreeData: [],
          serviceTreeNodeMap: {},
    })

    const serviceTreeContext = reactive({
      treeNodeId: "",
      namespace: localStorage.getItem("namespace") || "default",
      env: "",
      service: "",
      path: "",
    })
    provide("serviceTreeContext", serviceTreeContext)

    const getWorkloadTable = () => {
      data.workload = localStorage.getItem("workload");
      if (data.workload === "" || data.workload === undefined || data.workload === null) {
        data.workload = "1"
      }
    }

    const toTreeData = (nodes) => {
      if (!nodes) {
        return []
      }
      return nodes.map(item => {
        data.serviceTreeNodeMap[String(item.id)] = item
        return {
          key: String(item.id),
          title: item.name,
          children: toTreeData(item.children),
        }
      })
    }

    const findFirstEnvNode = (nodes) => {
      for (const item of nodes || []) {
        if (item.nodeType === "env") {
          return item
        }
        const child = findFirstEnvNode(item.children)
        if (child) {
          return child
        }
      }
      return null
    }

    const setServiceTreeContext = (node) => {
      if (!node) {
        return
      }
      serviceTreeContext.treeNodeId = String(node.id)
      serviceTreeContext.namespace = node.namespace || serviceTreeContext.namespace
      serviceTreeContext.env = node.nodeType === "env" ? node.name : ""
      serviceTreeContext.service = node.path ? node.path.split(" / ")[1] || "" : ""
      serviceTreeContext.path = node.path || node.name
      if (serviceTreeContext.namespace) {
        localStorage.setItem("namespace", serviceTreeContext.namespace)
      }
      localStorage.setItem("serviceTreeNodeId", serviceTreeContext.treeNodeId)
    }

    const onSelectServiceTree = (selectedKeys) => {
      if (!selectedKeys || selectedKeys.length === 0) {
        return
      }
      const node = data.serviceTreeNodeMap[String(selectedKeys[0])]
      setServiceTreeContext(node)
    }

    const getServiceTree = () => {
      data.treeLoading = true
      data.serviceTreeNodeMap = {}
      const cs = GetStorage()
      const clusterId = cs ? cs.clusterId : ""
      GetK8sServiceTree(clusterId).then(res => {
        if (res.errCode === 0) {
          const treeData = res.data || []
          data.serviceTreeData = toTreeData(treeData)
          const storedTreeNodeId = localStorage.getItem("serviceTreeNodeId")
          const selected = data.serviceTreeNodeMap[storedTreeNodeId] || findFirstEnvNode(treeData)
          if (selected) {
            data.selectedTreeKeys = [String(selected.id)]
            setServiceTreeContext(selected)
          }
        }
      }).finally(() => {
        data.treeLoading = false
      })
    }

    onMounted(() => {
      getWorkloadTable()
      getServiceTree()
    })

    return {
      callback,
      data,
      serviceTreeContext,
      getServiceTree,
      onSelectServiceTree,
    }
  },

  components: {
    CronJob,
    Job,
    DaemonSet,
    Deployment,
    StatefulSet,
    Pods,
  }
}
</script>

<style scoped>
.workload-page {
  background: #FFFFFF;
}
.workload-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
  padding: 8px 10px 18px 10px;
}
.workload-title {
  color: #262626;
  font-size: 24px;
  font-weight: 600;
}
.workload-subtitle {
  color: #8c8c8c;
  margin-left: 10px;
}
.workload-body {
  display: flex;
  gap: 12px;
}
.service-tree-panel {
  border: 1px solid #f0f0f0;
  flex: 0 0 300px;
  min-height: 620px;
  padding: 16px;
}
.service-tree-toolbar {
  align-items: center;
  display: flex;
  justify-content: space-between;
  margin-bottom: 12px;
}
.service-tree-title {
  color: #262626;
  font-size: 16px;
  font-weight: 600;
}
.workload-content {
  border: 1px solid #f0f0f0;
  flex: 1;
  min-width: 0;
  padding: 16px;
}
.current-tree-path {
  color: #262626;
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 12px;
}
</style>
