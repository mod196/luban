<template>
  <div class="settings-page">
    <div class="page-head">
      <div>
        <span class="page-title">系统设置</span>
        <span class="page-subtitle">System Settings</span>
      </div>
      <a-button @click="refreshRuntime">刷新</a-button>
    </div>

    <a-alert
        class="guardrail"
        type="warning"
        show-icon
        message="系统级配置写入需要后端配置接口和审计记录；当前页面只展示可确认的运行配置和管理入口。"
    />

    <div class="settings-grid">
      <section class="settings-section">
        <div class="section-title">平台信息</div>
        <a-descriptions bordered size="small" :column="1">
          <a-descriptions-item label="平台名称">{{ runtime.title }}</a-descriptions-item>
          <a-descriptions-item label="前端模式">{{ runtime.nodeEnv }}</a-descriptions-item>
          <a-descriptions-item label="后端地址">{{ runtime.apiBaseUrl || '-' }}</a-descriptions-item>
          <a-descriptions-item label="当前用户">{{ runtime.email || '-' }}</a-descriptions-item>
        </a-descriptions>
      </section>

      <section class="settings-section">
        <div class="section-title">访问控制</div>
        <a-descriptions bordered size="small" :column="1">
          <a-descriptions-item label="登录状态">
            <a-tag :color="runtime.online ? 'green' : 'default'">{{ runtime.online ? '已登录' : '未登录' }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="接口鉴权">JWT Token</a-descriptions-item>
          <a-descriptions-item label="菜单权限">前端路由 + 后端 RBAC</a-descriptions-item>
          <a-descriptions-item label="资源权限">服务树授权策略</a-descriptions-item>
        </a-descriptions>
      </section>

      <section class="settings-section wide">
        <div class="section-title">管理入口</div>
        <div class="quick-actions">
          <router-link to="/user/change/password">
            <a-button>修改密码</a-button>
          </router-link>
          <router-link to="/user/manage">
            <a-button>用户管理</a-button>
          </router-link>
          <router-link to="/k8s/resource-binding">
            <a-button>资源绑定</a-button>
          </router-link>
          <router-link to="/k8s/cluster">
            <a-button>集群管理</a-button>
          </router-link>
        </div>
      </section>
    </div>
  </div>
</template>

<script>
import {defineComponent, onMounted, reactive} from 'vue';
import env from '@/store/env';

export default defineComponent({
  name: 'SystemSettings',
  setup() {
    const runtime = reactive({
      title: env.Title,
      nodeEnv: process.env.NODE_ENV || '',
      apiBaseUrl: process.env.VUE_APP_BASE_URL || window.location.origin,
      email: '',
      online: false,
    });

    const refreshRuntime = () => {
      runtime.title = env.Title;
      runtime.nodeEnv = process.env.NODE_ENV || '';
      runtime.apiBaseUrl = process.env.VUE_APP_BASE_URL || window.location.origin;
      runtime.email = localStorage.getItem('email') || '';
      runtime.online = Boolean(localStorage.getItem('onLine') && localStorage.getItem('token'));
    };

    onMounted(refreshRuntime);

    return {
      refreshRuntime,
      runtime,
    };
  },
});
</script>

<style scoped>
.settings-page {
  background: #fff;
  min-height: calc(100vh - 170px);
}

.page-head {
  align-items: center;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  justify-content: space-between;
  padding: 24px 24px 18px;
}

.page-title {
  color: #202124;
  font-size: 28px;
  font-weight: 700;
  line-height: 1;
}

.page-subtitle {
  color: #8c8c8c;
  font-size: 16px;
  margin-left: 12px;
}

.guardrail {
  margin: 18px 24px 0;
}

.settings-grid {
  display: grid;
  gap: 24px;
  grid-template-columns: repeat(2, minmax(320px, 1fr));
  padding: 24px;
}

.settings-section {
  border: 1px solid #f0f0f0;
  padding: 20px;
}

.settings-section.wide {
  grid-column: 1 / -1;
}

.section-title {
  color: #262626;
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 16px;
}

.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

@media (max-width: 900px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}
</style>
