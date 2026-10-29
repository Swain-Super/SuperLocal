<script setup>
import { computed, onMounted, ref } from "vue";
import { parsePort } from "../src/port.js";

const status = ref("stopped");
const error = ref("");
const chatUrls = ref([]);
const adminUrls = ref([]);
const username = ref("");
const password = ref("");
const secretState = ref("hidden");
const portText = ref("8080");
const toast = ref("");
let toastTimer = 0;

const labels = {
  stopped: "已停止",
  starting: "启动中",
  running: "运行中",
  failed: "启动失败",
};

const canStart = computed(() => status.value === "stopped" || status.value === "failed");
const canStop = computed(() => status.value === "starting" || status.value === "running");
const canOpen = computed(() => status.value === "running");
const portLocked = computed(() => status.value === "starting" || status.value === "running");
const statusLabel = computed(() => labels[status.value] || "已停止");
const showAddresses = computed(() => chatUrls.value.length > 0 || adminUrls.value.length > 0);
const secretReady = computed(() => secretState.value === "ready");
const portOk = computed(() => parsePort(portText.value).ok);
const portHint = computed(() => (portLocked.value || portOk.value ? "" : "请输入 1 到 65535 的端口"));
const startReady = computed(() => canStart.value && portOk.value);

function apply(next) {
  if (!next) {
    return;
  }
  status.value = next.status;
  error.value = next.error || "";
  chatUrls.value = next.chatUrls || [];
  adminUrls.value = next.adminUrls || [];
  username.value = next.username || "";
  password.value = next.password || "";
  secretState.value = next.secretState || "hidden";
  if (next.port != null && portLocked.value) {
    portText.value = String(next.port);
  }
}

function bridge() {
  return window.launcher;
}

onMounted(async () => {
  if (!bridge()) {
    return;
  }
  const current = await bridge().getState();
  apply(current);
  portText.value = String(current.port);
  bridge().onState(apply);
});

function onPort(event) {
  portText.value = event.target.value;
  if (bridge()) {
    bridge().setPort(portText.value);
  }
}

function start() {
  if (!startReady.value || !bridge()) {
    return;
  }
  bridge().start(portText.value).then(apply);
}

function onEnter() {
  start();
}

function stop() {
  if (bridge()) {
    bridge().stop().then(apply);
  }
}

function refreshAddresses() {
  if (!bridge() || status.value !== "running") {
    return;
  }
  bridge().refreshAddresses().then((next) => {
    apply(next);
    toast.value = next?.addressChanged ? "已换成新地址" : "地址没有变化";
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      toast.value = "";
    }, 1600);
  });
}

function selectPort(event) {
  event.target.select();
}

async function copy(text) {
  if (!bridge() || !text) {
    return;
  }
  const ok = await bridge().copy(text);
  if (!ok) {
    return;
  }
  toast.value = "已复制到剪贴板";
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toast.value = "";
  }, 1600);
}

function hostKind(url) {
  try {
    const host = new URL(url).hostname;
    if (host === "127.0.0.1" || host === "localhost") {
      return "本机";
    }
    return "局域网";
  } catch {
    return "";
  }
}
</script>

<template>
  <main class="page">
    <header class="hero">
      <div class="brand">
        <span class="mark" aria-hidden="true">S</span>
        <div>
          <p class="brand-name">SuperLocal</p>
          <p class="brand-desc">局域网聊天服务</p>
        </div>
      </div>
      <p class="badge" :data-status="status">
        <span class="dot" aria-hidden="true"></span>
        {{ statusLabel }}
      </p>
    </header>

    <section class="block control">
      <div class="port-row">
        <label class="port-field">
          <span class="label">端口</span>
          <input
            :value="portText"
            :disabled="portLocked"
            :aria-invalid="portHint ? 'true' : 'false'"
            inputmode="numeric"
            autocomplete="off"
            spellcheck="false"
            @focus="selectPort"
            @input="onPort"
            @keydown.enter.prevent="onEnter"
          />
        </label>
        <div class="control-actions">
          <button
            v-if="canStart"
            class="primary"
            type="button"
            :disabled="!startReady"
            @click="start"
          >
            启动
          </button>
          <button
            v-else
            class="danger"
            type="button"
            :disabled="!canStop"
            @click="stop"
          >
            停止
          </button>
        </div>
      </div>
      <p v-if="portHint" class="field-error" role="alert">{{ portHint }}</p>
      <p v-else-if="error" class="error" role="alert">{{ error }}</p>
      <div v-if="canOpen" class="open-actions">
        <button class="primary" type="button" @click="bridge() && bridge().openChat()">打开聊天</button>
        <button type="button" @click="bridge() && bridge().openAdmin()">打开后台</button>
      </div>
    </section>

    <section class="block">
      <div class="section-head">
        <h2>访问地址</h2>
        <div class="head-side">
          <p class="hint">发给同一网络里的人</p>
          <button class="ghost" type="button" :disabled="status !== 'running'" @click="refreshAddresses">刷新</button>
        </div>
      </div>
      <template v-if="showAddresses">
        <div class="group">
          <p class="group-title">聊天</p>
          <ul>
            <li v-for="url in chatUrls" :key="url">
              <span class="tag">{{ hostKind(url) }}</span>
              <code>{{ url }}</code>
              <button class="ghost" type="button" @click="copy(url)">复制</button>
            </li>
          </ul>
        </div>
        <div class="group">
          <p class="group-title">后台</p>
          <ul>
            <li v-for="url in adminUrls" :key="url">
              <span class="tag">{{ hostKind(url) }}</span>
              <code>{{ url }}</code>
              <button class="ghost" type="button" @click="copy(url)">复制</button>
            </li>
          </ul>
        </div>
      </template>
      <p v-else-if="status === 'running'" class="empty">当前没有局域网地址</p>
      <p v-else class="empty">启动后显示局域网地址</p>
    </section>

    <section class="block">
      <div class="section-head">
        <h2>管理员</h2>
        <p class="hint">仅本机管理页使用</p>
      </div>
      <p v-if="secretState === 'hidden'" class="empty">启动后显示</p>
      <p v-else-if="secretState === 'unreadable'" class="empty">无法读取管理员密码</p>
      <div v-else-if="secretReady" class="creds">
        <div class="cred">
          <span class="label">账号</span>
          <div class="cred-value">
            <code>{{ username }}</code>
            <button class="ghost" type="button" @click="copy(username)">复制</button>
          </div>
        </div>
        <div class="cred">
          <span class="label">密码</span>
          <div class="cred-value">
            <code>{{ password }}</code>
            <button class="ghost" type="button" @click="copy(password)">复制</button>
          </div>
        </div>
      </div>
    </section>
    <p v-if="toast" class="toast" role="status">{{ toast }}</p>
  </main>
</template>
