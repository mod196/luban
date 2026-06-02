import {reactive} from 'vue';

// 首页URL地址, 钉钉登陆
const homeURL = 'http://devops.luban.com'

const DEFAULT_TITLE = '运维平台'

const env = reactive({
    homeURL,
    Title: DEFAULT_TITLE
})

export const normalizeTitle = (title) => {
    const value = typeof title === 'string' ? title.trim() : ''
    return value || DEFAULT_TITLE
}

export const setRuntimeConfig = (config = {}) => {
    env.Title = normalizeTitle(config.platformTitle)
}

export default env
