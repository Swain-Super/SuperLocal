<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiErrorMessage, checkedKey, keyRuleMessage, login } from '../api'

const router = useRouter()
const key = ref('')
const nickname = ref('')
const keyError = ref('')
const formError = ref('')
const submitting = ref(false)

async function onSubmit() {
  keyError.value = ''
  formError.value = ''
  const normalized = checkedKey(key.value)
  if (normalized == null) {
    keyError.value = keyRuleMessage
    return
  }
  submitting.value = true
  try {
    await login(normalized, nickname.value.trim())
    await router.push('/')
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
      <p class="mt-1 text-aux text-secondary">用 Key 进入，Key 即账号</p>

      <div class="mt-4">
        <label class="block text-aux text-secondary" for="chat-key">Key</label>
        <input
          id="chat-key"
          v-model="key"
          name="key"
          type="text"
          autocomplete="username"
          class="mt-1 box-border h-8 w-full border border-border rounded-panel bg-panel px-2 text-body text-text outline-none focus:border-primary"
        />
        <p v-if="keyError" class="mt-1 text-aux text-text">{{ keyError }}</p>
      </div>

      <div class="mt-3">
        <label class="block text-aux text-secondary" for="chat-nickname">昵称</label>
        <input
          id="chat-nickname"
          v-model="nickname"
          name="nickname"
          type="text"
          autocomplete="nickname"
          class="mt-1 box-border h-8 w-full border border-border rounded-panel bg-panel px-2 text-body text-text outline-none focus:border-primary"
        />
      </div>

      <button
        type="submit"
        class="mt-4 h-8 w-full rounded-panel bg-primary text-body text-panel active:bg-pressed disabled:opacity-60"
        :disabled="submitting"
      >
        进入
      </button>
      <p v-if="formError" class="mt-2 text-aux text-text">{{ formError }}</p>
    </form>
  </main>
</template>
