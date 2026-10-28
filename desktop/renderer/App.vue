<script setup>
import { computed, onMounted, ref } from "vue";

const status = ref("stopped");
const error = ref("");
const chatUrls = ref([]);
const adminUrls = ref([]);
const username = ref("");
const password = ref("");
const secretState = ref("hidden");
const portText = ref("8080");

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
  if (bridge()) {
    bridge().start(portText.value).then(apply);
  }
}

function stop() {
  if (bridge()) {
    bridge().stop().then(apply);
  }
}
</script>

<template>
  <main class="page">
    <section class="panel">
      <h1>SuperLocal</h1>
      <p class="status" :data-status="status">{{ statusLabel }}</p>

      <label class="field">
        <span>端口</span>
        <input :value="portText" :disabled="portLocked" inputmode="numeric" @input="onPort" />
      </label>

      <div class="actions">
        <button class="primary" type="button" :disabled="!canStart" @click="start">启动</button>
        <button type="button" :disabled="!canStop" @click="stop">停止</button>
      </div>
      <p v-if="error" class="error">{{ error }}</p>

      <h2>聊天地址</h2>
      <ul>
        <li v-for="url in chatUrls" :key="url">{{ url }}</li>
      </ul>
      <button type="button" :disabled="!canOpen" @click="bridge() && bridge().openChat()">打开聊天</button>

      <h2>后台地址</h2>
      <ul>
        <li v-for="url in adminUrls" :key="url">{{ url }}</li>
      </ul>
      <button type="button" :disabled="!canOpen" @click="bridge() && bridge().openAdmin()">打开后台</button>

      <h2>管理员</h2>
      <p v-if="secretState === 'hidden'" class="meta">启动后显示</p>
      <p v-else-if="secretState === 'unreadable'" class="meta">无法读取管理员密码</p>
      <div v-else class="secret">
        <p>账号 {{ username }}</p>
        <p>密码 {{ password }}</p>
      </div>
    </section>
  </main>
</template>
