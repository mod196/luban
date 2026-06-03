<template>
  <div class="pod-log-page">
    <div class="pod-log-header">
      <a-space align="center">
        <a-tooltip title="返回">
          <a-button class="pod-log-back" type="text" aria-label="返回" @click="goBack">
            <template #icon>
              <ArrowLeftOutlined />
            </template>
          </a-button>
        </a-tooltip>
        <div>
          <div class="pod-log-title">容器日志</div>
          <div class="pod-log-meta">
            <span>Namespace：{{ currentNamespace || '-' }}</span>
            <span>Pod：{{ selectedPodName || routePodName || '-' }}</span>
          </div>
        </div>
      </a-space>

      <a-button type="primary" :disabled="!canReturnTarget" @click="goBack">{{ returnButtonLabel }}</a-button>
    </div>

    <div class="pod-log-toolbar">
      <a-space :size="12" wrap>
        Container：
        <a-select v-model:value="data.container" style="min-width: 200px; max-width: 600px" @change="handleChange">
          <a-select-opt-group label="容器">
            <a-select-option :value=k v-for="(k, index) in data.containerData" :key="index">
              {{ k }}
            </a-select-option>
          </a-select-opt-group>

          <a-select-opt-group label="初始容器">
            <a-select-option :value=k v-for="(k, index) in data.initContainerData" :key="index">
              {{ k }}
            </a-select-option>
          </a-select-opt-group>
        </a-select>
        Pod：
        <a-select v-model:value="data.pod" placeholder="请选择容器" style="min-width: 200px; max-width: 800px" @change="handleLogChange">
          <a-select-option :value=k v-for="(k, index) in data.podNameData" :key="index">
            {{ k }}
          </a-select-option>
        </a-select>

        <a-button type="primary" @click="GetLog()">刷新</a-button>
        自动刷新/1秒：<a-switch v-model:checked="data.autoRefresh" checked-children="开" un-checked-children="关" @change="refreshLog" />
      </a-space>

      <a-tooltip color="#ffffff" :overlayStyle="{'font-size': '12px', 'max-width': '400px'}" placement="bottom">
        <template #title>
            <span style="color: #666">下载日志</span>
        </template>
        <DownloadOutlined class="pod-log-download" @click="downLoadLogFile()"/>
      </a-tooltip>
    </div>

    <div id="filelog-container" style="height: 600px; overflow-y: scroll; background: #404040; color: #dedede; padding: 10px;">
      <div id="viewLog" v-if="data.logData=='' || data.logData==null">
        <p>暂无日志</p>
      </div>
      <div id="viewLog" v-else>
        <div style="white-space: pre-wrap;" v-for="(log, i) in data.logData" :key="i">{{ log.content }}</div>
      </div>
    </div>
  </div>

</template>

