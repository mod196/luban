import router from "@/router";
import {GetStorage} from "@/plugin/state/stroge";

const getK8sPodTerminalQuery = (pod) => {
    const meta = pod && (pod.objectMeta || pod.metadata)
    if (!meta || !meta.name || !meta.namespace) {
        return false
    }
    const cs = GetStorage()
    const currentQuery = router.currentRoute && router.currentRoute.value ? router.currentRoute.value.query : {}
    const clusterId = (cs && cs.clusterId) || currentQuery.clusterId || pod.clusterId || "1"
    return {
        clusterId: clusterId,
        namespace: meta.namespace,
        pod: meta.name,
    }
}

export const k8sPodTerminalHref = (pod) => {
    const query = getK8sPodTerminalQuery(pod)
    if (!query) {
        return "javascript:void(0)"
    }
    return router.resolve({name: 'K8sTerminal', query}).href
}

export const openK8sPodTerminal = (pod) => {
    const query = getK8sPodTerminalQuery(pod)
    if (!query) {
        return
    }
    router.push({name: 'K8sTerminal', query})
}
