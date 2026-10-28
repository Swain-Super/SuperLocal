import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  addMembers as apiAddMembers,
  apiErrorMessage,
  conversations as apiConversations,
  createGroup as apiCreateGroup,
  isImageMime,
  leave as apiLeave,
  logout as apiLogout,
  markRead as apiMarkRead,
  me as fetchMe,
  messages as apiMessages,
  onlineUsers as apiOnlineUsers,
  openDirect as apiOpenDirect,
  patchMe,
  upload,
  uploadAvatar,
  type ChatMessage,
  type Conversation,
  type Me,
  type PublicUser,
} from '../api'

const retryCapMs = 15000

// 文件消息的 body 只带文件名和大小，方便对方画出文件横条。文字消息不会走这个格式。
export function fileBody(name: string, size: number): string {
  return JSON.stringify({ name, size })
}

export function fileMeta(body: string): { name: string; size: number | null } | null {
  if (!body.startsWith('{')) {
    return null
  }
  try {
    const parsed: unknown = JSON.parse(body)
    if (!parsed || typeof parsed !== 'object') {
      return null
    }
    const record = parsed as { name?: unknown; size?: unknown }
    if (typeof record.name !== 'string' || record.name === '') {
      return null
    }
    const size = typeof record.size === 'number' && Number.isFinite(record.size) ? record.size : null
    return { name: record.name, size }
  } catch {
    return null
  }
}

export function listPreview(body: string): string {
  return fileMeta(body)?.name ?? body
}

export function formatBytes(size: number): string {
  if (size < 1024) {
    return `${size} B`
  }
  const kb = size / 1024
  if (kb < 1024) {
    return `${trim1(kb)} KB`
  }
  const mb = kb / 1024
  if (mb < 1024) {
    return `${trim1(mb)} MB`
  }
  return `${trim1(mb / 1024)} GB`
}

function trim1(value: number): string {
  const text = value.toFixed(1)
  return text.endsWith('.0') ? text.slice(0, -2) : text
}

export function formatStamp(iso: string): string {
  if (!iso || iso.startsWith('0001-')) {
    return ''
  }
  const date = new Date(iso)
  if (Number.isNaN(date.getTime()) || date.getFullYear() < 1970) {
    return ''
  }
  const pad = (n: number) => String(n).padStart(2, '0')
  const hm = `${pad(date.getHours())}:${pad(date.getMinutes())}`
  const now = new Date()
  const sameDay = date.getFullYear() === now.getFullYear()
    && date.getMonth() === now.getMonth()
    && date.getDate() === now.getDate()
  if (sameDay) {
    return hm
  }
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${hm}`
}

export function runeCount(value: string): number {
  return Array.from(value).length
}

function wsURL(): string {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${location.host}/api/ws`
}

function ownedKey(userId: string): string {
  return `sl-owned-groups:${userId}`
}

type ClientEvent = {
  type: 'message.send'
  conversationId: string
  kind: string
  text?: string
  fileId?: string
}

