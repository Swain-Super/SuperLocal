<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  apiErrorMessage,
  isImageMime,
  searchUsers,
  type Conversation,
  type PublicUser,
} from '../api'
import {
  fileMeta,
  formatBytes,
  formatStamp,
  listPreview,
  runeCount,
  useChatStore,
} from '../stores/chat'

const store = useChatStore()
const router = useRouter()

const pane = ref<'messages' | 'contacts'>('messages')
const keyword = ref('')
const showThread = ref(false)
const narrow = ref(false)
const draft = ref('')
const composing = ref(false)
const profileOpen = ref(false)
const nickname = ref('')
const profileError = ref('')
const profileBusy = ref(false)
const picker = ref<'create' | 'add' | null>(null)
const groupName = ref('')
const userQuery = ref('')
const userHits = ref<PublicUser[]>([])
const selected = ref<PublicUser[]>([])
const pickerError = ref('')
const pickerBusy = ref(false)
const searching = ref(false)
const dragChat = ref(false)
const dragGroupId = ref('')
const dragContactId = ref('')

const scroller = ref<HTMLElement | null>(null)
const imageInput = ref<HTMLInputElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const avatarInput = ref<HTMLInputElement | null>(null)

let compositionTimer = 0
let searchTimer = 0
let removeMedia = () => {}

const filteredConversations = computed(() => {
  const q = keyword.value.trim()
  if (!q) {
    return store.conversations
  }
  return store.conversations.filter((item) => item.title.includes(q))
})

const filteredContacts = computed(() => {
  const selfId = store.me?.id
  const online = store.onlineUsers.filter((user) => user.id !== selfId)
  const q = keyword.value.trim()
  if (!q) {
    return online
  }
  return online.filter((user) => user.nickname.includes(q))
})

const canSendMessage = computed(() => store.me?.canSendMessage === true)
const canSendFile = computed(() => store.me?.canSendFile === true)

function initial(name: string): string {
  return Array.from(name.trim())[0] ?? ''
}

function peerOffline(conv: Conversation): boolean {
  return conv.type === 'direct' && !!conv.peer && !conv.peer.online
}

function selfAvatarSrc(): string {
  const url = store.me?.avatarUrl ?? ''
  if (!url) {
    return ''
  }
  const joiner = url.includes('?') ? '&' : '?'
  return `${url}${joiner}v=${store.avatarNonce}`
}

function fileName(body: string): string {
  return fileMeta(body)?.name ?? (body || '文件')
}

function fileSize(body: string): string {
  const size = fileMeta(body)?.size
  if (size == null) {
    return ''
  }
  return formatBytes(size)
}

function takeFiles(event: DragEvent): File[] {
  return Array.from(event.dataTransfer?.files ?? [])
}

function stayingInside(event: DragEvent): boolean {
  const next = event.relatedTarget
  const current = event.currentTarget
  return next instanceof Node && current instanceof Node && current.contains(next)
}

function onGroupDragOver(conv: Conversation, event: DragEvent) {
  if (conv.type !== 'group') {
    return
  }
  event.preventDefault()
  dragGroupId.value = conv.id
}

function onGroupDragLeave(conv: Conversation, event: DragEvent) {
  if (stayingInside(event) || dragGroupId.value !== conv.id) {
    return
  }
  dragGroupId.value = ''
}

function onGroupDrop(conv: Conversation, event: DragEvent) {
  if (conv.type !== 'group') {
    return
  }
  event.preventDefault()
  event.stopPropagation()
  dragGroupId.value = ''
  const files = takeFiles(event)
  if (files.length === 0) {
    return
  }
  void store.sendFiles(conv.id, files)
}

function onChatDragLeave(event: DragEvent) {
  if (stayingInside(event)) {
    return
  }
  dragChat.value = false
}

function onContactDragLeave(userId: string, event: DragEvent) {
  if (stayingInside(event) || dragContactId.value !== userId) {
    return
  }
  dragContactId.value = ''
}

async function stickToBottom(force = false) {
  await nextTick()
  const el = scroller.value
  if (!el) {
    return
  }
  const near = el.scrollHeight - el.scrollTop - el.clientHeight < 80
  if (force || near) {
    el.scrollTop = el.scrollHeight
  }
}

