<template>
  <div class="k8s-terminal-page">
    <a-page-header
        :title="currentPodName || '容器终端'"
        :backIcon="false"
    >
      <template #tags>
        <a-tag color="blue">{{ namespace }}</a-tag>
        <a-tag color="green">{{ clusterName }}</a-tag>
      </template>
      <template #extra>
        <a-space>
          <a-select
              v-if="state.podOptions.length > 1"
              v-model:value="state.pod"
              size="small"
              show-search
              style="width: 360px"
              option-filter-prop="children"
              :loading="state.loadingPods"
              @change="switchPod"
          >
            <a-select-option v-for="pod in state.podOptions" :key="pod.name" :value="pod.name">
              {{ pod.name }}
            </a-select-option>
          </a-select>
          <a-select v-model:value="state.shell" size="small" style="width: 100px" @change="reconnect">
            <a-select-option value="bash">bash</a-select-option>
            <a-select-option value="sh">sh</a-select-option>
          </a-select>
          <a-button size="small" @click="reconnect">重连</a-button>
          <a-button size="small" @click="goApplicationDetail">返回应用详情</a-button>
        </a-space>
      </template>
    </a-page-header>
    <div class="terminal-wrap">
      <div ref="terminalRef" class="terminal"></div>
    </div>
  </div>
</template>

<script>
import {computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch} from "vue";
import {useRoute, useRouter} from "vue-router";
import {Terminal} from "xterm";
import {FitAddon} from "xterm-addon-fit";
import "xterm/css/xterm.css";
import {DaemonSetDetail, DeploymentDetail, fetchK8SCluster, JobDetail, StatefulSetDetail} from "../../api/k8s";
import {GetStorage} from "../../plugin/state/stroge";

