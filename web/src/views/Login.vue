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

const keyAlphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789'
const nickHeads = ['清风', '晚霞', '星河', '青松', '海浪', '山泉', '云朵', '月光', '细雨', '暖阳', '远山', '白露']
const nickTails = ['行者', '听雨', '看海', '拾光', '慢步', '未眠', '向南', '临溪', '小筑', '初晴', '长歌', '归舟']

function randomIndex(size: number): number {
  const buf = new Uint32Array(1)
  crypto.getRandomValues(buf)
  return buf[0] % size
}

function randomKey(): string {
  const buf = new Uint8Array(12)
  crypto.getRandomValues(buf)
  return Array.from(buf, (value) => keyAlphabet[value % keyAlphabet.length]).join('')
}

function randomNickname(): string {
  const suffix = String(randomIndex(100)).padStart(2, '0')
  return `${nickHeads[randomIndex(nickHeads.length)]}${nickTails[randomIndex(nickTails.length)]}${suffix}`
}

function fillKey() {
  key.value = randomKey()
  keyError.value = ''
}

function fillNickname() {
  nickname.value = randomNickname()
}

async function registerOnce() {
  fillKey()
  fillNickname()
  await onSubmit()
}

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
  <main class="auth">
    <form class="card" @submit.prevent="onSubmit">
      <div class="brand">
        <span class="mark" aria-hidden="true">S</span>
        <div>
          <h1>SuperLocal</h1>
          <p>用 Key 进入，Key 即账号</p>
        </div>
      </div>

      <div class="field-head">
        <label for="chat-key">Key</label>
        <button type="button" class="text" @click="fillKey">随机</button>
      </div>
      <input
        id="chat-key"
        v-model="key"
        name="key"
        type="text"
        autocomplete="username"
      />
      <p v-if="keyError" class="error">{{ keyError }}</p>

      <div class="field-head">
        <label for="chat-nickname">昵称</label>
        <button type="button" class="text" @click="fillNickname">随机</button>
      </div>
      <input
        id="chat-nickname"
        v-model="nickname"
        name="nickname"
        type="text"
        autocomplete="nickname"
      />

      <button class="primary" type="submit" :disabled="submitting">进入</button>
      <button class="ghost" type="button" :disabled="submitting" @click="registerOnce">一键注册</button>
      <p v-if="formError" class="error">{{ formError }}</p>
    </form>
  </main>
</template>

<style scoped>
.auth {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: #f3f5f7;
  color: #1f2329;
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
  font-size: 14px;
}
.card {
  width: 100%;
  max-width: 400px;
  margin: 0;
  padding: 32px 32px 28px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(31, 35, 41, 0.08);
}
.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 28px;
}
.mark {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  background: #0b6bcb;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 600;
  flex: none;
}
h1 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  line-height: 1.3;
}
.brand p {
  margin: 4px 0 0;
  color: #646a73;
  font-size: 12px;
}
.field-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 14px 0 6px;
}
label {
  display: block;
  margin: 0;
  color: #646a73;
  font-size: 12px;
}
input {
  box-sizing: border-box;
  width: 100%;
  height: 40px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: #f3f5f7;
  padding: 0 12px;
  color: #1f2329;
  font: inherit;
  outline: none;
}
input:focus {
  border-color: #0b6bcb;
  background: #fff;
}
button {
  border: 0;
  font: inherit;
  cursor: pointer;
}
button:disabled {
  opacity: 0.6;
  cursor: default;
}
.primary {
  width: 100%;
  height: 40px;
  margin-top: 24px;
  border-radius: 6px;
  background: #0b6bcb;
  color: #fff;
}
.primary:hover:not(:disabled) {
  background: #0a61b8;
}
.primary:active:not(:disabled) {
  background: #0958a8;
}
.text {
  padding: 0;
  background: transparent;
  color: #0b6bcb;
  font-size: 12px;
}
.text:hover:not(:disabled) {
  color: #0958a8;
}
.ghost {
  width: 100%;
  height: 40px;
  margin-top: 8px;
  border-radius: 6px;
  border: 1px solid #e6e8eb;
  background: #fff;
  color: #0b6bcb;
}
.ghost:hover:not(:disabled) {
  background: #f3f5f7;
}
.ghost:active:not(:disabled) {
  background: #e7f0fa;
}
.error {
  margin: 6px 0 0;
  color: #1f2329;
  font-size: 12px;
}
</style>
