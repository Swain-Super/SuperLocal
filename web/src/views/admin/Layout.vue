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
    <div class="admin-shell sl-admin" :style="themeVars">
      <aside class="sider">
        <div class="brand">
          <span class="mark" aria-hidden="true">S</span>
          <span>SuperLocal</span>
        </div>
        <el-menu :default-active="active" router class="admin-nav">
          <el-menu-item index="/admin">概览</el-menu-item>
          <el-menu-item index="/admin/users">用户</el-menu-item>
          <el-menu-item index="/admin/groups">群</el-menu-item>
        </el-menu>
      </aside>
      <main class="main">
        <router-view />
      </main>
    </div>
  </el-config-provider>
</template>

<style scoped>
.admin-shell {
  min-height: 100vh;
  display: flex;
  background: #f3f5f7;
  color: #1f2329;
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
  font-size: 14px;
}
.sider {
  width: 220px;
  flex: none;
  background: #fff;
  border-right: 1px solid #e6e8eb;
}
.brand {
  height: 56px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  font-size: 16px;
  font-weight: 600;
}
.mark {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: #0b6bcb;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
}
.main {
  min-width: 0;
  flex: 1;
  padding: 20px 24px 32px;
}
.admin-nav {
  --el-menu-bg-color: transparent;
  --el-menu-item-height: 40px;
  --el-menu-hover-bg-color: #f3f5f7;
  border-right: none;
  background: transparent;
  padding: 8px;
}
</style>

<style>
.sl-admin .admin-nav .el-menu-item {
  border-radius: 6px;
  margin-bottom: 4px;
  color: #1f2329;
}
.sl-admin .admin-nav .el-menu-item.is-active {
  background: #e7f0fa;
  color: #0b6bcb;
  font-weight: 600;
}
.sl-admin .page-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  line-height: 32px;
}
.sl-admin .page-head {
  display: flex;
  align-items: center;
  gap: 16px;
}
.sl-admin .page-card {
  margin-top: 16px;
  background: #fff;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  padding: 4px 16px 12px;
}
.sl-admin .stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 12px;
  margin-top: 16px;
}
.sl-admin .stat {
  background: #fff;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  padding: 16px 16px 14px;
}
.sl-admin .stat p {
  margin: 0;
  color: #646a73;
  font-size: 12px;
}
.sl-admin .stat strong {
  display: block;
  margin-top: 8px;
  font-size: 22px;
  font-weight: 600;
  color: #1f2329;
}
.sl-admin .subcard {
  margin-top: 16px;
  max-width: 420px;
  background: #fff;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  padding: 16px;
}
.sl-admin .subcard h2 {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 600;
}
</style>
