import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { plainText } from '../rich'
import {
  addMembers as apiAddMembers,
  apiErrorMessage,
  conversations as apiConversations,
  createGroup as apiCreateGroup,
  isImageMime,
  leave as apiLeave,
  logout as apiLogout,
  conversationMembers as apiMembers,
  markRead as apiMarkRead,
  me as fetchMe,
  messages as apiMessages,
  setConversationFlags as apiSetFlags,
  onlineUsers as apiOnlineUsers,
  openDirect as apiOpenDirect,
  removeMember as apiRemoveMember,
  friendList,
  patchMe,
  removeFriend as apiRemoveFriend,
  requestFriend as apiRequestFriend,
  upload,
  uploadAvatar,
  type ChatMessage,
  type Conversation,
  type Me,
  type PublicUser,
} from '../api'

const retryCapMs = 15000

export function listPreview(body: string): string {
  return plainText(body).replace(/\s+/g, ' ').trim()
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
  const token = sessionStorage.getItem('sl_session') ?? ''
  const query = token ? `?token=${encodeURIComponent(token)}` : ''
  return `${protocol}//${location.host}/api/ws${query}`
}

type ClientEvent = {
  type: 'message.send' | 'message.recall' | 'message.react'
  conversationId?: string
  kind?: string
  text?: string
  fileId?: string
  replyToId?: string
  messageId?: string
  emoji?: string
}

