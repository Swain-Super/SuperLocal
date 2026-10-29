<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { adminLogin, apiErrorMessage } from '../../api'

const router = useRouter()
const username = ref('')
const password = ref('')
const formError = ref('')
const submitting = ref(false)

async function onSubmit() {
  formError.value = ''
  submitting.value = true
  try {
    await adminLogin(username.value.trim(), password.value)
    await router.push('/admin')
  } catch (err) {
    formError.value = apiErrorMessage(err)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="auth">
    <form class="card" @submit.prevent="onSubmit">
      <div class="brand">
        <span class="mark" aria-hidden="true">S</span>
        <div>
          <h1>SuperLocal</h1>
          <p>管理后台</p>
        </div>
      </div>

      <label for="admin-username">账号</label>
      <input
        id="admin-username"
        v-model="username"
        name="username"
        type="text"
        autocomplete="username"
      />

      <label for="admin-password">密码</label>
      <input
        id="admin-password"
        v-model="password"
        name="password"
        type="password"
        autocomplete="current-password"
      />

      <button type="submit" :disabled="submitting">登录</button>
      <p v-if="formError" class="error">{{ formError }}</p>
    </form>
  </main>
</template>

<style scoped>
.auth {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: #f3f5f7;
  color: #1f2329;
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
  font-size: 14px;
}
.card {
  width: 100%;
  max-width: 400px;
  padding: 32px 32px 28px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(31, 35, 41, 0.08);
}
.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 28px;
}
.mark {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  background: #0b6bcb;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 600;
  flex: none;
}
h1 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  line-height: 1.3;
}
.brand p {
  margin: 4px 0 0;
  color: #646a73;
  font-size: 12px;
}
label {
  display: block;
  margin: 14px 0 6px;
  color: #646a73;
  font-size: 12px;
}
input {
  box-sizing: border-box;
  width: 100%;
  height: 40px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: #f3f5f7;
  padding: 0 12px;
  color: #1f2329;
  font: inherit;
  outline: none;
}
input:focus {
  border-color: #0b6bcb;
  background: #fff;
}
button {
  width: 100%;
  height: 40px;
  margin-top: 24px;
  border: 0;
  border-radius: 6px;
  background: #0b6bcb;
  color: #fff;
  font: inherit;
  cursor: pointer;
}
button:hover:not(:disabled) {
  background: #0a61b8;
}
button:active:not(:disabled) {
  background: #0958a8;
}
button:disabled {
  opacity: 0.6;
  cursor: default;
}
.error {
  margin: 8px 0 0;
  font-size: 12px;
}
</style>