async function openConversation(conv: Conversation) {
  try {
    await store.select(conv)
    store.notice = ''
    showThread.value = true
    draft.value = ''
    await stickToBottom(true)
  } catch (err) {
    store.notice = apiErrorMessage(err)
  }
}

async function openContact(user: PublicUser) {
  try {
    await store.openDirect(user)
    store.notice = ''
    showThread.value = true
    draft.value = ''
    await stickToBottom(true)
  } catch (err) {
    store.notice = apiErrorMessage(err)
  }
}

function submitText() {
  const conv = store.active
  if (!conv) {
    return
  }
  const text = draft.value
  if (store.sendText(conv.id, text)) {
    draft.value = ''
  }
}

function onCompositionStart() {
  window.clearTimeout(compositionTimer)
  composing.value = true
}

function onCompositionEnd() {
  // compositionend 之后，同一轮事件里还可能再来一次 Enter。延后清除，组字回车才不会发出去。
  window.clearTimeout(compositionTimer)
  compositionTimer = window.setTimeout(() => {
    composing.value = false
  }, 0)
}

function onComposerKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.shiftKey) {
    return
  }
  const ime = event.isComposing || event.keyCode === 229
  if (ime) {
    return
  }
  // compositionend 之后紧跟着的 Enter 已经不是组字，只取消换行，不发送。
  if (composing.value) {
    event.preventDefault()
    return
  }
  event.preventDefault()
  submitText()
}

async function onPick(event: Event) {
  const input = event.target
  if (!(input instanceof HTMLInputElement)) {
    return
  }
  const files = Array.from(input.files ?? [])
  input.value = ''
  const conv = store.active
  if (!conv || files.length === 0) {
    return
  }
  await store.sendFiles(conv.id, files)
}

async function onDropChat(event: DragEvent) {
  dragChat.value = false
  const files = takeFiles(event)
  const conv = store.active
  if (!conv || files.length === 0) {
    return
  }
  await store.sendFiles(conv.id, files)
}

async function onDropContact(user: PublicUser, event: DragEvent) {
  dragContactId.value = ''
  const files = takeFiles(event)
  if (files.length === 0) {
    return
  }
  try {
    await store.openDirect(user)
    showThread.value = true
    if (store.activeId) {
      await store.sendFiles(store.activeId, files)
    }
    await stickToBottom(true)
  } catch (err) {
    store.notice = apiErrorMessage(err)
  }
}

function openProfile() {
  nickname.value = store.me?.nickname ?? ''
  profileError.value = ''
  profileOpen.value = true
}

async function saveProfile() {
  const nick = nickname.value.trim()
  if (nick.includes('\n') || nick.includes('\r')) {
    profileError.value = '昵称不能包含换行'
    return
  }
  const count = runeCount(nick)
  if (count < 1 || count > 32) {
    profileError.value = '昵称长度须为 1–32 个字'
    return
  }
  profileBusy.value = true
  profileError.value = ''
  try {
    await store.saveNickname(nick)
    profileOpen.value = false
  } catch (err) {
    profileError.value = apiErrorMessage(err)
  } finally {
    profileBusy.value = false
  }
}

async function onAvatar(event: Event) {
  const input = event.target
  if (!(input instanceof HTMLInputElement)) {
    return
  }
  const file = input.files?.[0]
  input.value = ''
  if (!file) {
    return
  }
  if (file.size > 2 * 1024 * 1024) {
    profileError.value = '头像不能超过 2MB'
    return
  }
  if (file.type && !isImageMime(file.type)) {
    profileError.value = '不支持的头像类型'
    return
  }
  profileBusy.value = true
  profileError.value = ''
  try {
    await store.saveAvatar(file)
  } catch (err) {
    profileError.value = apiErrorMessage(err)
  } finally {
    profileBusy.value = false
  }
}

async function onLogout() {
  profileBusy.value = true
  profileError.value = ''
  try {
    await store.logout()
    await router.push('/login')
  } catch (err) {
    profileError.value = apiErrorMessage(err)
    profileBusy.value = false
  }
}

function openPicker(mode: 'create' | 'add') {
  picker.value = mode
  groupName.value = ''
  userQuery.value = ''
  selected.value = []
  pickerError.value = ''
  userHits.value = []
  void runUserSearch()
}

