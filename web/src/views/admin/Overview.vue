<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElButton, ElInput } from 'element-plus'
import {
  adminDenied,
  adminPassword,
  adminStats,
  apiErrorMessage,
  type AdminStats,
} from '../../api'
import { formatBytes } from '../../stores/chat'

const router = useRouter()
const stats = ref<AdminStats | null>(null)
const statsError = ref('')
const password = ref('')
const passwordError = ref('')
const passwordOk = ref(false)
const submitting = ref(false)

const cards = computed(() => {
  const current = stats.value
  return [
    { label: '注册用户', value: current ? String(current.users) : '—' },
    { label: '当前在线', value: current ? String(current.online) : '—' },
    { label: '封禁', value: current ? String(current.banned) : '—' },
    { label: '保留期内消息', value: current ? String(current.messages) : '—' },
    { label: '文件占用', value: current ? formatBytes(current.fileBytes) : '—' },
  ]
})

async function load() {
  statsError.value = ''
  try {
    stats.value = await adminStats()
  } catch (err) {
    if (adminDenied(err)) {
      await router.replace('/admin/login')
      return
    }
    statsError.value = apiErrorMessage(err)
  }
}

async function onPassword() {
  if (submitting.value) {
    return
  }
  passwordError.value = ''
  passwordOk.value = false
  if (password.value.trim() === '') {
    passwordError.value = '请填写新密码'
    return
  }
  submitting.value = true
  try {
    await adminPassword(password.value)
    password.value = ''
    passwordOk.value = true
  } catch (err) {
    if (adminDenied(err)) {
      await router.replace('/admin/login')
      return
    }
    passwordError.value = apiErrorMessage(err)
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <section>
    <h1 class="text-body font-medium">概览</h1>
    <div class="mt-4 grid grid-cols-5 gap-3">
      <article
        v-for="card in cards"
        :key="card.label"
        class="rounded-panel bg-page px-4 py-3"
      >
        <p class="text-aux text-secondary">{{ card.label }}</p>
        <p class="mt-2 text-body font-medium">{{ card.value }}</p>
      </article>
    </div>
    <p v-if="statsError" class="mt-3 text-aux text-text">{{ statsError }}</p>

    <h2 class="mt-8 text-body font-medium">修改密码</h2>
    <form class="mt-4 max-w-sm" @submit.prevent="onPassword">
      <label class="block text-aux text-secondary" for="admin-new-password">新密码</label>
      <el-input
        id="admin-new-password"
        v-model="password"
        class="mt-1"
        type="password"
        show-password
        autocomplete="new-password"
      />
      <el-button class="mt-3" type="primary" native-type="submit" :disabled="submitting">
        保存
      </el-button>
    </form>
    <p v-if="passwordError" class="text-aux text-text">{{ passwordError }}</p>
    <p v-if="passwordOk" class="text-aux text-online">已修改</p>
  </section>
</template>
