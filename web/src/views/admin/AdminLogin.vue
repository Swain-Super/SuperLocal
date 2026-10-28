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
  <main class="min-h-screen flex items-center justify-center bg-page px-4 font-sans text-body text-text">
    <form
      class="w-full max-w-sm border border-border rounded-panel bg-panel px-6 py-5"
      @submit.prevent="onSubmit"
    >
      <h1 class="text-body font-medium">SuperLocal</h1>

      <div class="mt-4">
        <label class="block text-aux text-secondary" for="admin-username">账号</label>
        <input
          id="admin-username"
          v-model="username"
          name="username"
          type="text"
          autocomplete="username"
          class="mt-1 box-border h-8 w-full border border-border rounded-panel bg-panel px-2 text-body text-text outline-none focus:border-primary"
        />
      </div>

      <div class="mt-3">
        <label class="block text-aux text-secondary" for="admin-password">密码</label>
        <input
          id="admin-password"
          v-model="password"
          name="password"
          type="password"
          autocomplete="current-password"
          class="mt-1 box-border h-8 w-full border border-border rounded-panel bg-panel px-2 text-body text-text outline-none focus:border-primary"
        />
      </div>

      <button
        type="submit"
        class="mt-4 h-8 w-full rounded-panel bg-primary text-body text-panel active:bg-pressed disabled:opacity-60"
        :disabled="submitting"
      >
        登录
      </button>
      <p v-if="formError" class="mt-2 text-aux text-text">{{ formError }}</p>
    </form>
  </main>
</template>