async function runUserSearch() {
  if (!picker.value) {
    return
  }
  searching.value = true
  try {
    const users = await searchUsers(userQuery.value.trim())
    userHits.value = users.filter((user) => user.id !== store.me?.id)
  } catch (err) {
    pickerError.value = apiErrorMessage(err)
  } finally {
    searching.value = false
  }
}

function toggleUser(user: PublicUser) {
  const index = selected.value.findIndex((item) => item.id === user.id)
  if (index >= 0) {
    selected.value.splice(index, 1)
    return
  }
  selected.value.push(user)
}

function selectedHas(id: string): boolean {
  return selected.value.some((item) => item.id === id)
}

async function submitPicker() {
  if (!picker.value) {
    return
  }
  pickerError.value = ''
  if (selected.value.length === 0) {
    pickerError.value = '至少再选择一名成员'
    return
  }
  const memberIds = selected.value.map((user) => user.id)
  pickerBusy.value = true
  try {
    if (picker.value === 'create') {
      const name = groupName.value.trim()
      if (!name) {
        pickerError.value = '群名不能为空'
        return
      }
      if (runeCount(name) > 40) {
        pickerError.value = '群名最长 40 字'
        return
      }
      await store.createGroup(name, memberIds)
      showThread.value = true
    } else if (store.active) {
      await store.addMembers(store.active.id, memberIds)
    }
    picker.value = null
  } catch (err) {
    pickerError.value = apiErrorMessage(err)
  } finally {
    pickerBusy.value = false
  }
}

async function onLeave() {
  const conv = store.active
  if (!conv) {
    return
  }
  try {
    await store.leave(conv.id)
    showThread.value = false
  } catch (err) {
    store.notice = apiErrorMessage(err)
  }
}

async function leaveBanned() {
  try {
    await store.logout()
  } catch {
    store.stop()
  }
  await router.push('/login')
}

function onWindowKey(event: KeyboardEvent) {
  if (event.key !== 'Escape') {
    return
  }
  profileOpen.value = false
  picker.value = null
}

watch(userQuery, () => {
  if (!picker.value) {
    return
  }
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    void runUserSearch()
  }, 200)
})

watch(() => store.activeMessages.length, () => {
  void stickToBottom()
})

watch(() => store.activeId, () => {
  void stickToBottom(true)
})

onMounted(() => {
  const media = window.matchMedia('(max-width: 959px)')
  const apply = () => {
    narrow.value = media.matches
  }
  apply()
  media.addEventListener('change', apply)
  removeMedia = () => media.removeEventListener('change', apply)
  window.addEventListener('keydown', onWindowKey)
  void store.bootstrap()
})

onUnmounted(() => {
  removeMedia()
  window.removeEventListener('keydown', onWindowKey)
  window.clearTimeout(compositionTimer)
  window.clearTimeout(searchTimer)
  store.stop()
})
</script>