export const useChatStore = defineStore('chat', () => {
  const me = ref<Me | null>(null)
  const conversations = ref<Conversation[]>([])
  const onlineUsers = ref<PublicUser[]>([])
  const messages = ref<Record<string, ChatMessage[]>>({})
  const activeId = ref('')
  const pending = ref<Conversation | null>(null)
  const connected = ref(false)
  const latencyMs = ref<number | null>(null)
  const banned = ref(false)
  const notice = ref('')
  const ready = ref(false)
  const avatarNonce = ref(0)

  const active = computed(() => findConv(activeId.value))
  const activeMessages = computed(() => messages.value[activeId.value] ?? [])
  const freshFromId = ref('')
  const members = ref<PublicUser[]>([])
  const friends = ref<PublicUser[]>([])
  const incoming = ref<PublicUser[]>([])
  const outgoing = ref<string[]>([])

  let socket: WebSocket | null = null
  let stopped = true
  let started = false
  let retryMs = 1000
  let retryTimer = 0
  let pingTimer = 0
  let pingSeq = 0
  const pingSent = new Map<number, number>()
  const opened = new Set<string>()

  function findConv(id: string): Conversation | null {
    if (!id) {
      return null
    }
    return conversations.value.find((item) => item.id === id)
      ?? (pending.value?.id === id ? pending.value : null)
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
      if (a.pinned !== b.pinned) {
        return a.pinned ? -1 : 1
      }
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
    const list = await apiOnlineUsers()
    onlineUsers.value = list
    const ids = new Set(list.map((user) => user.id))
    for (const friend of friends.value) {
      friend.online = ids.has(friend.id)
    }
    for (const conv of conversations.value) {
      if (conv.peer) {
        conv.peer.online = ids.has(conv.peer.id)
      }
    }
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
    const unread = conv.unread
    const list = await apiMessages(conv.id)
    activeId.value = conv.id
    opened.add(conv.id)
    mergeMessages(conv.id, list)
    freshFromId.value = unreadMark(messages.value[conv.id] ?? [], unread)
    await readThrough(conv.id)
    members.value = []
    if (conv.type === 'group') {
      try {
        members.value = await apiMembers(conv.id)
      } catch {
        members.value = []
      }
    }
  }

  function unreadMark(list: ChatMessage[], unread: number): string {
    if (unread <= 0) {
      return ''
    }
    let left = unread
    let mark = ''
    for (let i = list.length - 1; i >= 0 && left > 0; i -= 1) {
      if (list[i].senderId !== me.value?.id) {
        left -= 1
        mark = list[i].id
      }
    }
    return mark
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
      ownerId: '',
      title: user.nickname,
      peer: { ...user, online: true },
      memberCount: 2,
      unread: 0,
      lastBody: '',
      lastAt: '',
      pinned: false,
      muted: false,
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

  function peerQuiet(conversationId: string): boolean {
    const conv = conversations.value.find((item) => item.id === conversationId)
    return conv?.type === 'direct' && conv.peer?.dnd === true
  }

  function sendText(conversationId: string, text: string, replyToId = ''): boolean {
    if (!me.value?.canSendMessage) {
      notice.value = '没有发言权限'
      return false
    }
    if (peerQuiet(conversationId)) {
      notice.value = '对方已开启免打扰'
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
      replyToId: replyToId || undefined,
    })
  }

  function recall(messageId: string): boolean {
    return sendEvent({ type: 'message.recall', messageId })
  }

  function react(messageId: string, emoji: string): boolean {
    return sendEvent({ type: 'message.react', messageId, emoji })
  }

  async function setFlags(id: string, flags: { pinned?: boolean, muted?: boolean }) {
    await apiSetFlags(id, flags)
    const conv = conversations.value.find((item) => item.id === id)
    if (!conv) {
      return
    }
    if (flags.pinned !== undefined) {
      conv.pinned = flags.pinned
    }
    if (flags.muted !== undefined) {
      conv.muted = flags.muted
    }
    sortConversations()
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
    if (peerQuiet(conversationId)) {
      notice.value = '对方已开启免打扰'
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
    await loadConversations()
    const conv = conversations.value.find((item) => item.id === id)
    if (conv) {
      await select(conv)
    }
  }

  async function addMembers(id: string, memberIds: string[]) {
    await apiAddMembers(id, memberIds)
    await loadConversations()
    if (activeId.value === id) {
      members.value = await apiMembers(id)
    }
  }

  async function removeMember(id: string, userId: string) {
    await apiRemoveMember(id, userId)
    members.value = members.value.filter((user) => user.id !== userId)
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

  function applyFriendList(list: { friends: PublicUser[], incoming: PublicUser[], outgoing?: string[] }) {
    friends.value = list.friends
    incoming.value = list.incoming
    outgoing.value = list.outgoing ?? []
  }

  async function loadFriends() {
    applyFriendList(await friendList())
  }

  async function requestFriend(userId: string) {
    applyFriendList(await apiRequestFriend(userId))
  }

  async function removeFriend(userId: string) {
    applyFriendList(await apiRemoveFriend(userId))
  }

  async function saveProfile(body: { nickname?: string, signature?: string, pendant?: string, dnd?: boolean }) {
    const updated = await patchMe(body)
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

  function applyDnd(userId: string, quiet: boolean) {
    for (const conv of conversations.value) {
      if (conv.peer?.id === userId) {
        conv.peer.dnd = quiet
      }
    }
    for (const friend of friends.value) {
      if (friend.id === userId) {
        friend.dnd = quiet
      }
    }
    for (const user of incoming.value) {
      if (user.id === userId) {
        user.dnd = quiet
      }
    }
    if (pending.value?.peer?.id === userId) {
      pending.value.peer.dnd = quiet
    }
  }

  function applyPresence(userId: string, online: boolean) {
    for (const conv of conversations.value) {
      if (conv.peer?.id === userId) {
        conv.peer.online = online
      }
    }
    for (const friend of friends.value) {
      if (friend.id === userId) {
        friend.online = online
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
      listed.lastBody = messagePreview(message)
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

  function replaceMessage(message: ChatMessage) {
    const id = message.conversationId
    const known = messages.value[id]
    if (known) {
      const index = known.findIndex((item) => item.id === message.id)
      if (index >= 0) {
        const next = known.slice()
        next[index] = message
        messages.value = { ...messages.value, [id]: next }
      }
    }
    const listed = conversations.value.find((item) => item.id === id)
    const latest = (messages.value[id] ?? []).at(-1)
    if (listed && latest?.id === message.id) {
      listed.lastBody = messagePreview(message)
      return
    }
    if (listed && message.recalled) {
      void loadConversations().catch(() => {})
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
      dnd?: boolean
      error?: string
      n?: number
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
      case 'message.updated':
        if (event.message) {
          replaceMessage(event.message)
        }
        break
      case 'presence':
        if (event.userId) {
          applyPresence(event.userId, event.online === true)
        }
        break
      case 'dnd':
        if (event.userId) {
          applyDnd(event.userId, event.dnd === true)
        }
        break
      case 'pong':
        notePong(event.n)
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
      startPing()
      void syncOpened()
    }
    ws.onmessage = (event) => {
      handleSocketData(event.data)
    }
    ws.onclose = () => {
      if (socket === ws) {
        socket = null
        connected.value = false
        stopPing()
      }
      if (stopped || socket) {
        return
      }
      const wait = retryMs
      retryMs = Math.min(retryMs * 2, retryCapMs)
      retryTimer = window.setTimeout(connect, wait)
    }
  }

  function startPing() {
    stopPing()
    latencyMs.value = null
    sendPing()
    pingTimer = window.setInterval(sendPing, 3000)
  }

  function stopPing() {
    window.clearInterval(pingTimer)
    pingTimer = 0
    pingSent.clear()
    latencyMs.value = null
  }

  function sendPing() {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return
    }
    pingSeq = (pingSeq + 1) % 1_000_000
    const n = pingSeq
    pingSent.set(n, performance.now())
    while (pingSent.size > 4) {
      const oldest = pingSent.keys().next().value
      if (oldest == null) {
        break
      }
      pingSent.delete(oldest)
    }
    socket.send(JSON.stringify({ type: 'ping', n }))
  }

  function notePong(n: number | undefined) {
    if (typeof n !== 'number') {
      return
    }
    const sent = pingSent.get(n)
    if (sent == null) {
      return
    }
    pingSent.delete(n)
    const rtt = Math.max(0, performance.now() - sent)
    latencyMs.value = Math.round(rtt * 10) / 10
  }

  function stopSocket() {
    stopped = true
    window.clearTimeout(retryTimer)
    stopPing()
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
    try {
      await Promise.all([loadConversations(), loadOnline(), loadFriends()])
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
    friends.value = []
    incoming.value = []
    outgoing.value = []
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
    freshFromId,
    members,
    friends,
    incoming,
    outgoing,
    connected,
    latencyMs,
    banned,
    notice,
    ready,
    avatarNonce,
    bootstrap,
    stop,
    select,
    openDirect,
    sendText,
    recall,
    react,
    setFlags,
    sendFiles,
    createGroup,
    addMembers,
    removeMember,
    leave,
    loadFriends,
    loadOnline,
    requestFriend,
    removeFriend,
    saveProfile,
    saveAvatar,
    logout,
  }
})

function messagePreview(message: ChatMessage): string {
  if (message.recalled) {
    return '撤回了一条消息'
  }
  return message.fileName || listPreview(message.body)
}

function stampValue(iso: string): number {
  if (!iso || iso.startsWith('0001-')) {
    return 0
  }
  const value = Date.parse(iso)
  return Number.isFinite(value) ? value : 0
}
