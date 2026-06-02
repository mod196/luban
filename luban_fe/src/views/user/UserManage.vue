<template>
  <div class="user-manage-page">
    <div class="page-head">
      <div>
        <span class="page-title">用户管理</span>
        <span class="page-subtitle">User Management</span>
      </div>
      <a-space>
        <a-button @click="loadUsers">刷新</a-button>
        <a-button type="primary" @click="openCreateModal">新建测试用户</a-button>
      </a-space>
    </div>

    <a-alert
        class="guardrail"
        type="warning"
        show-icon
        message="测试用户创建后，请在资源绑定页按用户授权服务树节点；生产账号启停、删除建议接入审计后再开放。"
    />

    <div class="toolbar">
      <a-input-search
          v-model:value="state.keyword"
          placeholder="搜索用户名、昵称或邮箱"
          allow-clear
          enter-button
          @search="loadUsers"
      />
      <span class="toolbar-meta">共 {{ state.users.length }} 个可授权用户</span>
    </div>

    <a-table
        :columns="columns"
        :data-source="state.users"
        :pagination="{ pageSize: 10 }"
        :loading="state.loading"
        row-key="id"
        :locale="{emptyText: '暂无用户'}"
    >
      <template #identity="{ record }">
        <div class="identity">
          <div class="identity-name">{{ record.name || '-' }}</div>
          <div class="identity-id">ID: {{ record.id }}</div>
        </div>
      </template>
      <template #email="{ text }">
        <span>{{ text || '-' }}</span>
      </template>
      <template #source>
        <a-tag color="cyan">授权主体</a-tag>
      </template>
      <template #action="{ record }">
        <a-space>
          <router-link :to="{ path: '/k8s/resource-binding', query: { principalType: 'user', principalId: record.id } }">
            配置授权
          </router-link>
        </a-space>
      </template>
    </a-table>

    <a-modal
        v-model:visible="state.createVisible"
        title="新建测试用户"
        ok-text="创建"
        cancel-text="取消"
        :confirm-loading="state.creating"
        :keyboard="false"
        :maskClosable="false"
        @ok="submitCreate"
        @cancel="resetCreateForm"
    >
      <a-form
          ref="formRef"
          :model="form"
          :rules="rules"
          layout="vertical"
      >
        <a-form-item label="邮箱" name="email">
          <a-input v-model:value="form.email" placeholder="qa-user@example.com" @blur="syncUsernameFromEmail" />
        </a-form-item>
        <a-form-item label="用户名" name="username">
          <a-input v-model:value="form.username" placeholder="qa-user" />
        </a-form-item>
        <a-form-item label="昵称" name="nickName">
          <a-input v-model:value="form.nickName" placeholder="QA 测试用户" />
        </a-form-item>
        <a-form-item label="角色" name="roleId">
          <a-select
              v-model:value="form.roleId"
              show-search
              option-filter-prop="label"
              placeholder="选择角色"
              :loading="state.loadingRoles"
          >
            <a-select-option v-for="role in state.roles" :key="role.id" :value="role.id" :label="role.name">
              {{ role.name }}
              <span class="option-meta">{{ role.description }}</span>
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="初始密码" name="password">
          <a-input-password v-model:value="form.password" placeholder="至少 6 位" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script>
import {defineComponent, onMounted, reactive, ref} from 'vue';
import {message} from 'ant-design-vue';
import {createUser, fetchRolePrincipals, fetchUserPrincipals} from '@/api/user';

const columns = [
  {
    title: '用户',
    slots: {customRender: 'identity'},
  },
  {
    title: '邮箱',
    dataIndex: 'email',
    slots: {customRender: 'email'},
  },
  {
    title: '来源',
    slots: {customRender: 'source'},
  },
  {
    title: '操作',
    slots: {customRender: 'action'},
  },
];

const defaultForm = () => ({
  email: '',
  username: '',
  nickName: '',
  roleId: undefined,
  password: `Qa${Date.now().toString().slice(-6)}!`,
});