<template>
  <main
    v-if="store.banned"
    class="flex min-h-screen items-center justify-center bg-page px-4 font-sans text-body text-text"
  >
    <div class="w-full max-w-sm border border-border rounded-panel bg-panel px-6 py-5">
      <h1 class="text-body font-medium">账号已被封禁</h1>
      <p class="mt-2 text-aux text-secondary">该 Key 不能再登录。连接已关闭。</p>
      <button
        type="button"
        class="mt-4 h-8 w-full rounded-panel bg-primary text-body text-panel active:bg-pressed"
        @click="leaveBanned"
      >
        返回登录
      </button>
    </div>
  </main>

  <div
    v-else
    class="flex h-screen flex-col overflow-hidden bg-page font-sans text-body text-text"
    @dragover.prevent
    @drop.prevent
  >
    <header class="flex h-12 shrink-0 items-center gap-2 border-b border-border bg-panel px-3">
      <span class="inline-flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-page text-aux text-secondary">
        <img v-if="selfAvatarSrc()" :src="selfAvatarSrc()" alt="" class="h-full w-full object-cover" />
        <template v-else>{{ initial(store.me?.nickname ?? '') }}</template>
      </span>
      <span class="min-w-0 truncate">{{ store.me?.nickname }}</span>
      <span
        class="inline-block h-2 w-2 shrink-0 rounded-full"
        :class="store.connected ? 'bg-online' : 'bg-border'"
      />
      <span class="text-aux text-secondary">{{ store.connected ? '在线' : '离线' }}</span>
      <button
        type="button"
        class="ml-auto h-8 rounded-panel border border-border bg-panel px-3 text-aux text-text active:bg-pressed active:text-panel"
        @click="openProfile"
      >
        个人资料
      </button>
    </header>

    <p v-if="store.notice" class="shrink-0 border-b border-border bg-panel px-3 py-1 text-aux text-text">
      {{ store.notice }}
    </p>

    <div class="flex min-h-0 flex-1">
      <nav
        class="w-[80px] shrink-0 flex-col gap-1 border-r border-border bg-panel p-2"
        :class="narrow && showThread ? 'hidden' : 'flex'"
      >
        <button
          type="button"
          class="h-8 rounded-panel text-body"
          :class="pane === 'messages' ? 'bg-primary text-panel' : 'text-secondary'"
          @click="pane = 'messages'"
        >
          消息
        </button>
        <button
          type="button"
          class="h-8 rounded-panel text-body"
          :class="pane === 'contacts' ? 'bg-primary text-panel' : 'text-secondary'"
          @click="pane = 'contacts'"
        >
          联系人
        </button>
      </nav>

      <section
        class="flex-col border-r border-border bg-panel"
        :class="[
          narrow && showThread ? 'hidden' : 'flex',
          narrow ? 'min-w-0 w-auto flex-1' : 'w-[280px] shrink-0',
        ]"
      >
        <div class="border-b border-border p-2">
          <input
            v-model="keyword"
            type="search"
            placeholder="搜索"
            class="box-border h-8 w-full border border-border rounded-panel bg-panel px-2 text-body text-text outline-none focus:border-primary"
          />
        </div>

        <div v-if="pane === 'contacts' && store.me?.canCreateGroup" class="border-b border-border p-2">
          <button
            type="button"
            class="h-8 w-full rounded-panel bg-primary text-body text-panel active:bg-pressed"
            @click="openPicker('create')"
          >
            建群
          </button>
        </div>

        <div class="min-h-0 flex-1 overflow-y-auto">
          <p v-if="!store.ready" class="px-3 py-6 text-center text-aux text-secondary">加载中</p>

          <template v-else-if="pane === 'messages'">
            <p v-if="filteredConversations.length === 0" class="px-3 py-6 text-center text-aux text-secondary">
              没有会话
            </p>
            <button
              v-for="conv in filteredConversations"
              :key="conv.id"
              type="button"
              class="flex w-full items-center gap-2 border-b border-border px-3 py-2 text-left"
              :class="[
                store.activeId === conv.id ? 'bg-page' : 'bg-panel',
                dragGroupId === conv.id ? 'border-primary' : '',
              ]"
              @click="openConversation(conv)"
              @dragover="onGroupDragOver(conv, $event)"
              @dragleave="onGroupDragLeave(conv, $event)"
              @drop="onGroupDrop(conv, $event)"
            >
              <span
                class="inline-flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-page text-aux text-secondary"
                :class="{ 'offline-avatar': peerOffline(conv) }"
              >
                <img
                  v-if="conv.peer?.avatarUrl"
                  :src="conv.peer.avatarUrl"
                  alt=""
                  class="h-full w-full object-cover"
                />
                <template v-else>{{ initial(conv.title) }}</template>
              </span>
              <span class="min-w-0 flex-1">
                <span class="flex items-center gap-1">
                  <span class="min-w-0 flex-1 truncate">{{ conv.title }}</span>
                  <span v-if="peerOffline(conv)" class="shrink-0 text-aux text-secondary">离线</span>
                  <span class="shrink-0 text-aux text-secondary">{{ formatStamp(conv.lastAt) }}</span>
                </span>
                <span class="mt-0.5 flex items-center gap-2">
                  <span class="min-w-0 flex-1 truncate text-aux text-secondary">{{ listPreview(conv.lastBody) }}</span>
                  <span
                    v-if="conv.unread > 0"
                    class="shrink-0 rounded-full bg-primary px-1.5 text-aux text-panel"
                  >
                    {{ conv.unread > 99 ? '99+' : conv.unread }}
                  </span>
                </span>
              </span>
            </button>
          </template>

          <template v-else>
            <p v-if="filteredContacts.length === 0" class="px-3 py-6 text-center text-aux text-secondary">
              没有在线联系人
            </p>
            <button
              v-for="user in filteredContacts"
              :key="user.id"
              type="button"
              class="flex w-full items-center gap-2 border-b border-border px-3 py-2 text-left bg-panel"
              @click="openContact(user)"
            >
              <span
                class="inline-flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-page text-aux text-secondary"
                :class="dragContactId === user.id ? 'outline outline-1 outline-primary' : ''"
                @dragover.prevent="dragContactId = user.id"
                @dragleave="onContactDragLeave(user.id, $event)"
                @drop.stop.prevent="onDropContact(user, $event)"
                @click.stop="openContact(user)"
              >
                <img v-if="user.avatarUrl" :src="user.avatarUrl" alt="" class="h-full w-full object-cover" />
                <template v-else>{{ initial(user.nickname) }}</template>
              </span>
              <span class="min-w-0 flex-1 truncate">{{ user.nickname }}</span>
              <span class="inline-block h-2 w-2 shrink-0 rounded-full bg-online" />
            </button>
          </template>
        </div>
      </section>

      <section
        class="min-w-0 flex-1 flex-col bg-page"
        :class="narrow && !showThread ? 'hidden' : 'flex'"
        @dragover.prevent="dragChat = true"
        @dragleave="onChatDragLeave"
        @drop.stop.prevent="onDropChat"
      >
        <header class="flex h-12 shrink-0 items-center gap-2 overflow-hidden border-b border-border bg-panel px-3">
          <button
            v-if="narrow && showThread"
            type="button"
            class="h-8 shrink-0 rounded-panel border border-border bg-panel px-2 text-aux text-text"
            @click="showThread = false"
          >
            返回
          </button>
          <template v-if="store.active">
            <span
              class="inline-flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-page text-aux text-secondary"
              :class="{ 'offline-avatar': peerOffline(store.active) }"
            >
              <img
                v-if="store.active.peer?.avatarUrl"
                :src="store.active.peer.avatarUrl"
                alt=""
                class="h-full w-full object-cover"
              />
              <template v-else>{{ initial(store.active.title) }}</template>
            </span>
            <span class="min-w-0 flex-1 truncate">{{ store.active.title }}</span>
            <span class="shrink-0 text-aux text-secondary">
              <template v-if="store.active.type === 'group'">{{ store.active.memberCount }} 人</template>
              <template v-else>{{ store.active.peer?.online ? '在线' : '离线' }}</template>
            </span>
            <button
              v-if="store.canAddMembers(store.active.id)"
              type="button"
              class="h-8 shrink-0 rounded-panel border border-border bg-panel px-2 text-aux text-text"
              @click="openPicker('add')"
            >
              添加成员
            </button>
            <button
              v-if="store.active.type === 'group'"
              type="button"
              class="h-8 shrink-0 rounded-panel border border-border bg-panel px-2 text-aux text-text"
              @click="onLeave"
            >
              退出群
            </button>
          </template>
        </header>

        <div ref="scroller" class="min-h-0 flex-1 overflow-y-auto px-3 py-3" :class="dragChat ? 'bg-panel' : ''">
          <p v-if="!store.active" class="py-6 text-center text-aux text-secondary">选择一个会话</p>
          <p
            v-else-if="store.activeMessages.length === 0"
            class="py-6 text-center text-aux text-secondary"
          >
            没有消息
          </p>
          <div
            v-for="msg in store.activeMessages"
            :key="msg.id"
            class="mb-3 flex"
            :class="msg.senderId === store.me?.id ? 'justify-end' : 'justify-start'"
          >
            <div
              class="flex max-w-[75%] flex-col"
              :class="msg.senderId === store.me?.id ? 'items-end' : 'items-start'"
            >
              <p class="mb-1 text-aux text-secondary">
                {{ msg.senderNickname }} {{ formatStamp(msg.createdAt) }}
              </p>
              <div
                v-if="msg.kind === 'image' && msg.fileId"
                class="rounded-panel p-1"
                :class="msg.senderId === store.me?.id ? 'bg-primary' : 'bg-bubble'"
              >
                <img
                  :src="`/api/files/${msg.fileId}`"
                  alt="图片"
                  class="block h-auto max-w-[min(320px,100%)]"
                  @load="stickToBottom()"
                />
              </div>
              <a
                v-else-if="msg.kind === 'file' && msg.fileId"
                class="flex w-[320px] max-w-full items-center gap-2 border border-border rounded-panel bg-panel px-2 py-1.5 no-underline"
                :href="`/api/files/${msg.fileId}`"
                :download="fileName(msg.body)"
              >
                <svg viewBox="0 0 16 16" class="h-4 w-4 shrink-0 text-secondary" aria-hidden="true">
                  <path
                    fill="currentColor"
                    d="M4 1.5h5.2L12.5 4.8V14a.5.5 0 0 1-.5.5H4a.5.5 0 0 1-.5-.5V2a.5.5 0 0 1 .5-.5zm4.8 1H4.5v11h7V5.3H8.8V2.5z"
                  />
                </svg>
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-body text-text">{{ fileName(msg.body) }}</span>
                  <span v-if="fileSize(msg.body)" class="block text-aux text-secondary">{{ fileSize(msg.body) }}</span>
                </span>
                <span class="shrink-0 text-aux text-primary">下载</span>
              </a>
              <p
                v-else
                class="whitespace-pre-wrap break-words rounded-panel px-3 py-2"
                :class="msg.senderId === store.me?.id ? 'bg-primary text-panel' : 'bg-bubble text-text'"
              >
                {{ msg.body }}
              </p>
            </div>
          </div>
        </div>

        <form class="shrink-0 border-t border-border bg-panel p-3" @submit.prevent="submitText">
          <div class="mb-2 flex gap-2">
            <button
              type="button"
              class="h-8 rounded-panel border border-border bg-panel px-3 text-aux text-text disabled:opacity-60"
              :disabled="!canSendFile || !canSendMessage || !store.active"
              @click="imageInput?.click()"
            >
              图片
            </button>
            <button
              type="button"
              class="h-8 rounded-panel border border-border bg-panel px-3 text-aux text-text disabled:opacity-60"
              :disabled="!canSendFile || !canSendMessage || !store.active"
              @click="fileInput?.click()"
            >
              文件
            </button>
            <input
              ref="imageInput"
              type="file"
              accept="image/jpeg,image/png,image/webp,image/gif"
              class="hidden"
              multiple
              @change="onPick"
            />
            <input ref="fileInput" type="file" class="hidden" multiple @change="onPick" />
          </div>
          <div class="flex items-end gap-2">
            <textarea
              v-model="draft"
              rows="4"
              class="box-border max-h-[6.5rem] min-h-[2rem] flex-1 resize-none overflow-y-auto border border-border rounded-panel bg-panel px-2 py-1 text-body text-text outline-none focus:border-primary disabled:opacity-60"
              :disabled="!canSendMessage || !store.active"
              :placeholder="canSendMessage ? '' : '没有发言权限'"
              @keydown="onComposerKeydown"
              @compositionstart="onCompositionStart"
              @compositionend="onCompositionEnd"
            />
            <button
              type="submit"
              class="h-8 shrink-0 rounded-panel bg-primary px-4 text-body text-panel active:bg-pressed disabled:opacity-60"
              :disabled="!canSendMessage || !store.active"
            >
              发送
            </button>
          </div>
        </form>
      </section>
    </div>

    <div
      v-if="profileOpen"
      class="fixed inset-0 z-30 flex items-center justify-center bg-text/40 px-4"
      @click.self="profileOpen = false"
    >
      <form
        class="w-full max-w-sm border border-border rounded-panel bg-panel px-6 py-5"
        @submit.prevent="saveProfile"
      >
        <h2 class="text-body font-medium">个人资料</h2>
        <div class="mt-4 flex items-center gap-3">
          <span class="inline-flex h-9 w-9 items-center justify-center overflow-hidden rounded-full bg-page text-aux text-secondary">
            <img v-if="selfAvatarSrc()" :src="selfAvatarSrc()" alt="" class="h-full w-full object-cover" />
            <template v-else>{{ initial(store.me?.nickname ?? '') }}</template>
          </span>
          <button
            type="button"
            class="h-8 rounded-panel border border-border bg-panel px-3 text-aux text-text"
            :disabled="profileBusy"
            @click="avatarInput?.click()"
          >
            更换头像
          </button>
          <input
            ref="avatarInput"
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            class="hidden"
            @change="onAvatar"
          />
        </div>
        <label class="mt-4 block text-aux text-secondary" for="profile-nickname">昵称</label>
        <input
          id="profile-nickname"
          v-model="nickname"
          type="text"
          class="mt-1 box-border h-8 w-full border border-border rounded-panel bg-panel px-2 text-body text-text outline-none focus:border-primary"
        />
        <label class="mt-3 block text-aux text-secondary" for="profile-key">Key</label>
        <input
          id="profile-key"
          :value="store.me?.key ?? ''"
          type="text"
          readonly
          class="mt-1 box-border h-8 w-full border border-border rounded-panel bg-page px-2 text-body text-text"
        />
        <p v-if="profileError" class="mt-2 text-aux text-text">{{ profileError }}</p>
        <div class="mt-4 flex gap-2">
          <button
            type="submit"
            class="h-8 flex-1 rounded-panel bg-primary text-body text-panel active:bg-pressed disabled:opacity-60"
            :disabled="profileBusy"
          >
            保存
          </button>
          <button
            type="button"
            class="h-8 flex-1 rounded-panel border border-border bg-panel text-body text-text"
            :disabled="profileBusy"
            @click="onLogout"
          >
            退出登录
          </button>
        </div>
      </form>
    </div>

    <div
      v-if="picker"
      class="fixed inset-0 z-30 flex items-center justify-center bg-text/40 px-4"
      @click.self="picker = null"
    >
      <form
        class="flex max-h-[80vh] w-full max-w-sm flex-col border border-border rounded-panel bg-panel px-6 py-5"
        @submit.prevent="submitPicker"
      >
        <h2 class="text-body font-medium">{{ picker === 'create' ? '建群' : '添加成员' }}</h2>
        <template v-if="picker === 'create'">
          <label class="mt-4 block text-aux text-secondary" for="group-name">群名</label>
          <input
            id="group-name"
            v-model="groupName"
            type="text"
            class="mt-1 box-border h-8 w-full border border-border rounded-panel bg-panel px-2 text-body text-text outline-none focus:border-primary"
          />
        </template>
        <label class="mt-3 block text-aux text-secondary" for="user-query">搜索用户</label>
        <input
          id="user-query"
          v-model="userQuery"
          type="search"
          class="mt-1 box-border h-8 w-full border border-border rounded-panel bg-panel px-2 text-body text-text outline-none focus:border-primary"
        />
        <div v-if="selected.length" class="mt-2 flex flex-wrap gap-1">
          <button
            v-for="user in selected"
            :key="user.id"
            type="button"
            class="h-6 rounded-panel bg-page px-2 text-aux text-text"
            @click="toggleUser(user)"
          >
            {{ user.nickname }}
          </button>
        </div>
        <div class="mt-2 min-h-0 flex-1 overflow-y-auto border border-border rounded-panel">
          <p v-if="searching" class="px-2 py-3 text-aux text-secondary">加载中</p>
          <p v-else-if="userHits.length === 0" class="px-2 py-3 text-aux text-secondary">没有匹配的用户</p>
          <label
            v-for="user in userHits"
            :key="user.id"
            class="flex items-center gap-2 border-b border-border px-2 py-2"
          >
            <input
              type="checkbox"
              :checked="selectedHas(user.id)"
              @change="toggleUser(user)"
            />
            <span class="inline-flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-page text-aux text-secondary">
              <img v-if="user.avatarUrl" :src="user.avatarUrl" alt="" class="h-full w-full object-cover" />
              <template v-else>{{ initial(user.nickname) }}</template>
            </span>
            <span class="min-w-0 flex-1 truncate">{{ user.nickname }}</span>
          </label>
        </div>
        <p v-if="pickerError" class="mt-2 text-aux text-text">{{ pickerError }}</p>
        <div class="mt-4 flex gap-2">
          <button
            type="button"
            class="h-8 flex-1 rounded-panel border border-border bg-panel text-body text-text"
            @click="picker = null"
          >
            取消
          </button>
          <button
            type="submit"
            class="h-8 flex-1 rounded-panel bg-primary text-body text-panel active:bg-pressed disabled:opacity-60"
            :disabled="pickerBusy"
          >
            {{ picker === 'create' ? '创建' : '添加' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.offline-avatar {
  filter: grayscale(1);
}
</style>