export default {
  name: "K8sTerminal",
  setup() {
    const route = useRoute()
    const router = useRouter()
    const terminalRef = ref()
    const state = reactive({
      shell: "sh",
      connected: false,
      pod: route.query.pod || route.query.name || "",
      clusterName: "",
      podOptions: [],
      loadingPods: false,
    })
    let term = null
    let fitAddon = null
    let ws = null

    const clusterId = computed(() => String(route.query.clusterId || "1"))
    const namespace = computed(() => String(route.query.namespace || ""))
    const currentPodName = computed(() => String(state.pod || route.query.pod || route.query.name || ""))
    const clusterName = computed(() => state.clusterName || route.query.clusterName || `cluster ${clusterId.value}`)

    const inferDeploymentNameFromPod = (podName) => {
      const name = String(podName || "")
      if (!/-[a-f0-9]{8,10}-[a-z0-9]{5}$/.test(name)) {
        return ""
      }
      return name.split("-").slice(0, -2).join("-")
    }

    const normalizePodOption = (pod) => {
      const meta = pod && (pod.objectMeta || pod.metadata)
      const name = (meta && meta.name) || (pod && pod.name)
      return name ? {name} : null
    }

    const fetchSourcePodList = (sourceKind, sourceName, sourceNamespace) => {
      const params = {
        clusterId: clusterId.value,
        namespace: sourceNamespace,
        name: sourceName,
      }
      if (sourceKind === "deployment") {
        return DeploymentDetail(params)
      }
      if (sourceKind === "statefulset") {
        return StatefulSetDetail(params)
      }
      if (sourceKind === "daemonset") {
        return DaemonSetDetail(clusterId.value, {namespace: sourceNamespace, name: sourceName})
      }
      if (sourceKind === "job") {
        return JobDetail(clusterId.value, {namespace: sourceNamespace, name: sourceName})
      }
      return Promise.resolve(null)
    }

    const sourceDetailRouteName = (sourceKind) => {
      if (sourceKind === "statefulset") {
        return "StatefulSetDetail"
      }
      if (sourceKind === "daemonset") {
        return "DaemonSetDetail"
      }
      if (sourceKind === "job") {
        return "JobDetail"
      }
      return "DeploymentDetail"
    }

    const loadClusterName = async () => {
      const storedCluster = GetStorage()
      if (storedCluster && String(storedCluster.clusterId) === clusterId.value && storedCluster.clusterName) {
        state.clusterName = storedCluster.clusterName
        return
      }
      if (route.query.clusterName) {
        state.clusterName = route.query.clusterName
        return
      }
      state.clusterName = `cluster ${clusterId.value}`
      try {
        const res = await fetchK8SCluster({page: 1, size: 100, itemsPerPage: 100})
        const payload = res && res.data ? res.data : {}
        const clusters = Array.isArray(payload.data) ? payload.data : (Array.isArray(payload.items) ? payload.items : [])
        const cluster = clusters.find(item => String(item.id || item.clusterId) === clusterId.value)
        if (cluster) {
          state.clusterName = cluster.clusterName || cluster.name || state.clusterName
        }
      } catch (e) {
        state.clusterName = state.clusterName || `cluster ${clusterId.value}`
      }
    }

    const loadPodOptions = async () => {
      const current = currentPodName.value
      const inferredDeploymentName = inferDeploymentNameFromPod(current)
      const sourceName = route.query.sourceName || inferredDeploymentName
      const sourceKind = route.query.sourceKind || (sourceName ? "deployment" : "")
      if (!sourceKind || !sourceName || !namespace.value) {
        state.podOptions = current ? [{name: current}] : []
        return
      }
      state.loadingPods = true
      try {
        const res = await fetchSourcePodList(sourceKind, sourceName, route.query.sourceNamespace || namespace.value)
        const pods = res && res.errCode === 0 && res.data && res.data.podList
            ? (res.data.podList.pods || []).map(normalizePodOption).filter(Boolean)
            : []
        if (current && !pods.some(pod => pod.name === current)) {
          pods.unshift({name: current})
        }
        state.podOptions = pods.length > 0 ? pods : (current ? [{name: current}] : [])
      } catch (e) {
        state.podOptions = current ? [{name: current}] : []
      } finally {
        state.loadingPods = false
      }
    }

    const goApplicationDetail = () => {
      closeSocket()
      const podName = currentPodName.value
      const inferredDeploymentName = inferDeploymentNameFromPod(podName)
      const sourceName = route.query.sourceName || inferredDeploymentName
      const sourceKind = route.query.sourceKind || (sourceName ? "deployment" : "")
      if (!sourceName) {
        router.push({
          name: "PodDetail",
          query: {
            clusterId: clusterId.value,
            namespace: namespace.value,
            name: podName,
          },
        })
        return
      }
      router.push({
        name: sourceDetailRouteName(sourceKind),
        query: {
          clusterId: clusterId.value,
          namespace: route.query.sourceNamespace || namespace.value,
          name: sourceName,
          scrollTo: "pods",
        },
      })
    }

    const switchPod = (podName) => {
      if (!podName) {
        return
      }
      state.pod = podName
      const query = {...route.query, pod: podName}
      delete query.name
      router.replace({name: "K8sTerminal", query})
    }

    const terminalBaseUrl = () => {
      const configuredBaseUrl = (process.env.VUE_APP_BASE_URL || "").trim()
      const fallbackOrigin = window.location.origin
      const baseUrl = configuredBaseUrl || fallbackOrigin
      try {
        const url = new URL(baseUrl, fallbackOrigin)
        url.protocol = url.protocol === "https:" ? "wss:" : "ws:"
        return url.origin.replace(/^http/, "ws").replace(/\/$/, "")
      } catch (e) {
        const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
        return `${protocol}//${window.location.host}`
      }
    }

    const terminalUrl = () => {
      const params = new URLSearchParams({
        clusterId: clusterId.value,
        namespace: namespace.value,
        pod: currentPodName.value,
        container: route.query.container || "",
        shell: state.shell,
        token: localStorage.getItem("token") || "",
      })
      return `${terminalBaseUrl()}/api/v1/ws/k8s/terminal?${params.toString()}`
    }

    const sendResize = () => {
      if (!term || !ws || ws.readyState !== WebSocket.OPEN) {
        return
      }
      ws.send(JSON.stringify({
        op: "resize",
        rows: term.rows,
        cols: term.cols,
      }))
    }

    const fitTerminal = () => {
      if (!fitAddon) {
        return
      }
      fitAddon.fit()
      sendResize()
    }

    const connect = () => {
      if (!term) {
        return
      }
      ws = new WebSocket(terminalUrl())
      ws.onopen = () => {
        state.connected = true
        term.writeln("")
        term.writeln("Connected.")
        sendResize()
      }
      ws.onmessage = (event) => {
        term.write(event.data)
      }
      ws.onerror = () => {
        term.writeln("\r\nWebSocket connection error.")
      }
      ws.onclose = () => {
        state.connected = false
        term.writeln("\r\nConnection closed.")
      }
    }

    const closeSocket = () => {
      if (ws) {
        ws.close()
        ws = null
      }
    }

    const reconnect = () => {
      closeSocket()
      if (term) {
        term.clear()
      }
      connect()
    }

    watch(() => route.query.pod, async (podName, oldPodName) => {
      state.pod = podName || route.query.name || ""
      await loadPodOptions()
      if (oldPodName !== undefined && podName !== oldPodName) {
        reconnect()
      }
    })

    watch([clusterId, namespace], async () => {
      await loadClusterName()
      await loadPodOptions()
      reconnect()
    })

    onMounted(async () => {
      term = new Terminal({
        cursorBlink: true,
        convertEol: true,
        fontSize: 13,
        fontFamily: "Menlo, Monaco, Consolas, monospace",
        theme: {
          background: "#111827",
          foreground: "#E5E7EB",
          cursor: "#60A5FA",
        },
      })
      fitAddon = new FitAddon()
      term.loadAddon(fitAddon)
      term.open(terminalRef.value)
      term.onData((value) => {
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({op: "stdin", data: value}))
        }
      })
      term.onResize(sendResize)
      await nextTick()
      fitTerminal()
      await Promise.all([
        loadClusterName(),
        loadPodOptions(),
      ])
      connect()
      window.addEventListener("resize", fitTerminal)
    })

    onBeforeUnmount(() => {
      window.removeEventListener("resize", fitTerminal)
      closeSocket()
      if (term) {
        term.dispose()
      }
    })

    return {
      route,
      state,
      namespace,
      clusterName,
      currentPodName,
      terminalRef,
      goApplicationDetail,
      switchPod,
      reconnect,
    }
  }
}
</script>

<style scoped>
.k8s-terminal-page {
  min-height: calc(100vh - 64px);
  background: #f0f2f5;
}

.terminal-wrap {
  padding: 0 24px 24px;
}

.terminal {
  width: 100%;
  height: calc(100vh - 210px);
  min-height: 520px;
  padding: 12px;
  background: #111827;
  border-radius: 4px;
  box-sizing: border-box;
}
</style>