export default defineComponent({
  name: 'UserManage',
  setup() {
    const formRef = ref();
    const form = reactive(defaultForm());
    const state = reactive({
      users: [],
      roles: [],
      keyword: '',
      loading: false,
      loadingRoles: false,
      createVisible: false,
      creating: false,
    });

    const rules = {
      email: [
        {required: true, message: '请输入邮箱', trigger: 'blur'},
        {type: 'email', message: '邮箱格式不正确', trigger: 'blur'},
      ],
      username: [
        {required: true, message: '请输入用户名', trigger: 'blur'},
        {min: 3, max: 64, message: '用户名长度应为 3~64', trigger: 'blur'},
      ],
      nickName: [
        {required: true, message: '请输入昵称', trigger: 'blur'},
      ],
      roleId: [
        {required: true, message: '请选择角色', trigger: 'change'},
      ],
      password: [
        {required: true, message: '请输入初始密码', trigger: 'blur'},
        {min: 6, message: '密码至少 6 位', trigger: 'blur'},
      ],
    };

    const normalizeUser = (item) => ({
      id: item.id,
      name: item.name,
      email: item.description,
      principalType: item.principalType,
    });

    const resetCreateForm = () => {
      Object.assign(form, defaultForm());
      formRef.value && formRef.value.clearValidate();
      setDefaultRole();
    };

    const setDefaultRole = () => {
      const normalRole = state.roles.find((role) => !String(role.name || '').includes('超级') && !String(role.name || '').toLowerCase().includes('super'));
      if (normalRole) {
        form.roleId = normalRole.id;
      }
    };

    const loadRoles = async () => {
      state.loadingRoles = true;
      try {
        const res = await fetchRolePrincipals({});
        if (res.errCode === 0) {
          state.roles = res.data || [];
          if (!form.roleId) {
            setDefaultRole();
          }
        } else {
          message.error(res.errMsg || '获取角色失败');
        }
      } finally {
        state.loadingRoles = false;
      }
    };

    const loadUsers = async () => {
      state.loading = true;
      try {
        const res = await fetchUserPrincipals({keyword: state.keyword});
        if (res.errCode === 0) {
          state.users = (res.data || []).map(normalizeUser);
        } else {
          message.error(res.errMsg || '获取用户失败');
        }
      } finally {
        state.loading = false;
      }
    };

    const syncUsernameFromEmail = () => {
      if (!form.email || form.username) {
        return;
      }
      form.username = form.email.split('@')[0].replace(/[^a-zA-Z0-9_.-]/g, '-').slice(0, 64);
      if (!form.nickName) {
        form.nickName = form.username;
      }
    };

    const openCreateModal = () => {
      resetCreateForm();
      state.createVisible = true;
    };

    const submitCreate = () => {
      syncUsernameFromEmail();
      formRef.value
          .validate()
          .then(async () => {
            state.creating = true;
            try {
              const res = await createUser({
                username: form.username,
                email: form.email,
                password: form.password,
                nick_name: form.nickName,
                role_id: Number(form.roleId),
                status: true,
                create_by: 'admin',
              });
              if (res.errCode === 0) {
                message.success('用户创建成功');
                state.keyword = form.email;
                state.createVisible = false;
                resetCreateForm();
                await loadUsers();
              } else {
                message.error(res.errMsg || '用户创建失败');
              }
            } finally {
              state.creating = false;
            }
          })
          .catch(() => {});
    };

    onMounted(async () => {
      await loadRoles();
      await loadUsers();
    });

    return {
      columns,
      form,
      formRef,
      loadUsers,
      openCreateModal,
      resetCreateForm,
      rules,
      state,
      submitCreate,
      syncUsernameFromEmail,
    };
  },
});
</script>

<style scoped>
.user-manage-page {
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

.toolbar {
  align-items: center;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  padding: 18px 24px;
}

.toolbar :deep(.ant-input-search) {
  max-width: 420px;
}

.toolbar-meta {
  color: #8c8c8c;
  white-space: nowrap;
}

.identity {
  line-height: 1.5;
}

.identity-name {
  color: #202124;
  font-weight: 600;
}

.identity-id {
  color: #8c8c8c;
  font-size: 12px;
}

.option-meta {
  color: #8c8c8c;
  margin-left: 8px;
}

.user-manage-page :deep(.ant-table-wrapper) {
  padding: 0 24px 24px;
}
</style>
