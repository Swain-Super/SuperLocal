<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElButton, ElMessageBox, ElTable, ElTableColumn } from 'element-plus'
import {
  adminDenied,
  adminGroups,
  apiErrorMessage,
  deleteAdminGroup,
  type AdminGroup,
} from '../../api'
import { formatStamp } from '../../stores/chat'

const router = useRouter()
const groups = ref<AdminGroup[]>([])
const error = ref('')
const loading = ref(false)
const dissolvingId = ref('')

const headerCellStyle = {
  color: '#646A73',
  fontWeight: 500,
  background: '#F7F8FA',
}

function asGroup(row: unknown): AdminGroup {
  return row as AdminGroup
}

function createdAt(row: AdminGroup): string {
  return formatStamp(row.createdAt)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    groups.value = await adminGroups()
  } catch (err) {
    if (adminDenied(err)) {
      await router.replace('/admin/login')
      return
    }
    error.value = apiErrorMessage(err)
  } finally {
    loading.value = false
  }
}

async function onDissolve(row: AdminGroup) {
  if (dissolvingId.value !== '') {
    return
  }
  try {
    await ElMessageBox.confirm(`确定解散「${row.name}」？`, '解散群', {
      confirmButtonText: '解散',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  dissolvingId.value = row.id
  error.value = ''
  try {
    await deleteAdminGroup(row.id)
    groups.value = groups.value.filter((item) => item.id !== row.id)
  } catch (err) {
    if (adminDenied(err)) {
      await router.replace('/admin/login')
      return
    }
    error.value = apiErrorMessage(err)
  } finally {
    dissolvingId.value = ''
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <section>
    <h1 class="page-title">群</h1>
    <p v-if="error" class="hint">{{ error }}</p>
    <p v-if="loading" class="hint">加载中</p>
    <div class="page-card">
    <el-table
      :data="groups"
      size="small"
      :header-cell-style="headerCellStyle"
      empty-text="暂无数据"
    >
      <el-table-column prop="name" label="群名" min-width="160" show-overflow-tooltip />
      <el-table-column prop="memberCount" label="人数" width="80" />
      <el-table-column label="创建时间" min-width="160">
        <template #default="{ row }">
          {{ createdAt(asGroup(row)) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90">
        <template #default="{ row }">
          <el-button
            link
            type="primary"
            :disabled="dissolvingId === row.id"
            @click="onDissolve(asGroup(row))"
          >
            解散
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    </div>
  </section>
</template>

<style scoped>
.hint {
  margin: 12px 0 0;
  font-size: 12px;
}
</style>