export const useChatStore = defineStore('chat', () => {
  const me = ref<Me | null>(null)
  const conversations = ref<Conversation[]>([])
  const onlineUsers = ref<PublicUser[]>([])
  const messages = ref<Record<string, ChatMessage[]>>({})
  const activeId = ref('')
  const pending = ref<Conversation | null>(null)
  const connected = ref(false)
  const banned = ref(false)
  const notice = ref('')
  const ready = ref(false)
  const avatarNonce = ref(0)
  // 会话 JSON 没有群主字段。本机记下自己创建的群，用来显示「添加成员」。
  const owned = ref<Set<string>>(new Set())

  const active = computed(() => findConv(activeId.value))
  const activeMessages = computed(() => messages.value[activeId.value] ?? [])

  let socket: WebSocket | null = null
  let stopped = true
  let started = false
  let retryMs = 1000
  let retryTimer = 0
  const opened = new Set<string>()

  function findConv(id: string): Conversation | null {
    if (!id) {
      return null
    }
    return conversations.value.find((item) => item.id === id)
      ?? (pending.value?.id === id ? pending.value : null)
  }

  function loadOwned() {
    const userId = me.value?.id
    if (!userId) {
      owned.value = new Set()
      return
    }
    try {
      const raw = localStorage.getItem(ownedKey(userId))
      const ids = raw ? JSON.parse(raw) as unknown : []
      owned.value = new Set(Array.isArray(ids) ? ids.filter((id) => typeof id === 'string') : [])
    } catch {
      owned.value = new Set()
    }
  }

  function rememberOwned(id: string) {
    const next = new Set(owned.value)
    next.add(id)
    owned.value = next
    if (me.value) {
      localStorage.setItem(ownedKey(me.value.id), JSON.stringify([...next]))
    }
  }

  function canAddMembers(id: string): boolean {
    const conv = conversations.value.find((item) => item.id === id)
    return conv?.type === 'group' && owned.value.has(id)
  }

  function mergeMessages(id: string, incoming: ChatMessage[]) {
    const map = new Map<string, ChatMessage>()
    for (const item of messages.value[id] ?? []) {
      map.set(item.id, item)
    }
    for (const item of incoming) {
      map.set(item.id, item)
    }
    const list = [...map.values()].sort((a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0))
    messages.value = { ...messages.value, [id]: list }
  }

  function sortConversations() {
    conversations.value = [...conversations.value].sort((a, b) => {
      const aTime = stampValue(a.lastAt)
      const bTime = stampValue(b.lastAt)
      if (aTime !== bTime) {
        return bTime - aTime
      }
      return a.id < b.id ? 1 : a.id > b.id ? -1 : 0
    })
  }

  async function loadConversations() {
    const list = await apiConversations()
    conversations.value = list
    if (pending.value && list.some((item) => item.id === pending.value?.id)) {
      pending.value = null
    }
    if (activeId.value && !findConv(activeId.value)) {
      activeId.value = ''
    }
  }

  async function loadOnline() {
    onlineUsers.value = await apiOnlineUsers()
  }

  async function readThrough(id: string) {
    if (activeId.value !== id) {
      return
    }
    const list = messages.value[id] ?? []
    const last = list[list.length - 1]
    if (!last) {
      return
    }
    try {
      await apiMarkRead(id, last.id)
    } catch {
      return
    }
    const conv = conversations.value.find((item) => item.id === id)
    if (conv) {
      conv.unread = 0
    }
  }

  async function select(conv: Conversation) {
    const list = await apiMessages(conv.id)
    activeId.value = conv.id
    opened.add(conv.id)
    mergeMessages(conv.id, list)
    await readThrough(conv.id)
  }

  async function openDirect(user: PublicUser) {
    const id = await apiOpenDirect(user.id)
    const existing = conversations.value.find((item) => item.id === id)
    if (existing) {
      pending.value = null
      await select(existing)
      return
    }
    const conv: Conversation = {
      id,
      type: 'direct',
      title: user.nickname,
      peer: { ...user, online: true },
      memberCount: 2,
      unread: 0,
      lastBody: '',
      lastAt: '',
    }
    pending.value = conv
    await select(conv)
  }

  function sendEvent(payload: ClientEvent): boolean {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      notice.value = '连接已断开'
      return false
    }
    socket.send(JSON.stringify(payload))
    notice.value = ''
    return true
  }

  function sendText(conversationId: string, text: string): boolean {
    if (!me.value?.canSendMessage) {
      notice.value = '没有发言权限'
      return false
    }
    if (runeCount(text) > 4000) {
      notice.value = '文字最长 4000 字'
      return false
    }
    if (text.trim() === '') {
      return false
    }
    return sendEvent({
      type: 'message.send',
      conversationId,
      kind: 'text',
      text,
    })
  }

  async function sendFiles(conversationId: string, files: File[]) {
    if (files.length === 0) {
      return
    }
    if (!me.value?.canSendMessage) {
      notice.value = '没有发言权限'
      return
    }
    if (!me.value.canSendFile) {
      notice.value = '没有发文件权限'
      return
    }
    for (const file of files) {
      try {
        const uploaded = await upload(file)
        const kind = isImageMime(uploaded.mime) ? 'image' : 'file'
        const ok = sendEvent({
          type: 'message.send',
          conversationId,
          kind,
          fileId: uploaded.id,
          text: fileBody(uploaded.name, uploaded.size),
        })
        if (!ok) {
          return
        }
      } catch (err) {
        notice.value = apiErrorMessage(err)
        return
      }
    }
  }

  async function createGroup(name: string, memberIds: string[]) {
    const id = await apiCreateGroup(name, memberIds)
    rememberOwned(id)
    await loadConversations()
    const conv = conversations.value.find((item) => item.id === id)
    if (conv) {
      await select(conv)
    }
  }

  async function addMembers(id: string, memberIds: string[]) {
    await apiAddMembers(id, memberIds)
    await loadConversations()
  }

  async function leave(id: string) {
    await apiLeave(id)
    opened.delete(id)
    const next = { ...messages.value }
    delete next[id]
    messages.value = next
    if (activeId.value === id) {
      activeId.value = ''
      pending.value = null
    }
    await loadConversations()
  }

  async function saveNickname(nickname: string) {
    const updated = await patchMe(nickname)
    me.value = updated
    const userId = updated.id
    for (const list of Object.values(messages.value)) {
      for (const message of list) {
        if (message.senderId === userId) {
          message.senderNickname = updated.nickname
        }
      }
    }
  }

  async function saveAvatar(file: File) {
    me.value = await uploadAvatar(file)
    avatarNonce.value += 1
  }

  function applyPresence(userId: string, online: boolean) {
    for (const conv of conversations.value) {
      if (conv.peer?.id === userId) {
        conv.peer.online = online
      }
    }
    if (pending.value?.peer?.id === userId) {
      pending.value.peer.online = online
    }
    if (online) {
      void loadOnline().catch(() => {})
      return
    }
    onlineUsers.value = onlineUsers.value.filter((user) => user.id !== userId)
  }

  function onMessageNew(message: ChatMessage) {
    const id = message.conversationId
    const known = messages.value[id] ?? []
    const duplicate = known.some((item) => item.id === message.id)
    if (!duplicate && (opened.has(id) || activeId.value === id)) {
      mergeMessages(id, [message])
    }
    const listed = conversations.value.find((item) => item.id === id)
    if (!listed) {
      void loadConversations().then(() => {
        if (activeId.value !== id) {
          return
        }
        const current = conversations.value.find((item) => item.id === id)
        if (current) {
          current.unread = 0
        }
      }).catch(() => {})
    } else {
      listed.lastBody = message.body
      listed.lastAt = message.createdAt
      if (activeId.value === id) {
        listed.unread = 0
      } else if (!duplicate && message.senderId !== me.value?.id) {
        listed.unread += 1
      }
      sortConversations()
    }
    if (activeId.value === id) {
      void readThrough(id).catch(() => {})
    }
  }

  function handleSocketData(data: unknown) {
    if (typeof data !== 'string') {
      return
    }
    let event: {
      type?: string
      message?: ChatMessage
      userId?: string
      online?: boolean
      error?: string
    }
    try {
      event = JSON.parse(data) as typeof event
    } catch {
      return
    }
    switch (event.type) {
      case 'message.new':
        if (event.message) {
          onMessageNew(event.message)
        }
        break
      case 'presence':
        if (event.userId) {
          applyPresence(event.userId, event.online === true)
        }
        break
      case 'error':
        notice.value = event.error || '发送失败'
        break
      case 'session.revoked':
        ban()
        break
      default:
        break
    }
  }

  async function syncOpened() {
    const ids = [...opened]
    await Promise.all(ids.map(async (id) => {
      const list = messages.value[id] ?? []
      const after = list.length > 0 ? list[list.length - 1].id : undefined
      try {
        const incoming = await apiMessages(id, after)
        if (incoming.length > 0) {
          mergeMessages(id, incoming)
        }
      } catch {
        // 下次重连再补。
      }
    }))
    try {
      await loadConversations()
      await loadOnline()
    } catch {
      // 列表留待下一次重连。
    }
    if (activeId.value && opened.has(activeId.value)) {
      try {
        await readThrough(activeId.value)
      } catch {
        // 未读下次打开再清。
      }
    }
  }

  function connect() {
    if (stopped) {
      return
    }
    if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
      return
    }
    const ws = new WebSocket(wsURL())
    socket = ws
    ws.onopen = () => {
      if (socket !== ws) {
        return
      }
      connected.value = true
      retryMs = 1000
      void syncOpened()
    }
    ws.onmessage = (event) => {
      handleSocketData(event.data)
    }
    ws.onclose = () => {
      if (socket === ws) {
        socket = null
        connected.value = false
      }
      if (stopped || socket) {
        return
      }
      const wait = retryMs
      retryMs = Math.min(retryMs * 2, retryCapMs)
      retryTimer = window.setTimeout(connect, wait)
    }
  }

  function stopSocket() {
    stopped = true
    window.clearTimeout(retryTimer)
    connected.value = false
    started = false
    const current = socket
    socket = null
    current?.close()
  }

  function ban() {
    stopSocket()
    banned.value = true
    notice.value = ''
  }

  async function bootstrap() {
    if (started) {
      return
    }
    started = true
    stopped = false
    banned.value = false
    try {
      me.value = await fetchMe()
    } catch (err) {
      started = false
      ready.value = true
      if (apiErrorMessage(err) === '账号已被封禁') {
        ban()
        return
      }
      notice.value = apiErrorMessage(err)
      return
    }
    if (stopped) {
      return
    }
    loadOwned()
    try {
      await Promise.all([loadConversations(), loadOnline()])
    } catch (err) {
      notice.value = apiErrorMessage(err)
    }
    ready.value = true
    if (!stopped) {
      connect()
    }
  }

  async function logout() {
    await apiLogout()
    stopSocket()
    me.value = null
    conversations.value = []
    onlineUsers.value = []
    messages.value = {}
    activeId.value = ''
    pending.value = null
    opened.clear()
    ready.value = false
  }

  function stop() {
    stopSocket()
  }

  return {
    me,
    conversations,
    onlineUsers,
    messages,
    active,
    activeId,
    activeMessages,
    connected,
    banned,
    notice,
    ready,
    avatarNonce,
    bootstrap,
    stop,
    select,
    openDirect,
    sendText,
    sendFiles,
    createGroup,
    addMembers,
    leave,
    saveNickname,
    saveAvatar,
    logout,
    canAddMembers,
  }
})

function stampValue(iso: string): number {
  if (!iso || iso.startsWith('0001-')) {
    return 0
  }
  const value = Date.parse(iso)
  return Number.isFinite(value) ? value : 0
}
