<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElButton, ElInput, ElSwitch, ElTable, ElTableColumn } from 'element-plus'
import {
  adminDenied,
  adminUsers,
  apiErrorMessage,
  patchAdminUser,
  type AdminUser,
  type AdminUserPatch,
} from '../../api'
import { formatStamp } from '../../stores/chat'

const router = useRouter()
const keyword = ref('')
const users = ref<AdminUser[]>([])
const error = ref('')
const loading = ref(false)
const savingIds = ref(new Set<string>())

const headerCellStyle = {
  color: '#646A73',
  fontWeight: 400,
}

let request = 0

async function load() {
  const current = ++request
  loading.value = true
  error.value = ''
  try {
    const list = await adminUsers(keyword.value)
    if (current !== request) {
      return
    }
    users.value = list
  } catch (err) {
    if (current !== request) {
      return
    }
    if (adminDenied(err)) {
      await router.replace('/admin/login')
      return
    }
    error.value = apiErrorMessage(err)
  } finally {
    if (current === request) {
      loading.value = false
    }
  }
}

function asUser(row: unknown): AdminUser {
  return row as AdminUser
}

function seenAt(row: AdminUser): string {
  return formatStamp(row.lastSeenAt)
}

async function onFlag(row: AdminUser, field: keyof AdminUserPatch, value: string | number | boolean) {
  const next = value === true
  if (row[field] === next || savingIds.value.has(row.id)) {
    return
  }
  const previous: AdminUserPatch & { online: boolean } = {
    banned: row.banned,
    canSendMessage: row.canSendMessage,
    canSendFile: row.canSendFile,
    canCreateGroup: row.canCreateGroup,
    online: row.online,
  }
  row[field] = next
  if (field === 'banned' && next) {
    row.online = false
  }
  const pending = new Set(savingIds.value)
  pending.add(row.id)
  savingIds.value = pending
  error.value = ''
  try {
    const body: AdminUserPatch = {
      banned: row.banned,
      canSendMessage: row.canSendMessage,
      canSendFile: row.canSendFile,
      canCreateGroup: row.canCreateGroup,
    }
    await patchAdminUser(row.id, body)
  } catch (err) {
    row.banned = previous.banned
    row.canSendMessage = previous.canSendMessage
    row.canSendFile = previous.canSendFile
    row.canCreateGroup = previous.canCreateGroup
    row.online = previous.online
    if (adminDenied(err)) {
      await router.replace('/admin/login')
      return
    }
    error.value = apiErrorMessage(err)
  } finally {
    const pending = new Set(savingIds.value)
    pending.delete(row.id)
    savingIds.value = pending
  }
}

function isSaving(id: string): boolean {
  return savingIds.value.has(id)
}

onMounted(() => {
  void load()
})
</script>

<template>
  <section>
    <h1 class="text-body font-medium">用户</h1>
    <div class="mt-4 flex items-center gap-2">
      <div class="w-60">
        <el-input
          v-model="keyword"
          clearable
          placeholder="昵称或 Key"
          @keyup.enter="load"
          @clear="load"
        />
      </div>
      <el-button type="primary" @click="load">搜索</el-button>
    </div>
    <p v-if="error" class="mt-3 text-aux text-text">{{ error }}</p>
    <p v-if="loading" class="mt-3 text-aux text-secondary">加载中</p>
    <el-table
      class="mt-3"
      :data="users"
      size="small"
      :header-cell-style="headerCellStyle"
      empty-text="暂无数据"
    >
      <el-table-column prop="nickname" label="昵称" min-width="120" show-overflow-tooltip />
      <el-table-column prop="key" label="Key" min-width="160" show-overflow-tooltip />
      <el-table-column label="在线" width="80">
        <template #default="{ row }">
          <span :class="row.online ? 'text-online' : 'text-secondary'">
            {{ row.online ? '在线' : '离线' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="最近在线" min-width="140">
        <template #default="{ row }">
          {{ seenAt(asUser(row)) }}
        </template>
      </el-table-column>
      <el-table-column label="发言" width="80">
        <template #default="{ row }">
          <el-switch
            size="small"
            :model-value="row.canSendMessage"
            :disabled="isSaving(row.id)"
            @change="onFlag(asUser(row), 'canSendMessage', $event)"
          />
        </template>
      </el-table-column>
      <el-table-column label="发文件" width="90">
        <template #default="{ row }">
          <el-switch
            size="small"
            :model-value="row.canSendFile"
            :disabled="isSaving(row.id)"
            @change="onFlag(asUser(row), 'canSendFile', $event)"
          />
        </template>
      </el-table-column>
      <el-table-column label="建群" width="80">
        <template #default="{ row }">
          <el-switch
            size="small"
            :model-value="row.canCreateGroup"
            :disabled="isSaving(row.id)"
            @change="onFlag(asUser(row), 'canCreateGroup', $event)"
          />
        </template>
      </el-table-column>
      <el-table-column label="封禁" width="80">
        <template #default="{ row }">
          <el-switch
            size="small"
            :model-value="row.banned"
            :disabled="isSaving(row.id)"
            @change="onFlag(asUser(row), 'banned', $event)"
          />
        </template>
      </el-table-column>
    </el-table>
  </section>
</template>
