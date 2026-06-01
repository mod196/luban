<template>
  <div class="k8s-terminal-page">
    <a-page-header
        :title="route.query.pod || '容器终端'"
        sub-title="Kubernetes Exec"
        @back="goBack"
    >
      <template #tags>
        <a-tag color="blue">{{ route.query.namespace }}</a-tag>
        <a-tag color="green">cluster {{ route.query.clusterId }}</a-tag>
      </template>
      <template #extra>
        <a-select v-model:value="state.shell" size="small" style="width: 100px" @change="reconnect">
          <a-select-option value="bash">bash</a-select-option>
          <a-select-option value="sh">sh</a-select-option>
        </a-select>
        <a-button size="small" @click="reconnect">重连</a-button>
      </template>
    </a-page-header>
    <div class="terminal-wrap">
      <div ref="terminalRef" class="terminal"></div>
    </div>
  </div>
</template>

<script>
import {nextTick, onBeforeUnmount, onMounted, reactive, ref} from "vue";
import {useRoute, useRouter} from "vue-router";
import {Terminal} from "xterm";
import {FitAddon} from "xterm-addon-fit";
import "xterm/css/xterm.css";

export default {
  name: "K8sTerminal",
  setup() {
    const route = useRoute()
    const router = useRouter()
    const terminalRef = ref()
    const state = reactive({
      shell: "sh",
      connected: false,
    })
    let term = null
    let fitAddon = null
    let ws = null

    const goBack = () => {
      router.back()
    }

    const terminalUrl = () => {
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
      const params = new URLSearchParams({
        clusterId: route.query.clusterId || "1",
        namespace: route.query.namespace || "",
        pod: route.query.pod || route.query.name || "",
        container: route.query.container || "",
        shell: state.shell,
        token: localStorage.getItem("token") || "",
      })
      return `${protocol}//${window.location.host}/api/v1/ws/k8s/terminal?${params.toString()}`
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
      terminalRef,
      goBack,
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