<script>
import {computed, onMounted, reactive, onUnmounted, watch} from 'vue';
import {useRoute, useRouter} from "vue-router";
import {get} from "../../plugin/utils/request";
import {ArrowLeftOutlined, DownloadOutlined} from "@ant-design/icons-vue";
import { saveAs } from 'file-saver';
export default {
  name: "PodLogs",
  setup() {
    const route = useRoute()
    const router = useRouter()
    const data = reactive({
      containerData: [],
      initContainerData: [],
      podNameData: [],
      container: undefined,
      pod: undefined,
      logData: [],
      autoRefresh: false,
      timer: null
    })
    const currentNamespace = computed(() => route.query.namespace || "")
    const routePodName = computed(() => route.query.name || "")
    const selectedPodName = computed(() => data.pod || "")
    const legacyDeploymentReturn = computed(() => route.query.returnTo === "podDetail" && !route.query.sourceKind)
    const returnButtonLabel = computed(() => {
      if (route.query.returnTo === "deploymentDetail" || legacyDeploymentReturn.value) {
        return "返回应用详情"
      }
      if (route.query.returnTo === "podDetail") {
        return "返回 Pod 详情"
      }
      if (route.query.returnTo === "podList") {
        return "返回 Pod 列表"
      }
      return "返回上一页"
    })
    const canReturnTarget = computed(() => {
      return !!(route.query.returnTo || selectedPodName.value || routePodName.value)
    })

    const GetLogSource = (params) => {
      const p = "/api/v1/k8s/log/source/" + params.namespace + "/" + params.name + "/" + params.type + "?clusterId=" + params.clusterId
      get(p, "").then(res => {
        if (res.errCode === 0) {
          data.containerData = res.data.containerNames
          data.container = res.data.containerNames[0]
          data.initContainerData = res.data.initContainerNames
          data.podNameData = res.data.podNames
          data.pod = res.data.podNames[0]
          GetLog()
        }
      })
    }
    const handleChange = value => {
      const url = "/api/v1/k8s/log/" + route.query.namespace + "/" + data.pod + "/" + value + "?clusterId=" + route.query.clusterId
      get(url, "").then(res => {
        if (res.errCode === 0) {
          data.logData = res.data.logs
          const div1 = document.getElementById('filelog-container')
          div1.scrollTop = div1.scrollHeight
        }
      })
    };
    const handleLogChange = value => {
      data.pod = value
      GetLog()
    }
    const GetLog = () => {
      const url = "/api/v1/k8s/log/" + route.query.namespace + "/" + data.pod + "?clusterId=" + route.query.clusterId
      get(url, "").then(res => {
        if (res.errCode === 0) {
          data.logData = res.data.logs
          const div1 = document.getElementById('filelog-container')
          div1.scrollTop = div1.scrollHeight
        }
      })
    }
    const refreshLog = (value) => {
      data.autoRefresh = value
    }
    watch(()=>data.autoRefresh,()=>{
      if (data.autoRefresh) {
        createSetInterval()
      }else {
        stopSetInterval()
      }
    })
    const createSetInterval = () => {
      stopSetInterval()
      data.timer = setInterval(() => {
        const url = "/api/v1/k8s/log/" + route.query.namespace + "/" +
            data.pod + "/" + data.container + "?clusterId=" +
            route.query.clusterId +
            "&logFilePosition=end&offsetFrom=2000000000&offsetTo=2000000100&previous=false&referenceLineNum=0&referenceTimestamp=newest"
        get(url, "").then(res => {
          if (res.errCode === 0) {
            data.logData = res.data.logs
            const div1 = document.getElementById('filelog-container')
            div1.scrollTop = div1.scrollHeight
          }
        })
      }, 1000)
    }
    // 关闭轮询
    const stopSetInterval = () => {
      if (data.timer) {
        clearInterval(data.timer)
        data.timer = null
      }
    }
    const downLoadLogFile = () => {
      const url = "/api/v1/k8s/log/file/" + route.query.namespace + "/" +
          data.pod + "/" + data.container + "?clusterId=" + route.query.clusterId + "&previous=false"
      get(url, "").then(res => {
          let log = new Blob([res], {type: 'text/plain;charset=utf-8'})
          saveAs(log, data.container + "-in-" + data.pod + ".log")
      })
    }

    const goPodDetail = () => {
      const podName = data.pod || route.query.name
      if (!podName) {
        goPodList()
        return
      }
      router.push({
        name: 'PodDetail',
        query: {
          clusterId: route.query.clusterId,
          namespace: route.query.namespace,
          name: podName,
          podDetailTab: "1"
        }
      })
    }

    const inferDeploymentNameFromPod = (podName) => {
      if (!podName || typeof podName !== "string") {
        return ""
      }
      const parts = podName.split("-")
      if (parts.length <= 2) {
        return podName
      }
      return parts.slice(0, -2).join("-")
    }

    const goDeploymentDetail = () => {
      const sourceName = route.query.sourceName || inferDeploymentNameFromPod(data.pod || route.query.name)
      if (!sourceName) {
        goPodList()
        return
      }
      router.push({
        name: 'DeploymentDetail',
        query: {
          clusterId: route.query.clusterId,
          namespace: route.query.sourceNamespace || route.query.namespace,
          name: sourceName,
          scrollTo: route.query.scrollTo || "pods"
        }
      })
    }

    const goSourceDetail = () => {
      if (route.query.returnTo === "deploymentDetail" || legacyDeploymentReturn.value) {
        goDeploymentDetail()
        return
      }
      if (route.query.returnTo === "podDetail") {
        goPodDetail()
        return
      }
      goPodDetail()
    }

    const goPodList = () => {
      if (route.query.namespace) {
        localStorage.setItem("namespace", route.query.namespace)
      }
      localStorage.setItem("workload", "6")
      router.push({
        name: 'WorkLoad',
        query: {
          clusterId: route.query.clusterId,
          namespace: route.query.namespace,
          workload: "pod"
        }
      })
    }

    const goBack = () => {
      if (route.query.returnTo === "deploymentDetail" || route.query.returnTo === "podDetail" || legacyDeploymentReturn.value) {
        goSourceDetail()
        return
      }
      if (route.query.returnTo === "podList") {
        goPodList()
        return
      }
      if (window.history.length > 1) {
        router.back()
        return
      }
      goPodDetail()
    }

    onMounted(() => {
      GetLogSource(route.query);
    })
    onUnmounted(() => {
      stopSetInterval()
    })
    return {
      handleChange,
      data,
      currentNamespace,
      routePodName,
      selectedPodName,
      returnButtonLabel,
      canReturnTarget,
      handleLogChange,
      GetLog,
      refreshLog,
      downLoadLogFile,
      goBack,
      goSourceDetail,
      goPodDetail,
      goPodList,
    };
  },
  components: {
    ArrowLeftOutlined,
    DownloadOutlined
  }

}
</script>

<style scoped>
.pod-log-page {
  background-color: #ffffff;
  padding: 16px 20px 20px;
}

.pod-log-header {
  align-items: center;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
  padding-bottom: 14px;
}

.pod-log-back {
  font-size: 18px;
}

.pod-log-title {
  color: #222;
  font-size: 20px;
  font-weight: 600;
  line-height: 28px;
}

.pod-log-meta {
  color: #8c8c8c;
  font-size: 12px;
  line-height: 20px;
}

.pod-log-meta span + span {
  margin-left: 16px;
}

.pod-log-toolbar {
  align-items: center;
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
}

.pod-log-download {
  cursor: pointer;
  font-size: 26px;
}
</style>
