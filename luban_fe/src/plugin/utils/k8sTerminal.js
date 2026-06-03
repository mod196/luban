import router from "@/router";
import {GetStorage} from "@/plugin/state/stroge";

const getK8sPodTerminalQuery = (pod) => {
    const meta = pod && (pod.objectMeta || pod.metadata)
    if (!meta || !meta.name || !meta.namespace) {
        return false
    }
    const cs = GetStorage()
    const currentRoute = router.currentRoute && router.currentRoute.value ? router.currentRoute.value : {}
    const currentQuery = currentRoute.query || {}
    const clusterId = (cs && cs.clusterId) || currentQuery.clusterId || pod.clusterId || "1"
    const query = {
        clusterId: clusterId,
        namespace: meta.namespace,
        pod: meta.name,
    }
    if (cs && cs.clusterName) {
        query.clusterName = cs.clusterName
    }
    const sourceMap = {
        DeploymentDetail: "deployment",
        StatefulSetDetail: "statefulset",
        DaemonSetDetail: "daemonset",
        JobDetail: "job",
    }
    const sourceKind = sourceMap[currentRoute.name]
    if (sourceKind) {
        query.sourceKind = sourceKind
        query.sourceName = currentQuery.name
        query.sourceNamespace = currentQuery.namespace || meta.namespace
        query.scrollTo = "pods"
        if (sourceKind === "deployment") {
            query.returnTo = "deploymentDetail"
        }
    } else if (currentRoute.name === "PodDetail") {
        query.returnTo = currentQuery.returnTo || "podDetail"
        query.sourceName = currentQuery.sourceName
        query.sourceKind = currentQuery.sourceKind
        query.sourceNamespace = currentQuery.sourceNamespace || currentQuery.namespace || meta.namespace
        query.scrollTo = currentQuery.scrollTo
    } else if (currentRoute.name === "WorkLoad") {
        query.returnTo = "podList"
    }
    return query
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
