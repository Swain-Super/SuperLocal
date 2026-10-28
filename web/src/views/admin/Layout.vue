<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { ElConfigProvider, ElMenu, ElMenuItem } from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/es/components/config-provider/style/css'
import 'element-plus/es/components/menu/style/css'
import 'element-plus/es/components/table/style/css'
import 'element-plus/es/components/switch/style/css'
import 'element-plus/es/components/button/style/css'
import 'element-plus/es/components/input/style/css'
import 'element-plus/es/components/message-box/style/css'

const themeVars: Record<string, string> = {
  '--el-color-primary': '#0B6BCB',
  '--el-color-primary-rgb': '11, 107, 203',
  '--el-color-primary-light-3': '#5497db',
  '--el-color-primary-light-5': '#85b5e5',
  '--el-color-primary-light-7': '#b6d3ef',
  '--el-color-primary-light-8': '#cee1f5',
  '--el-color-primary-light-9': '#e7f0fa',
  '--el-color-primary-dark-2': '#0958A8',
  '--el-font-family': '"PingFang SC", "Microsoft YaHei", sans-serif',
  '--el-font-size-base': '14px',
  '--el-text-color-primary': '#1F2329',
  '--el-text-color-regular': '#1F2329',
  '--el-text-color-secondary': '#646A73',
  '--el-border-color': '#E6E8EB',
}

// 确认框挂到 body，不在侧栏节点下，所以主色也写到文档根上。
for (const [key, value] of Object.entries(themeVars)) {
  document.documentElement.style.setProperty(key, value)
}

const route = useRoute()
const active = computed(() => (route.path === '/admin/' ? '/admin' : route.path))
</script>

<template>
  <el-config-provider :locale="zhCn">
    <div
      class="admin-shell flex min-h-screen bg-page font-sans text-body text-text"
      :style="themeVars"
    >
      <aside class="w-[200px] shrink-0 border-r border-border bg-page">
        <div class="h-12 flex items-center px-5 font-medium">SuperLocal</div>
        <el-menu :default-active="active" router class="admin-nav">
          <el-menu-item index="/admin">概览</el-menu-item>
          <el-menu-item index="/admin/users">用户</el-menu-item>
          <el-menu-item index="/admin/groups">群</el-menu-item>
        </el-menu>
      </aside>
      <main class="min-w-0 flex-1 bg-panel p-6">
        <router-view />
      </main>
    </div>
  </el-config-provider>
</template>

<style scoped>
.admin-nav {
  --el-menu-bg-color: transparent;
  --el-menu-item-height: 40px;
  border-right: none;
  background-color: transparent;
}
</style>
