<template>
  <div class="password-page">
    <div class="page-head">
      <div>
        <span class="page-title">修改密码</span>
        <span class="page-subtitle">Change Password</span>
      </div>
    </div>

    <a-alert
        class="guardrail"
        type="warning"
        show-icon
        message="修改成功后会清除当前登录态，请使用新密码重新登录。"
    />

    <div class="password-layout">
      <section class="form-panel">
        <a-form
            ref="formRef"
            :model="form"
            :rules="rules"
            layout="vertical"
        >
          <a-form-item label="原密码" name="oldPassword">
            <a-input-password v-model:value="form.oldPassword" placeholder="请输入当前登录密码" />
          </a-form-item>
          <a-form-item label="新密码" name="newPassword">
            <a-input-password v-model:value="form.newPassword" placeholder="至少 6 位" />
          </a-form-item>
          <a-form-item label="确认新密码" name="confirmPassword">
            <a-input-password v-model:value="form.confirmPassword" placeholder="再次输入新密码" @pressEnter="submit" />
          </a-form-item>
          <a-space>
            <a-button type="primary" :loading="state.submitting" @click="submit">保存修改</a-button>
            <a-button @click="resetForm">重置</a-button>
          </a-space>
        </a-form>
      </section>

      <section class="note-panel">
        <div class="note-title">安全建议</div>
        <ul>
          <li>不要与测试账号、生产账号复用同一个密码。</li>
          <li>不要把密码写入工单、截图、日志或聊天记录。</li>
          <li>如果账号接入 LDAP，建议在统一身份源完成密码修改。</li>
        </ul>
      </section>
    </div>
  </div>
</template>

<script>
import {defineComponent, reactive, ref} from 'vue';
import {message} from 'ant-design-vue';
import router from '@/router';
import {changePassword} from '@/api/user';

const emptyForm = () => ({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
});

export default defineComponent({
  name: 'ChangePassword',
  setup() {
    const formRef = ref();
    const form = reactive(emptyForm());
    const state = reactive({
      submitting: false,
    });

    const validateConfirmPassword = async (_rule, value) => {
      if (!value) {
        return Promise.reject('请再次输入新密码');
      }
      if (value !== form.newPassword) {
        return Promise.reject('两次输入的新密码不一致');
      }
      return Promise.resolve();
    };

    const rules = {
      oldPassword: [
        {required: true, message: '请输入原密码', trigger: 'blur'},
      ],
      newPassword: [
        {required: true, message: '请输入新密码', trigger: 'blur'},
        {min: 6, message: '新密码至少 6 位', trigger: 'blur'},
      ],
      confirmPassword: [
        {validator: validateConfirmPassword, trigger: 'blur'},
      ],
    };

    const resetForm = () => {
      Object.assign(form, emptyForm());
      formRef.value && formRef.value.clearValidate();
    };

    const logoutToLogin = () => {
      localStorage.removeItem('token');
      localStorage.removeItem('onLine');
      router.push('/user/login');
    };

    const submit = () => {
      formRef.value
          .validate()
          .then(async () => {
            if (form.oldPassword === form.newPassword) {
              message.warning('新密码不能与原密码相同');
              return;
            }
            state.submitting = true;
            try {
              const res = await changePassword({
                oldPassword: form.oldPassword,
                newPassword: form.newPassword,
                confirmPassword: form.confirmPassword,
              });
              if (res.errCode === 0) {
                message.success(res.msg || '密码修改成功，请重新登录');
                resetForm();
                setTimeout(logoutToLogin, 700);
              } else {
                message.error(res.errMsg || '密码修改失败');
              }
            } finally {
              state.submitting = false;
            }
          })
          .catch(() => {});
    };

    return {
      form,
      formRef,
      resetForm,
      rules,
      state,
      submit,
    };
  },
});
</script>

<style scoped>
.password-page {
  background: #fff;
  min-height: calc(100vh - 170px);
}

.page-head {
  border-bottom: 1px solid #f0f0f0;
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

.password-layout {
  display: grid;
  gap: 24px;
  grid-template-columns: minmax(360px, 520px) minmax(260px, 1fr);
  padding: 24px;
}

.form-panel {
  border: 1px solid #f0f0f0;
  padding: 24px;
}

.note-panel {
  background: #fafafa;
  border: 1px solid #f0f0f0;
  color: #595959;
  padding: 24px;
}

.note-title {
  color: #262626;
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 12px;
}

.note-panel ul {
  margin: 0;
  padding-left: 18px;
}

.note-panel li + li {
  margin-top: 8px;
}

@media (max-width: 900px) {
  .password-layout {
    grid-template-columns: 1fr;
  }
}
</style>
