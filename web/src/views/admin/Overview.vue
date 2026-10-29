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
    <h1 class="page-title">概览</h1>
    <div class="stat-grid">
      <article v-for="card in cards" :key="card.label" class="stat">
        <p>{{ card.label }}</p>
        <strong>{{ card.value }}</strong>
      </article>
    </div>
    <p v-if="statsError" class="hint">{{ statsError }}</p>

    <form class="subcard" @submit.prevent="onPassword">
      <h2>修改密码</h2>
      <label for="admin-new-password">新密码</label>
      <el-input
        id="admin-new-password"
        v-model="password"
        type="password"
        show-password
        autocomplete="new-password"
      />
      <el-button class="save" type="primary" native-type="submit" :disabled="submitting">
        保存
      </el-button>
      <p v-if="passwordError" class="hint">{{ passwordError }}</p>
      <p v-if="passwordOk" class="ok">已修改</p>
    </form>
  </section>
</template>

<style scoped>
label {
  display: block;
  margin-bottom: 6px;
  color: #646a73;
  font-size: 12px;
}
.save {
  margin-top: 12px;
}
.hint,
.ok {
  margin: 8px 0 0;
  font-size: 12px;
}
.ok {
  color: #22a06b;
}
</style>
