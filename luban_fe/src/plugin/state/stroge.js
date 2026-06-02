// export default class LocalStorage {
//
//     static GetCluster(key) {
//         const value = JSON.parse(localStorage.getItem(key))
//         if (value === null || value === undefined || value === '') {
//             return false
//         }
//         return value
//     }
//
// }
export
const GetStorage = () => {
    const defaultCluster = {
        clusterId: "1",
        clusterName: "",
    }
    try {
        const cs = JSON.parse(localStorage.getItem("cluster"))
        if (cs !== null && cs !== undefined && cs !== "" && cs.clusterId && cs.clusterId !== "undefined" && cs.clusterId !== "null") {
            return {
                clusterId: cs.clusterId,
                clusterName: cs.clusterName || "",
            }
        }
    } catch (e) {
        localStorage.removeItem("cluster")
    }
    return defaultCluster
}
