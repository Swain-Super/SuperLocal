<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  apiErrorMessage,
  isImageMime,
  searchUsers,
  withSession,
  type ChatMessage,
  type Conversation,
  type PublicUser,
} from '../api'
import {
  fontStacks,
  isShake,
  plainText,
  richBlocks,
  richColors,
  richFonts,
  richMarks,
  richSizes,
  serializeEditor,
  type RichStyle,
} from '../rich'
import {
  formatBytes,
  formatStamp,
  listPreview,
  runeCount,
  useChatStore,
} from '../stores/chat'

const store = useChatStore()
const router = useRouter()

const latencyText = computed(() => {
  if (!store.connected) {
    return '未连接'
  }
  if (store.latencyMs == null) {
    return '测量中'
  }
  if (store.latencyMs < 0.1) {
    return '<0.1 ms'
  }
  if (store.latencyMs < 10) {
    return `${store.latencyMs.toFixed(1)} ms`
  }
  return `${Math.round(store.latencyMs)} ms`
})

const latencyLevel = computed(() => {
  if (!store.connected || store.latencyMs == null) {
    return 'wait'
  }
  if (store.latencyMs <= 80) {
    return 'good'
  }
  if (store.latencyMs <= 200) {
    return 'ok'
  }
  return 'slow'
})

const pane = ref<'messages' | 'contacts'>('messages')
const keyword = ref('')
const showThread = ref(false)
const narrow = ref(false)
const draft = ref('')
const composing = ref(false)
const profileOpen = ref(false)
const helpOpen = ref(false)
const nickname = ref('')
const signature = ref('')
const pendant = ref('')
const dnd = ref(false)
const notifyOn = ref(localStorage.getItem('sl_notify') !== '0')
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

const quickReactions = [
  { emoji: 'thumb', label: '👍' },
  { emoji: 'ok', label: 'OK' },
  { emoji: 'plus', label: '+1' },
  { emoji: 'done', label: '✓' },
  { emoji: 'get', label: 'Get' },
  { emoji: 'smile', label: '😄' },
]
const composeEmoji = [
  '😀', '😁', '😂', '🤣', '😊', '😍', '😘', '😜', '😎', '🤔',
  '😅', '😭', '😤', '😱', '😴', '🤮', '👍', '👎', '👏', '🙏',
  '💪', '👋', '❤️', '💔', '🌹', '🎉', '🔥', '⭐', '💯', '✅',
  '☕', '🍻', '🌙', '☀️', '👀', '❓', '❗', '🎁',
]

const replyTo = ref<ChatMessage | null>(null)
const composer = ref<HTMLDivElement | null>(null)
const editorEmpty = ref(true)
const fontId = ref<(typeof richFonts)[number]['id']>('yahei')
const ink = ref('#1F2329')
const hoverInk = ref('')
const dropperOpen = ref(false)
const dropperHex = ref('#1F2329')
const dropperVideo = ref<HTMLVideoElement | null>(null)
const loupe = ref({ x: 0, y: 0, on: false })
let dropperStream: MediaStream | null = null
const membersOpen = ref(false)
const memberMenu = ref<{ x: number, y: number, id: string } | null>(null)
const fontPx = ref(14)
const storedComposerHeight = Number(localStorage.getItem('sl_composer_h') || '')
const composerHeight = ref(storedComposerHeight >= 96 && storedComposerHeight <= 720 ? storedComposerHeight : 0)
const colorOpen = ref(false)
const markOpen = ref(false)
const fmtState = ref({ bold: false, italic: false, underline: false, strike: false, align: 'left' })
const ctrlEnterSend = ref(localStorage.getItem('sl_send_mode') === 'ctrl')
const shaking = ref(false)
const historyOpen = ref(false)
const historyQuery = ref('')
const copiedKey = ref('')
const shotUrl = ref('')
const shotFile = ref<File | null>(null)
const contactHits = ref<PublicUser[]>([])
const contactSearching = ref(false)
const pendants = [
  { id: '', label: '无' },
  { id: 'ring', label: '圆环' },
  { id: 'star', label: '星标' },
  { id: 'leaf', label: '新叶' },
  { id: 'crown', label: '皇冠' },
]
const emojiOpen = ref(false)
const formatOpen = ref(false)
const shotMenu = ref(false)
const plusMenu = ref(false)
const sendMenu = ref(false)
const composerExpanded = ref(false)
const recording = ref(false)
const scrollingShot = ref(false)
const recentEmoji = ref<string[]>(readRecentEmoji())
let savedRange: Range | null = null
let shakeTimer = 0
const mentionOpen = ref(false)
const mentionQuery = ref('')
const convMenu = ref<{ x: number, y: number, id: string } | null>(null)
const moreId = ref('')
const forwardMsg = ref<ChatMessage | null>(null)
const previewUrl = ref('')

const scroller = ref<HTMLElement | null>(null)
const imageInput = ref<HTMLInputElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const avatarInput = ref<HTMLInputElement | null>(null)

let compositionTimer = 0
let searchTimer = 0
let contactTimer = 0
let removeMedia = () => {}

const filteredConversations = computed(() => {
  const q = keyword.value.trim()
  if (!q) {
    return store.conversations
  }
  return store.conversations.filter((item) => item.title.includes(q))
})

const onlineContacts = computed(() => {
  const selfId = store.me?.id
  const friendIds = new Set(store.friends.map((user) => user.id))
  return store.onlineUsers
    .filter((user) => user.id !== selfId && !friendIds.has(user.id))
    .slice()
    .sort((a, b) => a.nickname.localeCompare(b.nickname, 'zh'))
})

const mentionHits = computed(() => {
  const q = mentionQuery.value.trim()
  return store.members.filter((user) => user.id !== store.me?.id && (!q || user.nickname.includes(q)))
})

function reactionLabel(emoji: string): string {
  return quickReactions.find((item) => item.emoji === emoji)?.label ?? emoji
}

function quoteText(msg: { kind: string, body: string, fileName?: string, recalled: boolean }): string {
  if (msg.recalled) {
    return '消息已撤回'
  }
  if (msg.kind === 'image') {
    return '[图片]'
  }
  if (msg.kind === 'file') {
    return `[文件] ${msg.fileName || ''}`.trim()
  }
  if (isShake(msg.body)) {
    return '抖动了窗口'
  }
  const blocks = messageBlocks(msg.body)
  const codes = blocks.filter((block) => block.type === 'code')
  if (codes.length === 0) {
    return plainText(msg.body)
  }
  const plain = blocks
    .filter((block) => block.type === 'text')
    .map((block) => plainText(block.text).trim())
    .filter((text) => text !== '')
    .join(' ')
  return plain ? `${plain} [代码]` : '[代码]'
}

type TextBlock = { type: 'text', text: string }
type CodeBlock = { type: 'code', lang: string, text: string }

function messageBlocks(body: string): Array<TextBlock | CodeBlock> {
  const blocks: Array<TextBlock | CodeBlock> = []
  const re = /```([^\n`]*)\n?([\s\S]*?)```/g
  let last = 0
  for (const match of body.matchAll(re)) {
    const index = match.index ?? 0
    if (index > last) {
      blocks.push({ type: 'text', text: body.slice(last, index) })
    }
    const rawLang = (match[1] ?? '').trim()
    const langOk = rawLang === '' || /^[\w#+.-]{1,32}$/.test(rawLang)
    const codeText = (match[2] ?? '').replace(/\n$/, '')
    blocks.push({
      type: 'code',
      lang: langOk ? rawLang : '',
      text: langOk || rawLang === '' ? codeText : `${rawLang}\n${codeText}`,
    })
    last = index + match[0].length
  }
  if (last < body.length) {
    blocks.push({ type: 'text', text: body.slice(last) })
  }
  if (blocks.length === 0) {
    blocks.push({ type: 'text', text: body })
  }
  return blocks
}

function textParts(body: string): { text: string, mention: boolean }[] {
  const parts: { text: string, mention: boolean }[] = []
  const re = /@[^\s@]+/g
  let last = 0
  for (const match of body.matchAll(re)) {
    const index = match.index ?? 0
    if (index > last) {
      parts.push({ text: body.slice(last, index), mention: false })
    }
    parts.push({ text: match[0], mention: true })
    last = index + match[0].length
  }
  if (last < body.length) {
    parts.push({ text: body.slice(last), mention: false })
  }
  if (parts.length === 0) {
    parts.push({ text: body, mention: false })
  }
  return parts
}

function recallLine(msg: ChatMessage): string {
  if (msg.senderId === store.me?.id) {
    return '你撤回了一条消息'
  }
  return `${msg.senderNickname} 撤回了一条消息`
}

function canRecall(msg: ChatMessage): boolean {
  if (msg.senderId !== store.me?.id || msg.recalled) {
    return false
  }
  const at = Date.parse(msg.createdAt)
  return Number.isFinite(at) && Date.now() - at < 24 * 60 * 60 * 1000
}

const canSendMessage = computed(() => store.me?.canSendMessage === true)
const canSendFile = computed(() => store.me?.canSendFile === true)
const peerQuiet = computed(() => store.active?.type === 'direct' && store.active.peer?.dnd === true)
const shownInk = computed(() => hoverInk.value || (dropperOpen.value ? dropperHex.value : ink.value))
const dropperRgb = computed(() => hexToRgb(dropperHex.value))
const canCompose = computed(() => canSendMessage.value && store.active != null && !peerQuiet.value)

const historyHits = computed(() => {
  const q = historyQuery.value.trim()
  return store.activeMessages.filter((msg) => {
    if (msg.recalled) {
      return false
    }
    const text = msg.kind === 'text' ? plainText(msg.body) : (msg.fileName || msg.kind)
    return q === '' || text.includes(q) || msg.senderNickname.includes(q)
  })
})

function isFriend(id: string): boolean {
  return store.friends.some((user) => user.id === id)
}

function isOutgoing(id: string): boolean {
  return store.outgoing.includes(id)
}

function contactLine(user: PublicUser): string {
  if (user.dnd) {
    return '免打扰'
  }
  const signature = user.signature?.trim()
  if (signature) {
    return signature
  }
  return user.online ? '在线' : '离线'
}

function pendantClass(name?: string): string {
  if (name === 'ring' || name === 'star' || name === 'leaf' || name === 'crown') {
    return `pendant-${name}`
  }
  return ''
}

function codeLines(text: string): string[] {
  if (text === '') {
    return ['']
  }
  return text.split('\n')
}

const editorPlaceholder = computed(() => {
  if (peerQuiet.value) {
    return '对方已开启免打扰'
  }
  if (!canSendMessage.value) {
    return '没有发言权限'
  }
  if (store.active) {
    return `发送给 ${store.active.title}`
  }
  return '选择一个会话'
})

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
  return withSession(`${url}${joiner}v=${store.avatarNonce}`)
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
    resetEditor()
    replyTo.value = null
    mentionOpen.value = false
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
    resetEditor()
    await stickToBottom(true)
  } catch (err) {
    store.notice = apiErrorMessage(err)
  }
}

function submitText() {
  const conv = store.active
  const el = composer.value
  if (!conv || !el) {
    return
  }
  const text = serializeEditor(el)
  if (store.sendText(conv.id, text, replyTo.value?.id ?? '')) {
    resetEditor()
    replyTo.value = null
    mentionOpen.value = false
  }
}

function resetEditor() {
  draft.value = ''
  editorEmpty.value = true
  if (composer.value) {
    composer.value.innerHTML = ''
  }
}

function shakeLine(msg: ChatMessage): string {
  if (msg.senderId === store.me?.id) {
    return '你抖动了窗口'
  }
  return `${msg.senderNickname} 抖动了窗口`
}

function runStyle(style: RichStyle): Record<string, string> | undefined {
  const css: Record<string, string> = {}
  if (style.bold) {
    css.fontWeight = '700'
  }
  if (style.italic) {
    css.fontStyle = 'italic'
  }
  const deco = [style.underline ? 'underline' : '', style.strike ? 'line-through' : ''].filter(Boolean)
  if (deco.length > 0) {
    css.textDecoration = deco.join(' ')
  }
  if (style.color) {
    css.color = style.color
  }
  if (style.highlight) {
    css.backgroundColor = style.highlight
    css.borderRadius = '2px'
  }
  if (style.size) {
    css.fontSize = `${style.size}px`
  }
  if (style.font) {
    css.fontFamily = fontStacks[style.font]
  }
  return Object.keys(css).length > 0 ? css : undefined
}

function playShake() {
  shaking.value = false
  window.clearTimeout(shakeTimer)
  void nextTick(() => {
    shaking.value = true
    shakeTimer = window.setTimeout(() => {
      shaking.value = false
    }, 450)
  })
}

function sendShake() {
  const conv = store.active
  if (!conv || peerQuiet.value) {
    return
  }
  if (store.sendText(conv.id, '[[shake]]')) {
    playShake()
  }
}

function syncEditor() {
  const el = composer.value
  if (!el) {
    return
  }
  refreshCodeGutters()
  draft.value = serializeEditor(el)
  editorEmpty.value = plainText(draft.value).trim() === ''
  const before = textBeforeCaret(el)
  if (store.active?.type !== 'group') {
    mentionOpen.value = false
    return
  }
  const match = before.match(/@([^\s@]*)$/)
  if (!match) {
    mentionOpen.value = false
    return
  }
  mentionOpen.value = true
  mentionQuery.value = match[1] ?? ''
}

function textBeforeCaret(root: HTMLElement): string {
  const selection = window.getSelection()
  if (!selection || selection.rangeCount === 0 || !selection.anchorNode || !root.contains(selection.anchorNode)) {
    return ''
  }
  const range = selection.getRangeAt(0).cloneRange()
  range.selectNodeContents(root)
  range.setEnd(selection.anchorNode, selection.anchorOffset)
  return range.toString()
}

function saveRange() {
  const selection = window.getSelection()
  if (!selection || selection.rangeCount === 0 || !composer.value?.contains(selection.anchorNode)) {
    savedRange = null
    return
  }
  savedRange = selection.getRangeAt(0).cloneRange()
}

function restoreRange() {
  const selection = window.getSelection()
  composer.value?.focus()
  if (!savedRange || !selection) {
    return
  }
  selection.removeAllRanges()
  selection.addRange(savedRange)
}

function hexToRgb(hex: string): string {
  const raw = hex.replace('#', '')
  if (raw.length !== 6) {
    return hex
  }
  const r = Number.parseInt(raw.slice(0, 2), 16)
  const g = Number.parseInt(raw.slice(2, 4), 16)
  const b = Number.parseInt(raw.slice(4, 6), 16)
  if ([r, g, b].some((n) => Number.isNaN(n))) {
    return hex
  }
  return `rgb(${r}, ${g}, ${b})`
}

function applyInk(color: string) {
  ink.value = color.toUpperCase()
  hoverInk.value = ''
  colorOpen.value = false
  formatCommand('foreColor', ink.value)
}

function formatCommand(command: string, value?: string) {
  if (!canCompose.value) {
    return
  }
  const selection = window.getSelection()
  if (!composer.value?.contains(selection?.anchorNode ?? null)) {
    restoreRange()
  } else {
    composer.value.focus()
  }
  document.execCommand(command, false, value)
  syncEditor()
  saveRange()
}

function startComposerResize(event: PointerEvent) {
  const handle = event.currentTarget
  const form = handle instanceof HTMLElement ? handle.closest('.composer') : null
  if (!(handle instanceof HTMLElement) || !(form instanceof HTMLElement)) {
    return
  }
  event.preventDefault()
  handle.setPointerCapture(event.pointerId)
  const origin = { y: event.clientY, h: form.getBoundingClientRect().height }
  const move = (next: PointerEvent) => {
    const min = window.innerHeight <= 520 ? 96 : 150
    const max = Math.max(min + 40, Math.min(window.innerHeight * 0.62, window.innerHeight - 140))
    composerHeight.value = Math.round(Math.min(max, Math.max(min, origin.h + origin.y - next.clientY)))
  }
  const end = (next: PointerEvent) => {
    if (handle.hasPointerCapture(next.pointerId)) {
      handle.releasePointerCapture(next.pointerId)
    }
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', end)
    window.removeEventListener('pointercancel', end)
    localStorage.setItem('sl_composer_h', String(composerHeight.value))
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', end)
  window.addEventListener('pointercancel', end)
}

function applyFont() {
  const face = richFonts.find((item) => item.id === fontId.value)?.face
  if (face) {
    formatCommand('fontName', face)
  }
}

function applySize() {
  const command = richSizes.find((item) => item.px === Number(fontPx.value))?.command
  if (command) {
    formatCommand('fontSize', command)
  }
}

function applyMark(color: string) {
  markOpen.value = false
  formatCommand('hiliteColor', color)
}

function clearFormat() {
  colorOpen.value = false
  markOpen.value = false
  formatCommand('removeFormat')
  formatCommand('hiliteColor', 'transparent')
  formatCommand('justifyLeft')
  fmtState.value = { bold: false, italic: false, underline: false, strike: false, align: 'left' }
}

function refreshFmt() {
  const selection = window.getSelection()
  if (!selection || !composer.value?.contains(selection.anchorNode)) {
    return
  }
  try {
    fmtState.value = {
      bold: document.queryCommandState('bold'),
      italic: document.queryCommandState('italic'),
      underline: document.queryCommandState('underline'),
      strike: document.queryCommandState('strikeThrough'),
      align: document.queryCommandState('justifyCenter') ? 'center' : document.queryCommandState('justifyRight') ? 'right' : 'left',
    }
  } catch {
    // 选区不在编辑器里时，浏览器会拒绝查询格式。
  }
}

function toggleColor() {
  if (!canCompose.value) {
    return
  }
  markOpen.value = false
  colorOpen.value = !colorOpen.value
}

function toggleMark() {
  if (!canCompose.value) {
    return
  }
  colorOpen.value = false
  markOpen.value = !markOpen.value
}

function startReply(msg: ChatMessage) {
  replyTo.value = msg
  moreId.value = ''
  composer.value?.focus()
}

async function copyMessage(msg: ChatMessage) {
  moreId.value = ''
  try {
    await navigator.clipboard.writeText(msg.kind === 'text' ? plainText(msg.body) : quoteText(msg))
  } catch {
    store.notice = '复制失败'
  }
}

function doRecall(msg: ChatMessage) {
  moreId.value = ''
  store.recall(msg.id)
}

function doReact(msg: ChatMessage, emoji: string) {
  store.react(msg.id, emoji)
}

function toggleMore(id: string) {
  moreId.value = moreId.value === id ? '' : id
}

function openForward(msg: ChatMessage) {
  moreId.value = ''
  forwardMsg.value = msg
}

function sendForward(conv: Conversation) {
  const msg = forwardMsg.value
  if (!msg) {
    return
  }
  if (store.sendText(conv.id, msg.kind === 'text' ? msg.body : quoteText(msg))) {
    forwardMsg.value = null
  }
}

function onConvMenu(event: MouseEvent, conv: Conversation) {
  memberMenu.value = null
  convMenu.value = { x: event.clientX, y: event.clientY, id: conv.id }
}

function menuConv(): Conversation | undefined {
  return store.conversations.find((item) => item.id === convMenu.value?.id)
}

async function togglePin() {
  const conv = menuConv()
  convMenu.value = null
  if (!conv) {
    return
  }
  try {
    await store.setFlags(conv.id, { pinned: !conv.pinned })
  } catch (err) {
    store.notice = apiErrorMessage(err)
  }
}

async function toggleMute() {
  const conv = menuConv()
  convMenu.value = null
  if (!conv) {
    return
  }
  try {
    await store.setFlags(conv.id, { muted: !conv.muted })
  } catch (err) {
    store.notice = apiErrorMessage(err)
  }
}

function pickMention(user: PublicUser) {
  const el = composer.value
  if (!el) {
    return
  }
  el.focus()
  const before = textBeforeCaret(el)
  const match = before.match(/@([^\s@]*)$/)
  const remove = match ? match[0].length : 0
  for (let i = 0; i < remove; i += 1) {
    document.execCommand('delete')
  }
  document.execCommand('insertText', false, `@${user.nickname} `)
  mentionOpen.value = false
  syncEditor()
}

function insertCodeBlock() {
  const el = composer.value
  if (!el) {
    return
  }
  el.focus()
  restoreRange()
  const selected = window.getSelection()?.toString() ?? ''
  const card = document.createElement('div')
  card.className = 'draft-code'
  card.innerHTML = `<div class="draft-code-bar" contenteditable="false"><select class="draft-lang" aria-label="语言"><option value="">无标题</option><option value="Go">Go</option><option value="JavaScript">JavaScript</option><option value="TypeScript">TypeScript</option><option value="Python">Python</option><option value="Java">Java</option><option value="SQL">SQL</option><option value="Shell">Shell</option></select><span class="draft-spacer"></span><button type="button" class="draft-wrap">取消换行</button><button type="button" class="draft-copy">复制</button></div><div class="draft-code-scroll" contenteditable="false"><ol class="draft-gutter"></ol><pre class="draft-code-body" contenteditable="true"></pre></div>`
  const body = card.querySelector('.draft-code-body')
  if (body) {
    body.textContent = selected
  }
  const selection = window.getSelection()
  if (!selection || selection.rangeCount === 0) {
    el.appendChild(card)
  } else {
    const range = selection.getRangeAt(0)
    range.deleteContents()
    range.insertNode(card)
  }
  const caret = document.createRange()
  const target = card.querySelector('.draft-code-body')
  if (target && selection) {
    caret.selectNodeContents(target)
    caret.collapse(false)
    selection.removeAllRanges()
    selection.addRange(caret)
  }
  composerExpanded.value = true
  refreshCodeGutters()
  syncEditor()
}

function refreshCodeGutters() {
  const root = composer.value
  if (!root) {
    return
  }
  for (const card of root.querySelectorAll('.draft-code')) {
    const body = card.querySelector('.draft-code-body')
    const gutter = card.querySelector('.draft-gutter')
    if (!(body instanceof HTMLElement) || !(gutter instanceof HTMLOListElement)) {
      continue
    }
    const count = Math.max(1, body.innerText.replace(/\n$/, '').split('\n').length)
    if (gutter.childElementCount === count) {
      continue
    }
    gutter.replaceChildren()
    for (let index = 0; index < count; index += 1) {
      gutter.appendChild(document.createElement('li'))
    }
  }
}

function onDraftCode(event: MouseEvent) {
  const target = event.target
  if (!(target instanceof HTMLElement)) {
    return
  }
  const card = target.closest('.draft-code')
  if (!(card instanceof HTMLElement)) {
    return
  }
  const body = card.querySelector('.draft-code-body')
  if (target.closest('.draft-copy')) {
    event.preventDefault()
    void copyCode(body?.textContent ?? '', 'draft')
    return
  }
  const wrap = target.closest('.draft-wrap')
  if (wrap && body instanceof HTMLElement) {
    event.preventDefault()
    const off = body.dataset.wrap !== 'off'
    body.dataset.wrap = off ? 'off' : 'on'
    wrap.textContent = off ? '自动换行' : '取消换行'
  }
}

async function copyCode(text: string, key: string) {
  try {
    await navigator.clipboard.writeText(text)
    copiedKey.value = key
    window.setTimeout(() => {
      if (copiedKey.value === key) {
        copiedKey.value = ''
      }
    }, 1500)
  } catch {
    store.notice = '复制失败'
  }
}

function stopDropperStream() {
  dropperStream?.getTracks().forEach((track) => track.stop())
  dropperStream = null
}

function closeDropper() {
  stopDropperStream()
  dropperOpen.value = false
  loupe.value = { x: 0, y: 0, on: false }
}

async function pickScreenColor() {
  if (!navigator.mediaDevices?.getDisplayMedia) {
    store.notice = '当前浏览器不支持屏幕取色'
    return
  }
  try {
    const stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: false })
    stopDropperStream()
    dropperStream = stream
    dropperHex.value = ink.value
    dropperOpen.value = true
    await nextTick()
    const video = dropperVideo.value
    if (!video) {
      closeDropper()
      return
    }
    video.srcObject = stream
    video.muted = true
    await video.play()
    stream.getVideoTracks()[0]?.addEventListener('ended', closeDropper)
  } catch {
    closeDropper()
    store.notice = '没有完成取色'
  }
}

function onDropperMove(event: MouseEvent) {
  const video = dropperVideo.value
  const stage = event.currentTarget
  if (!video || !(stage instanceof HTMLElement) || video.videoWidth === 0) {
    return
  }
  const box = stage.getBoundingClientRect()
  const scale = Math.min(box.width / video.videoWidth, box.height / video.videoHeight)
  const width = video.videoWidth * scale
  const height = video.videoHeight * scale
  const left = (box.width - width) / 2
  const top = (box.height - height) / 2
  const x = event.clientX - box.left - left
  const y = event.clientY - box.top - top
  loupe.value = { x: event.clientX - box.left, y: event.clientY - box.top, on: true }
  if (x < 0 || y < 0 || x >= width || y >= height) {
    return
  }
  const canvas = document.createElement('canvas')
  canvas.width = 1
  canvas.height = 1
  const ctx = canvas.getContext('2d', { willReadFrequently: true })
  if (!ctx) {
    return
  }
  ctx.drawImage(video, x / scale, y / scale, 1, 1, 0, 0, 1, 1)
  const pixel = ctx.getImageData(0, 0, 1, 1).data
  const hex = `#${[pixel[0], pixel[1], pixel[2]].map((n) => n.toString(16).padStart(2, '0')).join('')}`.toUpperCase()
  dropperHex.value = hex
}

function commitDropper() {
  const color = dropperHex.value
  closeDropper()
  applyInk(color)
}

async function captureScreen() {
  if (!store.active || !canSendFile.value) {
    return
  }
  if (!navigator.mediaDevices?.getDisplayMedia) {
    store.notice = '当前浏览器不支持截图'
    return
  }
  try {
    const stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: false })
    const video = document.createElement('video')
    video.srcObject = stream
    video.muted = true
    await video.play()
    if (video.videoWidth === 0) {
      await new Promise<void>((resolve) => {
        video.onloadeddata = () => resolve()
      })
    }
    const canvas = document.createElement('canvas')
    canvas.width = video.videoWidth
    canvas.height = video.videoHeight
    if (canvas.width === 0 || canvas.height === 0) {
      stream.getTracks().forEach((track) => track.stop())
      store.notice = '截图失败'
      return
    }
    canvas.getContext('2d')?.drawImage(video, 0, 0)
    stream.getTracks().forEach((track) => track.stop())
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'))
    if (!blob) {
      store.notice = '截图失败'
      return
    }
    showShot(blob, 'screenshot.png')
  } catch {
    store.notice = '没有完成截图'
  }
}

function closeShot() {
  if (shotUrl.value) {
    URL.revokeObjectURL(shotUrl.value)
  }
  shotUrl.value = ''
  shotFile.value = null
}

async function sendShot() {
  const file = shotFile.value
  const conv = store.active
  closeShot()
  if (!file || !conv) {
    return
  }
  await store.sendFiles(conv.id, [file])
}

async function addFriend(user: PublicUser) {
  try {
    await store.requestFriend(user.id)
    store.notice = ''
  } catch (err) {
    store.notice = apiErrorMessage(err)
  }
}

async function dropFriend(userId: string) {
  try {
    await store.removeFriend(userId)
  } catch (err) {
    store.notice = apiErrorMessage(err)
  }
}

function insertEmoji(token: string) {
  const el = composer.value
  if (!el) {
    return
  }
  restoreRange()
  document.execCommand('insertText', false, token)
  const next = [token, ...recentEmoji.value.filter((item) => item !== token)].slice(0, 16)
  recentEmoji.value = next
  localStorage.setItem('sl_emoji_recent', JSON.stringify(next))
  syncEditor()
  saveRange()
}

let scrollTimer = 0
let scrollStream: MediaStream | null = null
let scrollVideo: HTMLVideoElement | null = null
let scrollFrames: ImageData[] = []
let recorder: MediaRecorder | null = null
let recordStream: MediaStream | null = null
let recordChunks: Blob[] = []

function showShot(blob: Blob, name: string) {
  if (shotUrl.value) {
    URL.revokeObjectURL(shotUrl.value)
  }
  shotFile.value = new File([blob], name, { type: blob.type || 'image/png' })
  shotUrl.value = URL.createObjectURL(blob)
}

async function grabFrame(stream: MediaStream): Promise<HTMLCanvasElement | null> {
  const video = document.createElement('video')
  video.srcObject = stream
  video.muted = true
  await video.play()
  if (video.videoWidth === 0) {
    await new Promise<void>((resolve) => {
      video.onloadeddata = () => resolve()
    })
  }
  if (video.videoWidth === 0 || video.videoHeight === 0) {
    return null
  }
  const canvas = document.createElement('canvas')
  canvas.width = video.videoWidth
  canvas.height = video.videoHeight
  canvas.getContext('2d')?.drawImage(video, 0, 0)
  return canvas
}

function rowSig(data: ImageData, y: number): number {
  let hash = 0
  const start = y * data.width * 4
  for (let x = 0; x < data.width; x += 12) {
    const index = start + x * 4
    hash = (hash * 33 + data.data[index] + data.data[index + 1] + data.data[index + 2]) | 0
  }
  return hash
}

function stitchFrames(frames: ImageData[]): HTMLCanvasElement | null {
  if (frames.length === 0) {
    return null
  }
  const width = frames[0].width
  const pieces: { frame: ImageData, from: number }[] = [{ frame: frames[0], from: 0 }]
  let total = frames[0].height
  for (let index = 1; index < frames.length; index += 1) {
    const next = frames[index]
    if (next.width !== width) {
      continue
    }
    const prev = pieces[pieces.length - 1].frame
    const anchor = Math.max(0, prev.height - 12)
    const sig = rowSig(prev, anchor)
    let match = -1
    for (let y = next.height - 12; y >= 0; y -= 1) {
      if (rowSig(next, y) === sig) {
        match = y
        break
      }
    }
    const from = match < 0 ? 0 : Math.min(next.height, match + (prev.height - anchor))
    const add = next.height - from
    if (add < 8 || total + add > 12000) {
      continue
    }
    pieces.push({ frame: next, from })
    total += add
  }
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = total
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    return null
  }
  let y = 0
  for (const piece of pieces) {
    const temp = document.createElement('canvas')
    temp.width = piece.frame.width
    temp.height = piece.frame.height
    temp.getContext('2d')?.putImageData(piece.frame, 0, 0)
    const height = piece.frame.height - piece.from
    ctx.drawImage(temp, 0, piece.from, width, height, 0, y, width, height)
    y += height
  }
  return canvas
}

async function beginScrollShot() {
  shotMenu.value = false
  if (!navigator.mediaDevices?.getDisplayMedia) {
    store.notice = '当前浏览器不支持滚动截图'
    return
  }
  try {
    const stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: false })
    const video = document.createElement('video')
    video.srcObject = stream
    video.muted = true
    await video.play()
    scrollVideo = video
    scrollStream = stream
    scrollFrames = []
    scrollingShot.value = true
    const grab = () => {
      if (!scrollVideo || scrollVideo.videoWidth === 0) {
        return
      }
      const canvas = document.createElement('canvas')
      canvas.width = scrollVideo.videoWidth
      canvas.height = scrollVideo.videoHeight
      const ctx = canvas.getContext('2d', { willReadFrequently: true })
      if (!ctx) {
        return
      }
      ctx.drawImage(scrollVideo, 0, 0)
      scrollFrames.push(ctx.getImageData(0, 0, canvas.width, canvas.height))
      if (scrollFrames.length > 24) {
        scrollFrames.shift()
      }
    }
    grab()
    window.clearInterval(scrollTimer)
    scrollTimer = window.setInterval(grab, 400)
    stream.getVideoTracks()[0]?.addEventListener('ended', () => {
      void finishScrollShot()
    })
  } catch {
    scrollingShot.value = false
    store.notice = '没有完成滚动截图'
  }
}

async function finishScrollShot() {
  window.clearInterval(scrollTimer)
  scrollTimer = 0
  scrollingShot.value = false
  scrollStream?.getTracks().forEach((track) => track.stop())
  scrollStream = null
  scrollVideo = null
  const frames = scrollFrames
  scrollFrames = []
  const canvas = stitchFrames(frames)
  if (!canvas) {
    store.notice = '滚动截图失败'
    return
  }
  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'))
  if (!blob) {
    store.notice = '滚动截图失败'
    return
  }
  showShot(blob, 'scroll.png')
}

async function beginRecord() {
  shotMenu.value = false
  if (!navigator.mediaDevices?.getDisplayMedia || typeof MediaRecorder === 'undefined') {
    store.notice = '当前浏览器不支持录屏'
    return
  }
  try {
    const stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: false })
    recordStream = stream
    recordChunks = []
    const mime = MediaRecorder.isTypeSupported('video/webm') ? 'video/webm' : ''
    const next = mime ? new MediaRecorder(stream, { mimeType: mime }) : new MediaRecorder(stream)
    recorder = next
    next.ondataavailable = (event) => {
      if (event.data.size > 0) {
        recordChunks.push(event.data)
      }
    }
    next.onstop = () => {
      const type = next.mimeType || 'video/webm'
      const blob = new Blob(recordChunks, { type })
      recordChunks = []
      recordStream?.getTracks().forEach((track) => track.stop())
      recordStream = null
      recording.value = false
      const conv = store.active
      if (!conv || blob.size === 0) {
        return
      }
      void store.sendFiles(conv.id, [new File([blob], 'recording.webm', { type })])
    }
    next.start()
    recording.value = true
    stream.getVideoTracks()[0]?.addEventListener('ended', stopRecord)
  } catch {
    recording.value = false
    store.notice = '没有完成录屏'
  }
}

function stopRecord() {
  if (recorder && recorder.state !== 'inactive') {
    recorder.stop()
    return
  }
  recording.value = false
  recordStream?.getTracks().forEach((track) => track.stop())
  recordStream = null
}

async function extractText() {
  shotMenu.value = false
  const Detector = (window as unknown as { TextDetector?: new () => { detect: (source: ImageBitmap) => Promise<{ rawValue?: string }[]> } }).TextDetector
  if (!Detector || !navigator.mediaDevices?.getDisplayMedia) {
    store.notice = '当前浏览器不能提取文字'
    return
  }
  try {
    const stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: false })
    const canvas = await grabFrame(stream)
    stream.getTracks().forEach((track) => track.stop())
    if (!canvas) {
      store.notice = '没有完成提取文字'
      return
    }
    const bitmap = await createImageBitmap(canvas)
    const found = await new Detector().detect(bitmap)
    bitmap.close()
    const text = found.map((item) => item.rawValue || '').filter(Boolean).join('\n')
    if (!text) {
      store.notice = '没有识别到文字'
      return
    }
    restoreRange()
    document.execCommand('insertText', false, text)
    syncEditor()
  } catch {
    store.notice = '没有完成提取文字'
  }
}

function onPaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.items ?? [])
    .filter((item) => item.type.startsWith('image/'))
    .map((item) => item.getAsFile())
    .filter((file): file is File => file != null)
  const conv = store.active
  if (files.length > 0 && conv && canCompose.value && canSendFile.value) {
    event.preventDefault()
    void store.sendFiles(conv.id, files)
    return
  }
  const text = event.clipboardData?.getData('text/plain')
  if (text == null) {
    return
  }
  event.preventDefault()
  document.execCommand('insertText', false, text)
  syncEditor()
}

function closeFloating() {
  convMenu.value = null
  memberMenu.value = null
  moreId.value = ''
  emojiOpen.value = false
  colorOpen.value = false
  markOpen.value = false
  shotMenu.value = false
  plusMenu.value = false
  sendMenu.value = false
}

function readRecentEmoji(): string[] {
  try {
    const raw = JSON.parse(localStorage.getItem('sl_emoji_recent') || '[]') as unknown
    if (!Array.isArray(raw)) {
      return []
    }
    return raw.filter((item): item is string => typeof item === 'string').slice(0, 16)
  } catch {
    return []
  }
}

function closeComposerPop() {
  emojiOpen.value = false
  shotMenu.value = false
  plusMenu.value = false
  sendMenu.value = false
  mentionOpen.value = false
}

function toggleFormat() {
  formatOpen.value = !formatOpen.value
  closeComposerPop()
}

function toggleEmoji() {
  const next = !emojiOpen.value
  closeComposerPop()
  emojiOpen.value = next
}

function toggleShotMenu() {
  const next = !shotMenu.value
  closeComposerPop()
  shotMenu.value = next
}

function togglePlusMenu() {
  const next = !plusMenu.value
  closeComposerPop()
  plusMenu.value = next
}

function toggleSendMenu() {
  const next = !sendMenu.value
  closeComposerPop()
  sendMenu.value = next
}

function setSendMode(ctrl: boolean) {
  ctrlEnterSend.value = ctrl
  localStorage.setItem('sl_send_mode', ctrl ? 'ctrl' : 'enter')
  sendMenu.value = false
}

function openAt() {
  if (!canCompose.value) {
    return
  }
  closeComposerPop()
  if (store.active?.type !== 'group') {
    store.notice = '在群里可以 @成员'
    return
  }
  restoreRange()
  document.execCommand('insertText', false, '@')
  syncEditor()
}

function insertQuote() {
  restoreRange()
  document.execCommand('formatBlock', false, 'blockquote')
  syncEditor()
}

function insertLink() {
  const url = window.prompt('链接地址', 'https://')
  if (!url) {
    return
  }
  if (!/^https?:\/\//i.test(url.trim())) {
    store.notice = '链接需要以 http:// 或 https:// 开头'
    return
  }
  restoreRange()
  document.execCommand('createLink', false, url.trim())
  syncEditor()
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
  if (event.key !== 'Enter') {
    return
  }
  const anchor = window.getSelection()?.anchorNode
  const field = anchor instanceof HTMLElement ? anchor : anchor?.parentElement
  if (field?.closest('.draft-code-body')) {
    return
  }
  const ime = event.isComposing || event.keyCode === 229
  if (ime) {
    return
  }
  if (composing.value) {
    event.preventDefault()
    return
  }
  const chord = event.ctrlKey || event.metaKey
  if (ctrlEnterSend.value) {
    if (!chord) {
      return
    }
    event.preventDefault()
    submitText()
    return
  }
  if (event.shiftKey || chord) {
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

function openContacts() {
  pane.value = 'contacts'
  void store.loadOnline().catch(() => {})
}

function openProfile() {
  nickname.value = store.me?.nickname ?? ''
  signature.value = store.me?.signature ?? ''
  pendant.value = store.me?.pendant ?? ''
  dnd.value = store.me?.dnd === true
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
    await store.saveProfile({
      nickname: nick,
      signature: signature.value.trim(),
      pendant: pendant.value,
      dnd: dnd.value,
    })
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
  if (dropperOpen.value) {
    closeDropper()
    return
  }
  if (colorOpen.value || markOpen.value || emojiOpen.value || shotMenu.value || plusMenu.value || sendMenu.value || mentionOpen.value) {
    colorOpen.value = false
    markOpen.value = false
    closeComposerPop()
    return
  }
  if (formatOpen.value) {
    formatOpen.value = false
    return
  }
  profileOpen.value = false
  helpOpen.value = false
  picker.value = null
  historyOpen.value = false
  membersOpen.value = false
  memberMenu.value = null
}

function groupNo(id: string): string {
  return id.slice(-6).toUpperCase()
}

function isOwner(conv: Conversation | null | undefined): boolean {
  return !!conv && conv.type === 'group' && conv.ownerId === store.me?.id
}

function openMembersFromMenu() {
  const conv = menuConv()
  convMenu.value = null
  if (!conv || conv.type !== 'group') {
    return
  }
  if (store.activeId !== conv.id) {
    void openConversation(conv).then(() => {
      membersOpen.value = true
    })
    return
  }
  membersOpen.value = true
}

function addFromMenu() {
  const conv = menuConv()
  convMenu.value = null
  if (!conv || !isOwner(conv)) {
    return
  }
  if (store.activeId !== conv.id) {
    void openConversation(conv).then(() => openPicker('add'))
    return
  }
  openPicker('add')
}

function leaveFromMenu() {
  const conv = menuConv()
  convMenu.value = null
  if (!conv || conv.type !== 'group') {
    return
  }
  void store.leave(conv.id).then(() => {
    if (store.activeId === '') {
      showThread.value = false
      membersOpen.value = false
    }
  }).catch((err) => {
    store.notice = apiErrorMessage(err)
  })
}

function onMemberMenu(event: MouseEvent, user: PublicUser) {
  convMenu.value = null
  memberMenu.value = { x: event.clientX, y: event.clientY, id: user.id }
}

function menuMember(): PublicUser | undefined {
  return store.members.find((user) => user.id === memberMenu.value?.id)
}

function messageMember() {
  const user = menuMember()
  memberMenu.value = null
  if (!user || user.id === store.me?.id) {
    return
  }
  void openContact(user)
}

function mentionMember() {
  const user = menuMember()
  memberMenu.value = null
  if (!user || user.id === store.me?.id || !canCompose.value) {
    return
  }
  const el = composer.value
  if (!el) {
    return
  }
  el.focus()
  document.execCommand('insertText', false, `@${user.nickname} `)
  syncEditor()
}

async function kickMember() {
  const user = menuMember()
  const conv = store.active
  memberMenu.value = null
  if (!user || !conv || !isOwner(conv) || user.id === store.me?.id) {
    return
  }
  try {
    await store.removeMember(conv.id, user.id)
  } catch (err) {
    store.notice = apiErrorMessage(err)
  }
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
  membersOpen.value = false
  memberMenu.value = null
  void stickToBottom(true)
})

watch(ctrlEnterSend, (value) => {
  localStorage.setItem('sl_send_mode', value ? 'ctrl' : 'enter')
})

watch(notifyOn, (value) => {
  localStorage.setItem('sl_notify', value ? '1' : '0')
  if (value && typeof Notification !== 'undefined' && Notification.permission === 'default') {
    void Notification.requestPermission()
  }
})

const noticed = new Map<string, string>()
watch(store.conversations, (list) => {
  for (const conv of list) {
    const mark = `${conv.lastAt}:${conv.unread}`
    const previous = noticed.get(conv.id)
    noticed.set(conv.id, mark)
    if (!previous || previous === mark || !notifyOn.value || conv.muted || conv.unread <= 0) {
      continue
    }
    if (conv.id === store.activeId && document.hasFocus()) {
      continue
    }
    if (typeof Notification === 'undefined' || Notification.permission !== 'granted') {
      continue
    }
    const body = listPreview(conv.lastBody) || '新消息'
    new Notification(conv.title || '新消息', { body })
  }
})

watch(keyword, () => {
  window.clearTimeout(contactTimer)
  if (pane.value !== 'contacts') {
    contactHits.value = []
    return
  }
  const q = keyword.value.trim()
  if (q === '') {
    contactHits.value = []
    contactSearching.value = false
    return
  }
  contactSearching.value = true
  contactTimer = window.setTimeout(() => {
    void searchUsers(q).then((users) => {
      contactHits.value = users.filter((user) => user.id !== store.me?.id)
    }).catch((err) => {
      store.notice = apiErrorMessage(err)
    }).finally(() => {
      contactSearching.value = false
    })
  }, 200)
})

watch(
  () => [store.activeId, store.activeMessages.map((msg) => msg.id).join(',')] as const,
  (current, previous) => {
    if (!previous || previous[0] !== current[0]) {
      return
    }
    const seen = new Set(previous[1] === '' ? [] : previous[1].split(','))
    for (const msg of store.activeMessages) {
      if (seen.has(msg.id) || !isShake(msg.body) || msg.senderId === store.me?.id) {
        continue
      }
      playShake()
    }
  },
)

watch(narrow, (value) => {
  if (value && store.activeId) {
    showThread.value = true
  }
})

onMounted(() => {
  const media = window.matchMedia('(max-width: 719px)')
  const apply = () => {
    narrow.value = media.matches
  }
  apply()
  media.addEventListener('change', apply)
  removeMedia = () => media.removeEventListener('change', apply)
  window.addEventListener('keydown', onWindowKey)
  window.addEventListener('click', closeFloating)
  document.addEventListener('selectionchange', refreshFmt)
  void store.bootstrap()
})

onUnmounted(() => {
  removeMedia()
  window.removeEventListener('keydown', onWindowKey)
  window.removeEventListener('click', closeFloating)
  document.removeEventListener('selectionchange', refreshFmt)
  window.clearTimeout(compositionTimer)
  window.clearTimeout(searchTimer)
  window.clearTimeout(shakeTimer)
  window.clearTimeout(contactTimer)
  window.clearInterval(scrollTimer)
  closeDropper()
  if (recorder && recorder.state !== 'inactive') {
    recorder.stop()
  }
  store.stop()
})
</script>

<template>
  <main v-if="store.banned" class="auth">
    <div class="card banned-card">
      <h1>账号已被封禁</h1>
      <p class="muted">该 Key 不能再登录。连接已关闭。</p>
      <button type="button" class="btn primary block" @click="leaveBanned">返回登录</button>
    </div>
  </main>

  <div v-else class="shell" :class="{ shaking }" @dragover.prevent @drop.prevent>
    <p v-if="store.notice" class="notice">{{ store.notice }}</p>
    <div class="workspace">
      <nav class="rail" :class="{ hidden: narrow && showThread }">
        <div class="rail-top">
          <button type="button" class="rail-btn me" :aria-label="store.me?.nickname || '个人资料'" @click="openProfile">
            <span class="avatar" :class="pendantClass(store.me?.pendant)">
              <img v-if="selfAvatarSrc()" :src="selfAvatarSrc()" alt="" />
              <template v-else>{{ initial(store.me?.nickname ?? '') }}</template>
              <span class="presence" :class="{ on: store.connected }"></span>
            </span>
          </button>
          <button type="button" class="rail-btn" aria-label="设置" @click="openProfile">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M12 8.2a3.8 3.8 0 1 0 0 7.6 3.8 3.8 0 0 0 0-7.6Zm8.2 3.1-.9-.2a6.7 6.7 0 0 0-.5-1.2l.6-.7a1 1 0 0 0 0-1.4l-.8-.8a1 1 0 0 0-1.4 0l-.7.6a6.7 6.7 0 0 0-1.2-.5l-.2-.9a1 1 0 0 0-1-.8h-1.2a1 1 0 0 0-1 .8l-.2.9a6.7 6.7 0 0 0-1.2.5l-.7-.6a1 1 0 0 0-1.4 0l-.8.8a1 1 0 0 0 0 1.4l.6.7c-.2.4-.4.8-.5 1.2l-.9.2a1 1 0 0 0-.8 1v1.2a1 1 0 0 0 .8 1l.9.2c.1.4.3.8.5 1.2l-.6.7a1 1 0 0 0 0 1.4l.8.8a1 1 0 0 0 1.4 0l.7-.6c.4.2.8.4 1.2.5l.2.9a1 1 0 0 0 1 .8h1.2a1 1 0 0 0 1-.8l.2-.9c.4-.1.8-.3 1.2-.5l.7.6a1 1 0 0 0 1.4 0l.8-.8a1 1 0 0 0 0-1.4l-.6-.7c.2-.4.4-.8.5-1.2l.9-.2a1 1 0 0 0 .8-1v-1.2a1 1 0 0 0-.8-1Z"/></svg>
            设置
          </button>
        </div>
        <div class="rail-nav">
          <button type="button" class="rail-btn" :class="{ active: pane === 'messages' }" @click="pane = 'messages'">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M4 5.5A2.5 2.5 0 0 1 6.5 3h11A2.5 2.5 0 0 1 20 5.5v9A2.5 2.5 0 0 1 17.5 17H9l-3.8 2.7A.8.8 0 0 1 4 19V5.5Z"/></svg>
            消息
          </button>
          <button type="button" class="rail-btn" :class="{ active: pane === 'contacts' }" @click="openContacts">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M9 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6Zm7.5 1a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5ZM9 13c-2.8 0-5 1.6-5 3.5V18h10v-1.5C14 14.6 11.8 13 9 13Zm6.2.2c-.5.1-1 .3-1.5.6 1.4.8 2.3 2 2.3 3.2V18H20v-1.2c0-1.7-2-3.2-4.8-3.6Z"/></svg>
            联系人
          </button>
        </div>
        <button type="button" class="rail-btn rail-help" @click="helpOpen = true">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18Zm0 13.2a1.1 1.1 0 1 1 0-2.2 1.1 1.1 0 0 1 0 2.2Zm1.4-5.5-.5.3c-.3.2-.4.4-.4.8v.4h-1.6v-.6c0-.9.4-1.6 1.3-2.1.4-.2.6-.5.6-.9 0-.6-.5-1-1.2-1-.8 0-1.3.5-1.4 1.2H8.6C8.8 7.4 10 6.2 12 6.2c1.8 0 3.1 1.1 3.1 2.7 0 1-.5 1.8-1.7 2.5Z"/></svg>
          帮助
        </button>
      </nav>

      <section class="list" :class="{ hidden: narrow && showThread, grow: narrow }">
        <div class="search-wrap">
          <input v-model="keyword" type="search" :placeholder="pane === 'contacts' ? '搜索联系人' : '搜索会话'" aria-label="搜索" />
          <span v-if="narrow && !showThread" class="latency" :class="latencyLevel" :title="`网络延迟 ${latencyText}`">{{ latencyText }}</span>
        </div>
        <button
          v-if="pane === 'contacts' && store.me?.canCreateGroup"
          type="button"
          class="create"
          @click="openPicker('create')"
        >
          建群
        </button>
        <div class="scroll">
          <p v-if="!store.ready" class="empty">加载中</p>
          <template v-else-if="pane === 'messages'">
            <p v-if="filteredConversations.length === 0" class="empty">没有会话</p>
            <button
              v-for="conv in filteredConversations"
              :key="conv.id"
              type="button"
              class="row"
              :class="{ active: store.activeId === conv.id, drop: dragGroupId === conv.id, pinned: conv.pinned }"
              @click="openConversation(conv)"
              @contextmenu.prevent="onConvMenu($event, conv)"
              @dragover="onGroupDragOver(conv, $event)"
              @dragleave="onGroupDragLeave(conv, $event)"
              @drop="onGroupDrop(conv, $event)"
            >
              <span class="avatar" :class="[conv.type === 'group' ? 'group' : pendantClass(conv.peer?.pendant), { 'offline-avatar': peerOffline(conv) }]">
                <img v-if="conv.type !== 'group' && conv.peer?.avatarUrl" :src="withSession(conv.peer.avatarUrl)" alt="" />
                <template v-else>{{ initial(conv.title) }}</template>
                <i v-if="conv.type === 'group'" class="group-mark">群</i>
              </span>
              <span class="row-body">
                <span class="row-top">
                  <span class="name">{{ conv.title }}</span>
                  <span v-if="conv.type === 'group'" class="pin">群</span>
                  <span v-if="conv.peer?.dnd" class="pin">勿扰</span>
                  <span v-if="conv.pinned" class="pin">置顶</span>
                  <span v-if="peerOffline(conv)" class="time">离线</span>
                  <span class="time">{{ formatStamp(conv.lastAt) }}</span>
                </span>
                <span class="row-bottom">
                  <span class="preview">{{ listPreview(conv.lastBody) }}</span>
                  <span v-if="conv.muted" class="mute">免</span>
                  <span v-else-if="conv.unread > 0" class="badge">{{ conv.unread > 99 ? '99+' : conv.unread }}</span>
                </span>
              </span>
            </button>
          </template>
          <template v-else>
            <p v-if="contactSearching" class="empty">搜索中</p>
            <template v-else-if="keyword.trim()">
              <p v-if="contactHits.length === 0" class="empty">没有匹配的用户</p>
              <div v-for="user in contactHits" :key="user.id" class="contact" @click="openContact(user)">
                <span class="avatar" :class="pendantClass(user.pendant)">
                  <img v-if="user.avatarUrl" :src="withSession(user.avatarUrl)" alt="" />
                  <template v-else>{{ initial(user.nickname) }}</template>
                  <i v-if="user.online" class="presence on"></i>
                </span>
                <span class="row-body">
                  <span class="name">{{ user.nickname }}</span>
                  <span class="preview">{{ contactLine(user) }}</span>
                </span>
                <button v-if="isFriend(user.id)" type="button" class="contact-act" @click.stop="openContact(user)">发消息</button>
                <button v-else-if="isOutgoing(user.id)" type="button" class="contact-act quiet" disabled>已申请</button>
                <button v-else type="button" class="contact-act" @click.stop="addFriend(user)">加好友</button>
              </div>
            </template>
            <template v-else>
              <section v-if="store.incoming.length" class="contact-block">
                <h3 class="contact-head"><strong>新的朋友</strong><span>{{ store.incoming.length }}</span></h3>
                <div v-for="user in store.incoming" :key="user.id" class="contact">
                  <span class="avatar" :class="pendantClass(user.pendant)">
                    <img v-if="user.avatarUrl" :src="withSession(user.avatarUrl)" alt="" />
                    <template v-else>{{ initial(user.nickname) }}</template>
                  </span>
                  <span class="row-body">
                    <span class="name">{{ user.nickname }}</span>
                    <span class="preview">请求加你为好友</span>
                  </span>
                  <span class="contact-acts">
                    <button type="button" class="contact-act" @click="addFriend(user)">同意</button>
                    <button type="button" class="contact-act quiet" @click="dropFriend(user.id)">拒绝</button>
                  </span>
                </div>
              </section>
              <section v-if="onlineContacts.length" class="contact-block">
                <h3 class="contact-head"><strong>在线</strong><span>{{ onlineContacts.length }}</span></h3>
                <div v-for="user in onlineContacts" :key="user.id" class="contact" @click="openContact(user)">
                  <span class="avatar" :class="pendantClass(user.pendant)">
                    <img v-if="user.avatarUrl" :src="withSession(user.avatarUrl)" alt="" />
                    <template v-else>{{ initial(user.nickname) }}</template>
                    <i class="presence on"></i>
                  </span>
                  <span class="row-body">
                    <span class="name">{{ user.nickname }}</span>
                    <span class="preview">{{ contactLine(user) }}</span>
                  </span>
                  <button v-if="isOutgoing(user.id)" type="button" class="contact-act quiet" disabled>已申请</button>
                  <button v-else type="button" class="contact-act" @click.stop="addFriend(user)">加好友</button>
                </div>
              </section>
              <section class="contact-block">
                <h3 class="contact-head"><strong>好友</strong><span>{{ store.friends.length }}</span></h3>
                <p v-if="store.friends.length === 0" class="contact-note">搜索昵称添加好友</p>
                <div
                  v-for="user in store.friends"
                  :key="user.id"
                  class="contact"
                  :class="{ drop: dragContactId === user.id }"
                  @click="openContact(user)"
                  @dragover.prevent="dragContactId = user.id"
                  @dragleave="onContactDragLeave(user.id, $event)"
                  @drop.stop.prevent="onDropContact(user, $event)"
                >
                  <span class="avatar" :class="pendantClass(user.pendant)">
                    <img v-if="user.avatarUrl" :src="withSession(user.avatarUrl)" alt="" />
                    <template v-else>{{ initial(user.nickname) }}</template>
                    <i v-if="user.online" class="presence on"></i>
                  </span>
                  <span class="row-body">
                    <span class="name">{{ user.nickname }}</span>
                    <span class="preview">{{ contactLine(user) }}</span>
                  </span>
                  <button type="button" class="contact-act quiet" @click.stop="dropFriend(user.id)">删除</button>
                </div>
              </section>
            </template>
          </template>
        </div>
      </section>

      <section
        class="thread"
        :class="{ hidden: narrow && !showThread, drag: dragChat }"
        @dragover.prevent="dragChat = true"
        @dragleave="onChatDragLeave"
        @drop.stop.prevent="onDropChat"
      >
        <header class="thread-head">
          <button v-if="narrow && showThread" type="button" class="btn" @click="showThread = false">返回</button>
          <template v-if="store.active">
            <span class="avatar" :class="[store.active.type === 'group' ? 'group' : pendantClass(store.active.peer?.pendant), { 'offline-avatar': peerOffline(store.active) }]">
              <img v-if="store.active.type !== 'group' && store.active.peer?.avatarUrl" :src="withSession(store.active.peer.avatarUrl)" alt="" />
              <template v-else>{{ initial(store.active.title) }}</template>
              <i v-if="store.active.type === 'group'" class="group-mark">群</i>
            </span>
            <span class="thread-title">
              <span class="name">
                {{ store.active.title }}
                <em v-if="store.active.type === 'group'" class="group-no">群号 {{ groupNo(store.active.id) }}</em>
              </span>
              <span class="sub">
                <template v-if="store.active.type === 'group'">群 · {{ store.members.length || store.active.memberCount }} 人</template>
                <template v-else>{{ store.active.peer?.dnd ? '免打扰' : (store.active.peer?.online ? '在线' : '离线') }}<template v-if="store.active.peer?.signature"> · {{ store.active.peer.signature }}</template></template>
              </span>
            </span>
            <button v-if="store.active.type === 'group'" type="button" class="btn" @click="membersOpen = !membersOpen">成员</button>
            <button
              v-if="isOwner(store.active)"
              type="button"
              class="btn"
              @click="openPicker('add')"
            >
              添加成员
            </button>
            <button v-if="store.active.type === 'group'" type="button" class="btn" @click="onLeave">退出群</button>
          </template>
          <button v-if="narrow" type="button" class="btn" @click="openProfile">个人资料</button>
          <span v-if="!(narrow && !showThread)" class="latency" :class="latencyLevel" :title="`网络延迟 ${latencyText}`">{{ latencyText }}</span>
        </header>

        <div v-if="store.active?.type === 'group' && store.members.length" class="member-strip">
          <button
            v-for="user in store.members.slice(0, 8)"
            :key="user.id"
            type="button"
            class="avatar sm"
            :class="{ owner: user.id === store.active.ownerId }"
            :title="user.id === store.active.ownerId ? `${user.nickname} · 群主` : user.nickname"
            @click="membersOpen = true"
            @contextmenu.prevent="onMemberMenu($event, user)"
          >
            <img v-if="user.avatarUrl" :src="withSession(user.avatarUrl)" alt="" />
            <template v-else>{{ initial(user.nickname) }}</template>
          </button>
          <button type="button" class="strip-more" @click="membersOpen = true">{{ store.members.length }} 人</button>
        </div>

        <div class="thread-main">
        <div ref="scroller" class="msgs">
          <p v-if="!store.active" class="empty">选择一个会话</p>
          <p v-else-if="store.activeMessages.length === 0" class="empty">没有消息</p>
          <template v-for="msg in store.activeMessages" :key="msg.id">
            <p v-if="msg.id === store.freshFromId" class="fresh">以下为新消息</p>
            <p v-if="msg.recalled" class="recall">{{ recallLine(msg) }}</p>
            <p v-else-if="isShake(msg.body)" class="recall">{{ shakeLine(msg) }}</p>
            <div
              v-else
              class="msg"
              :class="{ mine: msg.senderId === store.me?.id }"
            >
              <span v-if="msg.senderId !== store.me?.id" class="avatar">{{ initial(msg.senderNickname) }}</span>
              <div class="msg-body">
                <p v-if="store.active?.type === 'group' && msg.senderId !== store.me?.id" class="who">
                  {{ msg.senderNickname }}
                </p>
                <div class="bubble-wrap" @click.stop>
                  <div class="toolbar">
                    <button
                      v-for="item in quickReactions"
                      :key="item.emoji"
                      type="button"
                      :title="item.label"
                      @click="doReact(msg, item.emoji)"
                    >{{ item.label }}</button>
                    <button type="button" @click="startReply(msg)">回复</button>
                    <button type="button" @click="toggleMore(msg.id)">···</button>
                    <div v-if="moreId === msg.id" class="more">
                      <button type="button" @click="copyMessage(msg)">复制</button>
                      <button type="button" @click="openForward(msg)">转发</button>
                      <button v-if="canRecall(msg)" type="button" @click="doRecall(msg)">撤回</button>
                    </div>
                  </div>
                  <div v-if="msg.reply" class="quote">
                    <span class="quote-name">{{ msg.reply.senderNickname }}</span>
                    <span>{{ quoteText(msg.reply) }}</span>
                  </div>
                  <img
                    v-if="msg.kind === 'image' && msg.fileId"
                    class="photo"
                    :src="withSession(`/api/files/${msg.fileId}`)"
                    alt="图片"
                    @click="previewUrl = withSession(`/api/files/${msg.fileId}`)"
                    @load="stickToBottom()"
                  />
                  <a
                    v-else-if="msg.kind === 'file' && msg.fileId"
                    class="file"
                    :href="withSession(`/api/files/${msg.fileId}`)"
                    :download="msg.fileName || '文件'"
                  >
                    <span class="file-name">{{ msg.fileName || '文件' }}</span>
                    <span class="file-meta">
                      <template v-if="msg.fileSize != null">{{ formatBytes(msg.fileSize) }} · </template>下载
                    </span>
                  </a>
                  <div v-else class="bubble">
                    <template v-for="(block, blockIndex) in messageBlocks(msg.body)" :key="blockIndex">
                      <div v-if="block.type === 'code'" class="code">
                        <div class="code-bar">
                          <span class="code-lang">{{ block.lang || '代码' }}</span>
                          <button type="button" @click="copyCode(block.text, `${msg.id}-${blockIndex}`)">{{ copiedKey === `${msg.id}-${blockIndex}` ? '已复制' : '复制' }}</button>
                        </div>
                        <ol class="code-lines">
                          <li v-for="(line, lineIndex) in codeLines(block.text)" :key="lineIndex"><code>{{ line || ' ' }}</code></li>
                        </ol>
                      </div>
                      <template v-else>
                        <template v-for="(piece, pieceIndex) in richBlocks(block.text)" :key="`${blockIndex}-${pieceIndex}`">
                          <ul v-if="piece.kind === 'list' && piece.list === 'ul'" class="rich-list">
                            <li v-for="(item, itemIndex) in piece.items" :key="itemIndex">
                              <template v-for="(run, runIndex) in item" :key="runIndex">
                                <template v-for="(part, index) in textParts(run.text)" :key="`${runIndex}-${index}`">
                                  <span v-if="part.mention" class="mention" :style="runStyle(run.style)">{{ part.text }}</span>
                                  <span v-else :style="runStyle(run.style)">{{ part.text }}</span>
                                </template>
                              </template>
                            </li>
                          </ul>
                          <ol v-else-if="piece.kind === 'list'" class="rich-list">
                            <li v-for="(item, itemIndex) in piece.items" :key="itemIndex">
                              <template v-for="(run, runIndex) in item" :key="runIndex">
                                <template v-for="(part, index) in textParts(run.text)" :key="`${runIndex}-${index}`">
                                  <span v-if="part.mention" class="mention" :style="runStyle(run.style)">{{ part.text }}</span>
                                  <span v-else :style="runStyle(run.style)">{{ part.text }}</span>
                                </template>
                              </template>
                            </li>
                          </ol>
                          <div v-else class="rich-line" :style="piece.align ? { textAlign: piece.align } : undefined">
                            <br v-if="piece.runs.length === 0" />
                            <template v-for="(run, runIndex) in piece.runs" :key="runIndex">
                              <template v-for="(part, index) in textParts(run.text)" :key="`${runIndex}-${index}`">
                                <span v-if="part.mention" class="mention" :style="runStyle(run.style)">{{ part.text }}</span>
                                <span v-else :style="runStyle(run.style)">{{ part.text }}</span>
                              </template>
                            </template>
                          </div>
                        </template>
                      </template>
                    </template>
                  </div>
                  <div v-if="msg.reactions?.length" class="reacts">
                    <button
                      v-for="item in msg.reactions"
                      :key="item.emoji"
                      type="button"
                      class="react"
                      :class="{ on: item.self }"
                      @click="doReact(msg, item.emoji)"
                    >{{ reactionLabel(item.emoji) }} {{ item.count }}</button>
                  </div>
                </div>
                <p class="stamp">{{ formatStamp(msg.createdAt) }}</p>
              </div>
            </div>
          </template>
        </div>

        <form class="composer" :class="{ sized: composerHeight > 0, expanded: composerExpanded }" :style="composerHeight > 0 ? { height: `${composerHeight}px` } : undefined" @submit.prevent="submitText">
          <div class="composer-resize" role="separator" aria-orientation="horizontal" aria-label="调整输入区高度" @pointerdown="startComposerResize"></div>
          <div v-if="replyTo" class="replying">
            <span>回复 {{ replyTo.senderNickname }}：{{ quoteText(replyTo) }}</span>
            <button type="button" @click="replyTo = null">取消</button>
          </div>
          <div v-if="recording || scrollingShot" class="capture-bar">
            <span>{{ recording ? '正在录屏' : '滚动画面，完成后点完成' }}</span>
            <button type="button" @click="recording ? stopRecord() : finishScrollShot()">完成</button>
          </div>
          <div v-if="mentionOpen" class="mentions" @click.stop>
            <p class="emoji-label">选择成员</p>
            <p class="emoji-label">试试直接输入 @</p>
            <button
              v-for="user in mentionHits"
              :key="user.id"
              type="button"
              @click="pickMention(user)"
            >{{ user.nickname }}</button>
            <p v-if="mentionHits.length === 0" class="empty">没有匹配的成员</p>
          </div>
          <div v-if="emojiOpen" class="emoji-panel" @click.stop>
            <template v-if="recentEmoji.length">
              <p class="emoji-label">最常使用</p>
              <div class="emoji-grid">
                <button v-for="token in recentEmoji" :key="`recent-${token}`" type="button" @mousedown.prevent @click="insertEmoji(token)">{{ token }}</button>
              </div>
            </template>
            <p class="emoji-label">默认表情</p>
            <div class="emoji-grid">
              <button v-for="token in composeEmoji" :key="token" type="button" @mousedown.prevent @click="insertEmoji(token)">{{ token }}</button>
            </div>
          </div>
          <div v-if="formatOpen" class="format" @click.stop>
            <button type="button" class="fmt" :class="{ on: fmtState.bold }" :disabled="!canCompose" title="加粗" aria-label="加粗" @mousedown.prevent @click="formatCommand('bold')">B</button>
            <button type="button" class="fmt strike" :class="{ on: fmtState.strike }" :disabled="!canCompose" title="删除线" aria-label="删除线" @mousedown.prevent @click="formatCommand('strikeThrough')">S</button>
            <button type="button" class="fmt italic" :class="{ on: fmtState.italic }" :disabled="!canCompose" title="斜体" aria-label="斜体" @mousedown.prevent @click="formatCommand('italic')">I</button>
            <button type="button" class="fmt under" :class="{ on: fmtState.underline }" :disabled="!canCompose" title="下划线" aria-label="下划线" @mousedown.prevent @click="formatCommand('underline')">U</button>
            <span class="fmt-gap"></span>
            <div class="fmt-pop">
              <button type="button" class="fmt color-btn" :disabled="!canCompose" title="文字颜色" :aria-label="`文字颜色 ${shownInk}`" @mousedown.prevent @click="toggleColor">
                <span>A</span>
                <i :style="{ background: shownInk }"></i>
              </button>
              <div v-if="colorOpen" class="swatch-panel">
                <button
                  v-for="color in richColors"
                  :key="color"
                  type="button"
                  class="swatch"
                  :class="{ on: shownInk === color }"
                  :style="{ background: color }"
                  :aria-label="color"
                  @mousedown.prevent
                  @mouseenter="hoverInk = color"
                  @mouseleave="hoverInk = ''"
                  @click="applyInk(color)"
                ></button>
                <span class="swatch-hex">{{ shownInk }}</span>
              </div>
            </div>
            <div class="fmt-pop">
              <button type="button" class="fmt color-btn" :disabled="!canCompose" title="高亮" aria-label="高亮" @mousedown.prevent @click="toggleMark">
                <span class="mark-letter">A</span>
                <i class="mark-bar"></i>
              </button>
              <div v-if="markOpen" class="swatch-panel">
                <button
                  v-for="color in richMarks"
                  :key="color"
                  type="button"
                  class="swatch"
                  :style="{ background: color }"
                  :aria-label="color"
                  @mousedown.prevent
                  @click="applyMark(color)"
                ></button>
              </div>
            </div>
            <span class="fmt-gap"></span>
            <button type="button" class="fmt" :class="{ on: fmtState.align === 'left' }" :disabled="!canCompose" title="左对齐" aria-label="左对齐" @mousedown.prevent @click="formatCommand('justifyLeft')">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M4 6h16v2H4V6Zm0 5h10v2H4v-2Zm0 5h16v2H4v-2Z"/></svg>
            </button>
            <button type="button" class="fmt" :class="{ on: fmtState.align === 'center' }" :disabled="!canCompose" title="居中" aria-label="居中" @mousedown.prevent @click="formatCommand('justifyCenter')">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M4 6h16v2H4V6Zm3 5h10v2H7v-2Zm-3 5h16v2H4v-2Z"/></svg>
            </button>
            <button type="button" class="fmt" :class="{ on: fmtState.align === 'right' }" :disabled="!canCompose" title="右对齐" aria-label="右对齐" @mousedown.prevent @click="formatCommand('justifyRight')">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M4 6h16v2H4V6Zm6 5h10v2H10v-2Zm-6 5h16v2H4v-2Z"/></svg>
            </button>
            <span class="fmt-gap"></span>
            <button type="button" class="fmt" :disabled="!canCompose" title="无序列表" aria-label="无序列表" @mousedown.prevent @click="formatCommand('insertUnorderedList')">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M5 6.5a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0ZM8 5h13v3H8V5Zm-3 8.5a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0ZM8 12h13v3H8v-3Zm-3 8.5a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0ZM8 19h13v3H8v-3Z"/></svg>
            </button>
            <button type="button" class="fmt" :disabled="!canCompose" title="有序列表" aria-label="有序列表" @mousedown.prevent @click="formatCommand('insertOrderedList')">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M3 5h2v4H4V6H3V5Zm0 7h3v1H4v1h2v1H3v-1H2v-2h1v-0Zm1 6H3v1h2v1H2v-1h2v-1H3v0Zm5-12h13v3H8V6Zm0 7h13v3H8v-3Zm0 7h13v3H8v-3Z"/></svg>
            </button>
            <span class="fmt-gap"></span>
            <button type="button" class="fmt" :disabled="!canCompose" title="引用" aria-label="引用" @mousedown.prevent @click="insertQuote">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M7 7h4v6H6V9a2 2 0 0 1 2-2Zm8 0h4v6h-5V9a2 2 0 0 1 2-2Z"/></svg>
            </button>
            <button type="button" class="fmt" :disabled="!canCompose" title="链接" aria-label="链接" @mousedown.prevent @click="insertLink">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M10.6 13.4a1 1 0 0 1 0-1.4l2.8-2.8a2 2 0 0 1 2.8 2.8l-1.2 1.2 1.4 1.4 1.2-1.2a4 4 0 0 0-5.6-5.6l-2.8 2.8a4 4 0 0 0 0 5.6l.6.6 1.4-1.4-.6-.6Zm2.8-2.8a1 1 0 0 1 0 1.4l-2.8 2.8a2 2 0 1 1-2.8-2.8l1.2-1.2-1.4-1.4-1.2 1.2a4 4 0 0 0 5.6 5.6l2.8-2.8a4 4 0 0 0 0-5.6l-.6-.6-1.4 1.4.6.6Z"/></svg>
            </button>
            <button type="button" class="fmt" :disabled="!canCompose" title="代码" aria-label="代码" @mousedown.prevent @click="insertCodeBlock">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M8.7 16.3 4.4 12l4.3-4.3L7.3 6.3 1.6 12l5.7 5.7 1.4-1.4Zm6.6 0 1.4 1.4L22.4 12l-5.7-5.7-1.4 1.4L19.6 12l-4.3 4.3Z"/></svg>
            </button>
            <span class="fmt-gap"></span>
            <label class="fmt-select font">
              <select v-model="fontId" :disabled="!canCompose" aria-label="字体" @mousedown="saveRange" @change="applyFont">
                <option v-for="item in richFonts" :key="item.id" :value="item.id" :style="{ fontFamily: fontStacks[item.id] }">{{ item.label }}</option>
              </select>
            </label>
            <label class="fmt-select size">
              <select v-model="fontPx" :disabled="!canCompose" aria-label="字号" @mousedown="saveRange" @change="applySize">
                <option v-for="item in richSizes" :key="item.px" :value="item.px">{{ item.px }}</option>
              </select>
            </label>
            <span class="fmt-gap"></span>
            <button type="button" class="fmt clear" :disabled="!canCompose" title="清除格式" aria-label="清除格式" @mousedown.prevent @click="clearFormat">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M6 5h9.2l-1.2 2H8.2L6 5Zm2.6 4h6.1L11 18h-2.1l2.7-9ZM15.8 7l1.2-2H20v2h-4.2ZM4.7 19.3 3.3 17.9 17.9 3.3l1.4 1.4L4.7 19.3Z"/></svg>
            </button>
          </div>
          <div class="compose-main">
            <div
              ref="composer"
              class="editor"
              :class="{ empty: editorEmpty }"
              :contenteditable="canCompose ? 'true' : 'false'"
              :data-placeholder="editorPlaceholder"
              role="textbox"
              aria-multiline="true"
              :aria-label="editorPlaceholder"
              @input="syncEditor"
              @mouseup="saveRange"
              @keyup="saveRange"
              @keydown="onComposerKeydown"
              @paste="onPaste"
              @click="onDraftCode"
              @compositionstart="onCompositionStart"
              @compositionend="onCompositionEnd"
            ></div>
            <div class="composer-icons">
              <button type="button" class="icon-btn aa" :class="{ on: formatOpen }" :disabled="!canCompose" title="排版" aria-label="排版" @click.stop="toggleFormat">Aa</button>
              <button type="button" class="icon-btn" :class="{ on: emojiOpen }" :disabled="!canCompose" title="表情" aria-label="表情" @click.stop="toggleEmoji">
                <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="none" stroke="currentColor" stroke-width="1.8" d="M12 20a8 8 0 1 0 0-16 8 8 0 0 0 0 16Z"/><path fill="currentColor" d="M9 10.2a1 1 0 1 0 0-2 1 1 0 0 0 0 2Zm6 0a1 1 0 1 0 0-2 1 1 0 0 0 0 2Z"/><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" d="M8.5 13.5c.8 1.4 2 2.1 3.5 2.1s2.7-.7 3.5-2.1"/></svg>
              </button>
              <button type="button" class="icon-btn" :disabled="!canCompose" title="选择成员，试试直接输入 @" aria-label="选择成员" @click.stop="openAt">
                <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="none" stroke="currentColor" stroke-width="1.8" d="M12 16.5a4.5 4.5 0 1 0-3.2-7.7"/><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" d="M16 12v1.2a2.3 2.3 0 0 0 4.6 0V12a8.6 8.6 0 1 0-3.4 6.8"/></svg>
              </button>
              <div class="icon-pop">
                <button type="button" class="icon-btn" :class="{ on: shotMenu }" :disabled="!canCompose" title="截图" aria-label="截图" @click.stop="toggleShotMenu">
                  <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" d="M8 5.5 9.2 4h5.6L16 5.5h3.2v12H4.8v-12H8Z"/><circle cx="12" cy="11.5" r="2.6" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>
                </button>
                <div v-if="shotMenu" class="pop-menu" @click.stop>
                  <button type="button" @click="extractText">提取文字</button>
                  <button type="button" :disabled="!canSendFile" @click="beginScrollShot">滚动截图</button>
                  <button type="button" :disabled="!canSendFile" @click="beginRecord">录屏</button>
                  <button type="button" :disabled="!canSendFile" @click="shotMenu = false; captureScreen()">截图</button>
                </div>
              </div>
              <div class="icon-pop">
                <button type="button" class="icon-btn" :class="{ on: plusMenu }" :disabled="!canCompose" title="更多" aria-label="更多" @click.stop="togglePlusMenu">
                  <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" d="M12 6v12M6 12h12"/></svg>
                </button>
                <div v-if="plusMenu" class="pop-menu" @click.stop>
                  <button type="button" :disabled="!canSendFile" @click="plusMenu = false; imageInput?.click()">图片</button>
                  <button type="button" :disabled="!canSendFile" @click="plusMenu = false; fileInput?.click()">文件</button>
                  <button type="button" :disabled="!store.active" @click="plusMenu = false; historyOpen = true">聊天记录</button>
                  <button type="button" @click="plusMenu = false; sendShake()">窗口抖动</button>
                  <button type="button" @mousedown.prevent @click="plusMenu = false; pickScreenColor()">屏幕取色</button>
                </div>
              </div>
              <button type="button" class="icon-btn" :class="{ on: composerExpanded }" :disabled="!canCompose" :title="composerExpanded ? '收起' : '展开'" :aria-label="composerExpanded ? '收起' : '展开'" @click.stop="composerExpanded = !composerExpanded; formatOpen = composerExpanded || formatOpen">
                <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" d="M14 5h5v5M10 19H5v-5M19 5l-6 6M5 19l6-6"/></svg>
              </button>
              <div class="send-split">
                <button type="submit" class="send-icon" :class="{ ready: !editorEmpty }" :disabled="!canCompose" aria-label="发送">
                  <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M4.2 11.2 19.5 4.3c.7-.3 1.4.4 1.1 1.1l-6.9 15.3c-.3.7-1.3.6-1.5-.1l-1.6-5.6-5.6-1.6c-.7-.2-.8-1.2-.1-1.5Z"/></svg>
                </button>
                <button type="button" class="send-caret" :disabled="!canCompose" aria-label="发送方式" @click.stop="toggleSendMenu">
                  <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M7 10h10l-5 5-5-5Z"/></svg>
                </button>
                <div v-if="sendMenu" class="pop-menu send-menu" @click.stop>
                  <button type="button" :class="{ on: !ctrlEnterSend }" @click="setSendMode(false)">Enter 发送</button>
                  <button type="button" :class="{ on: ctrlEnterSend }" @click="setSendMode(true)">Ctrl+Enter 发送</button>
                </div>
              </div>
              <input ref="imageInput" type="file" accept="image/jpeg,image/png,image/webp,image/gif" class="hidden" multiple @change="onPick" />
              <input ref="fileInput" type="file" class="hidden" multiple @change="onPick" />
            </div>
          </div>
        </form>
        </div>
        <aside v-if="membersOpen && store.active?.type === 'group'" class="members">
          <div class="members-head">
            <div>
              <strong>群成员</strong>
              <span>群号 {{ groupNo(store.active.id) }}</span>
            </div>
            <button type="button" class="btn" @click="membersOpen = false">关闭</button>
          </div>
          <div class="member-list">
            <button
              v-for="user in store.members"
              :key="user.id"
              type="button"
              class="member-row"
              @contextmenu.prevent="onMemberMenu($event, user)"
            >
              <span class="avatar sm" :class="pendantClass(user.pendant)">
                <img v-if="user.avatarUrl" :src="withSession(user.avatarUrl)" alt="" />
                <template v-else>{{ initial(user.nickname) }}</template>
              </span>
              <span class="member-copy">
                <span class="name">
                  {{ user.nickname }}
                  <em v-if="user.id === store.active.ownerId">群主</em>
                  <em v-else-if="user.id === store.me?.id">我</em>
                </span>
                <span class="sub">{{ user.dnd ? '免打扰' : (user.online ? '在线' : '离线') }}<template v-if="user.signature"> · {{ user.signature }}</template></span>
              </span>
            </button>
          </div>
        </aside>
      </section>
    </div>

    <div
      v-if="convMenu"
      class="conv-menu"
      :style="{ left: `${convMenu.x}px`, top: `${convMenu.y}px` }"
      @click.stop
    >
      <button type="button" @click="togglePin">{{ menuConv()?.pinned ? '取消置顶' : '置顶' }}</button>
      <button type="button" @click="toggleMute">{{ menuConv()?.muted ? '允许通知' : '消息免打扰' }}</button>
      <template v-if="menuConv()?.type === 'group'">
        <button type="button" @click="openMembersFromMenu">查看成员</button>
        <button v-if="isOwner(menuConv())" type="button" @click="addFromMenu">添加成员</button>
        <button type="button" @click="leaveFromMenu">退出群</button>
      </template>
    </div>

    <div
      v-if="memberMenu"
      class="conv-menu"
      :style="{ left: `${memberMenu.x}px`, top: `${memberMenu.y}px` }"
      @click.stop
    >
      <button v-if="menuMember()?.id !== store.me?.id" type="button" @click="messageMember">发消息</button>
      <button v-if="menuMember()?.id !== store.me?.id" type="button" :disabled="!canCompose" @click="mentionMember">@{{ menuMember()?.nickname }}</button>
      <button
        v-if="isOwner(store.active) && menuMember()?.id !== store.me?.id"
        type="button"
        @click="kickMember"
      >
        移出群
      </button>
    </div>

    <div v-if="historyOpen" class="mask" @click.self="historyOpen = false">
      <div class="card picker">
        <h2>聊天记录</h2>
        <input v-model="historyQuery" type="search" placeholder="搜索记录" aria-label="搜索记录" />
        <div class="hits">
          <p v-if="historyHits.length === 0" class="empty">没有记录</p>
          <div v-for="msg in historyHits" :key="msg.id" class="hit history-hit">
            <span class="name">{{ msg.senderNickname }}</span>
            <span class="time">{{ formatStamp(msg.createdAt) }}</span>
            <span class="preview">{{ msg.kind === 'text' ? plainText(msg.body) : (msg.fileName || '文件') }}</span>
          </div>
        </div>
        <div class="actions">
          <button type="button" class="btn" @click="historyOpen = false">关闭</button>
        </div>
      </div>
    </div>

    <div v-if="forwardMsg" class="mask" @click.self="forwardMsg = null">
      <div class="card picker">
        <h2>转发到</h2>
        <div class="hits">
          <button
            v-for="conv in store.conversations"
            :key="conv.id"
            type="button"
            class="hit"
            @click="sendForward(conv)"
          >
            <span class="name">{{ conv.title }}</span>
          </button>
          <p v-if="store.conversations.length === 0" class="empty">没有会话</p>
        </div>
        <div class="actions">
          <button type="button" class="btn" @click="forwardMsg = null">取消</button>
        </div>
      </div>
    </div>

    <div v-if="shotUrl" class="mask" @click.self="closeShot">
      <div class="card shot-card">
        <h2>截图</h2>
        <img class="shot" :src="shotUrl" alt="截图预览" />
        <div class="actions">
          <button type="button" class="btn" @click="closeShot">取消</button>
          <button type="button" class="btn primary" @click="sendShot">发送</button>
        </div>
      </div>
    </div>

    <div v-if="previewUrl" class="mask lightbox" @click="previewUrl = ''">
      <img :src="previewUrl" alt="图片" />
    </div>

    <div v-if="helpOpen" class="mask" @click.self="helpOpen = false">
      <div class="card help-card">
        <h2>帮助</h2>
        <dl>
          <div>
            <dt>账号</dt>
            <dd>用 Key 登录。每个浏览器窗口是独立账号，可用随机或一键注册。</dd>
          </div>
          <div>
            <dt>消息</dt>
            <dd>Enter 发送，Shift+Enter 换行。也可以改成 Ctrl+Enter 发送。输入栏里有表情、代码、截图、取色、图片和文件。</dd>
          </div>
          <div>
            <dt>好友</dt>
            <dd>在联系人里搜索昵称并申请。对方同意后即可发消息。</dd>
          </div>
          <div>
            <dt>群</dt>
            <dd>建群后会显示群标识和群号。点成员查看名单。右键会话或成员，可以添加、@ 或移出。</dd>
          </div>
          <div>
            <dt>免打扰</dt>
            <dd>设置里打开后，别人不能给你发私聊，群聊不受影响。会话上的消息免打扰只是不再提醒。</dd>
          </div>
          <div>
            <dt>使用问题</dt>
            <dd>有使用问题可以找诗文修改。</dd>
          </div>
        </dl>
        <div class="actions">
          <button type="button" class="btn primary" @click="helpOpen = false">知道了</button>
        </div>
      </div>
    </div>

    <div v-if="profileOpen" class="mask" @click.self="profileOpen = false">
      <form class="card settings" @submit.prevent="saveProfile">
        <div class="settings-head">
          <div>
            <h2>设置</h2>
            <p>头像、资料和消息偏好</p>
          </div>
          <button type="button" class="settings-close" aria-label="关闭" @click="profileOpen = false">×</button>
        </div>
        <div class="settings-body">
          <div class="settings-hero">
            <span class="avatar lg" :class="pendantClass(pendant)">
              <img v-if="selfAvatarSrc()" :src="selfAvatarSrc()" alt="" />
              <template v-else>{{ initial(nickname || store.me?.nickname || '') }}</template>
            </span>
            <div class="settings-id">
              <strong>{{ nickname || '未命名' }}</strong>
              <span>{{ signature.trim() || '还没有个性签名' }}</span>
              <button type="button" class="text-btn" :disabled="profileBusy" @click="avatarInput?.click()">更换头像</button>
            </div>
            <input ref="avatarInput" type="file" accept="image/jpeg,image/png,image/webp,image/gif" class="hidden" @change="onAvatar" />
          </div>

          <section class="settings-block">
            <h3>资料</h3>
            <label for="profile-nickname">昵称</label>
            <input id="profile-nickname" v-model="nickname" type="text" maxlength="32" />
            <label for="profile-sign">个性签名</label>
            <input id="profile-sign" v-model="signature" type="text" maxlength="60" placeholder="写一句介绍自己的话" />
            <span class="field-label">头像挂件</span>
            <div class="pendant-pick">
              <button
                v-for="item in pendants"
                :key="item.id || 'none'"
                type="button"
                class="pendant-option"
                :class="{ on: pendant === item.id }"
                @click="pendant = item.id"
              >
                <span class="avatar sm" :class="pendantClass(item.id)">{{ initial(nickname || store.me?.nickname || '') }}</span>
                {{ item.label }}
              </button>
            </div>
          </section>

          <section class="settings-block">
            <h3>账号</h3>
            <div class="info-row">
              <span>Key</span>
              <code>{{ store.me?.key ?? '' }}</code>
            </div>
            <div class="info-row">
              <span>本机 IP</span>
              <code>{{ store.me?.ip || '未知' }}</code>
            </div>
          </section>

          <section class="settings-block">
            <h3>消息</h3>
            <label class="switch-row">
              <span>
                <strong>免打扰</strong>
                <em>其他人不能给你发私聊。群聊不受影响。</em>
              </span>
              <input v-model="dnd" class="switch" type="checkbox" />
            </label>
            <label class="switch-row">
              <span>
                <strong>消息提醒</strong>
                <em>不在当前会话时，用系统通知提醒。</em>
              </span>
              <input v-model="notifyOn" class="switch" type="checkbox" />
            </label>
          </section>
          <p v-if="profileError" class="form-error">{{ profileError }}</p>
        </div>
        <div class="settings-foot">
          <button type="button" class="btn ghost" :disabled="profileBusy" @click="onLogout">退出登录</button>
          <button type="submit" class="btn primary" :disabled="profileBusy">保存</button>
        </div>
      </form>
    </div>

    <div v-if="picker" class="mask" @click.self="picker = null">
      <form class="card picker group-dialog" @submit.prevent="submitPicker">
        <div class="group-dialog-head">
          <span class="avatar group preview">
            {{ initial(groupName.trim() || '群') }}
            <i class="group-mark">群</i>
          </span>
          <div class="group-dialog-copy">
            <h2>{{ picker === 'create' ? '建群' : '添加成员' }}</h2>
            <p>{{ picker === 'create' ? '给群起个名字，再选要邀请的人' : '搜索并加入新成员' }}</p>
          </div>
        </div>
        <label v-if="picker === 'create'" for="group-name">群名</label>
        <input v-if="picker === 'create'" id="group-name" v-model="groupName" type="text" maxlength="40" placeholder="例如：周五复盘" />
        <label for="user-query">搜索用户</label>
        <input id="user-query" v-model="userQuery" type="search" placeholder="昵称" />
        <div v-if="selected.length" class="selected-row">
          <button v-for="user in selected" :key="user.id" type="button" class="selected-chip" @click="toggleUser(user)">
            <span class="avatar sm">{{ initial(user.nickname) }}</span>
            {{ user.nickname }}
            <i aria-hidden="true">×</i>
          </button>
        </div>
        <div class="hits people">
          <p v-if="searching" class="empty">加载中</p>
          <p v-else-if="userHits.length === 0" class="empty">没有匹配的用户</p>
          <button
            v-for="user in userHits"
            :key="user.id"
            type="button"
            class="person"
            :class="{ on: selectedHas(user.id) }"
            @click="toggleUser(user)"
          >
            <span class="avatar sm" :class="pendantClass(user.pendant)">
              <img v-if="user.avatarUrl" :src="withSession(user.avatarUrl)" alt="" />
              <template v-else>{{ initial(user.nickname) }}</template>
            </span>
            <span class="member-copy">
              <span class="name">{{ user.nickname }}</span>
              <span class="sub">{{ user.signature || (user.online ? '在线' : '离线') }}</span>
            </span>
            <span class="tick">{{ selectedHas(user.id) ? '已选' : '选择' }}</span>
          </button>
        </div>
        <p v-if="pickerError" class="form-error">{{ pickerError }}</p>
        <div class="actions">
          <span class="picked">已选 {{ selected.length }} 人</span>
          <button type="button" class="btn" @click="picker = null">取消</button>
          <button type="submit" class="btn primary" :disabled="pickerBusy">{{ picker === 'create' ? '创建群' : '添加' }}</button>
        </div>
      </form>
    </div>

    <div v-if="dropperOpen" class="mask" @click.self="closeDropper">
      <div class="card dropper-card">
        <div class="dropper-readout">
          <i :style="{ background: dropperHex }"></i>
          <div>
            <strong>{{ dropperHex }}</strong>
            <span>{{ dropperRgb }}</span>
          </div>
        </div>
        <p class="hint">在画面上移动查看色值，点击取用</p>
        <div class="dropper-stage" @mousemove="onDropperMove" @click="commitDropper">
          <video ref="dropperVideo" muted playsinline></video>
          <span v-if="loupe.on" class="loupe" :style="{ left: `${loupe.x}px`, top: `${loupe.y}px`, background: dropperHex }"></span>
        </div>
        <div class="actions">
          <button type="button" class="btn" @click="closeDropper">取消</button>
          <button type="button" class="btn primary" @click="commitDropper">使用 {{ dropperHex }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.shell,
.auth {
  height: 100vh;
  height: 100dvh;
  display: flex;
  flex-direction: column;
  background: #f3f5f7;
  color: #1f2329;
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
  font-size: 14px;
}
.shell {
  position: fixed;
  inset: 0;
  width: 100%;
  max-width: 100%;
  overflow: hidden;
  box-sizing: border-box;
  padding:
    env(safe-area-inset-top)
    env(safe-area-inset-right)
    env(safe-area-inset-bottom)
    env(safe-area-inset-left);
}
.shell.shaking {
  animation: feige-shake 0.45s linear;
}
@keyframes feige-shake {
  0%,
  100% { transform: translateX(0); }
  20% { transform: translateX(-8px); }
  40% { transform: translateX(8px); }
  60% { transform: translateX(-5px); }
  80% { transform: translateX(5px); }
}
.auth {
  align-items: center;
  justify-content: center;
}
.workspace {
  min-width: 0;
  min-height: 0;
  flex: 1;
  display: flex;
  overflow: hidden;
}
.hidden {
  display: none !important;
}
.notice {
  margin: 0;
  padding: 8px 16px;
  background: #fff7e8;
  color: #1f2329;
  font-size: 12px;
}
.rail {
  width: 68px;
  flex: none;
  min-height: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
  background: #fff;
  border-right: 1px solid #e6e8eb;
  overflow: hidden;
}
.rail-top,
.rail-nav {
  width: 100%;
  box-sizing: border-box;
  padding: 0 6px;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 4px;
}
.rail-nav {
  margin-top: 4px;
}
.rail-btn {
  box-sizing: border-box;
  width: 100%;
  height: 52px;
  margin: 0;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: #646a73;
  font: inherit;
  font-size: 12px;
  line-height: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  cursor: pointer;
}
.rail-btn svg {
  display: block;
  width: 20px;
  height: 20px;
  flex: none;
}
.rail-btn.active {
  background: #e7f0fa;
  color: #0b6bcb;
}
.me {
  cursor: pointer;
}
.me .avatar {
  width: 32px;
  height: 32px;
  font-size: 13px;
}
.rail-help {
  box-sizing: border-box;
  width: calc(100% - 12px);
  margin: auto 6px 0;
}
.help-card {
  max-width: 420px;
}
.help-card dl {
  margin: 12px 0 0;
}
.help-card div + div {
  margin-top: 12px;
}
.help-card dt {
  font-weight: 600;
}
.help-card dd {
  margin: 4px 0 0;
  color: #646a73;
  font-size: 13px;
  line-height: 1.5;
}
.presence {
  position: absolute;
  right: -1px;
  bottom: -1px;
  width: 8px;
  height: 8px;
  border: 2px solid #fff;
  border-radius: 50%;
  background: #c4c7cc;
}
.presence.on {
  background: #22a06b;
}
.list {
  width: 280px;
  flex: none;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-right: 1px solid #e6e8eb;
  overflow: hidden;
}
.list.grow {
  width: auto;
  flex: 1;
}
.search-wrap {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 12px 4px;
}
.latency {
  flex: none;
  margin-left: auto;
  height: 22px;
  padding: 0 8px;
  border-radius: 11px;
  background: #f3f5f7;
  color: #646a73;
  font-size: 12px;
  line-height: 22px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.latency.good {
  color: #22a06b;
}
.latency.slow {
  color: #b45309;
}
.search-wrap input,
.card input {
  box-sizing: border-box;
  width: 100%;
  height: 32px;
  border: 0;
  border-radius: 6px;
  background: #f3f5f7;
  padding: 0 10px;
  color: #1f2329;
  font: inherit;
  outline: none;
}
.search-wrap input:focus,
.card input:focus {
  box-shadow: 0 0 0 1px #0b6bcb inset;
}
.search-wrap input {
  width: auto;
  flex: 1;
  min-width: 0;
}
.create {
  flex-shrink: 0;
  margin: 4px 12px 0;
  height: 32px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #0b6bcb;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.scroll,
.msgs,
.hits {
  min-width: 0;
  min-height: 0;
  flex: 1;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
}
.scroll,
.msgs {
  display: flex;
  flex-direction: column;
}
.scroll > .empty,
.msgs > .empty {
  margin: auto;
}
.row,
.hit {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: 10px;
  border: 0;
  background: #fff;
  padding: 10px 12px;
  text-align: left;
  cursor: pointer;
  font: inherit;
  color: inherit;
}
.row:hover,
.hit:hover {
  background: #f7f8fa;
}
.row.active {
  background: #e7f0fa;
  box-shadow: inset 3px 0 0 #0b6bcb;
}
.contact-block + .contact-block .contact-head {
  padding-top: 16px;
}
.contact-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin: 0;
  padding: 12px 14px 2px;
  font-size: 12px;
  font-weight: 400;
}
.contact-head strong {
  color: #1f2329;
  font-weight: 600;
}
.contact-head span {
  color: #8a9099;
}
.contact-note {
  margin: 2px 14px 10px;
  color: #8a9099;
  font-size: 12px;
}
.contact {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  cursor: pointer;
}
.contact .row-body {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 2px;
}
.contact .name,
.contact .preview {
  flex: none;
  width: 100%;
}
.contact:hover {
  background: #f7f8fa;
}
.contact.drop {
  background: #e7f0fa;
}
.contact-acts {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: none;
}
.contact-act {
  flex: none;
  border: 0;
  background: transparent;
  color: #0b6bcb;
  font: inherit;
  font-size: 12px;
  line-height: 1;
  padding: 4px 0;
  cursor: pointer;
}
.contact-act.quiet {
  color: #8a9099;
}
.contact-act:hover:not(:disabled) {
  color: #0958a8;
}
.contact-act.quiet:hover:not(:disabled) {
  color: #1f2329;
}
.contact-act:disabled {
  color: #c4c7cc;
  cursor: default;
}
.row.drop,
.avatar.drop {
  outline: 1px solid #0b6bcb;
}
.avatar {
  position: relative;
  width: 36px;
  height: 36px;
  flex: none;
  border-radius: 50%;
  background: #e7f0fa;
  color: #0b6bcb;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: visible;
  font-size: 14px;
}
.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 50%;
}
.avatar.group {
  border-radius: 8px;
  background: #0b6bcb;
  color: #fff;
}
.avatar.group img {
  border-radius: 8px;
}
.avatar.sm {
  width: 28px;
  height: 28px;
  font-size: 12px;
}
.avatar.preview {
  width: 48px;
  height: 48px;
  font-size: 18px;
}
.group-mark {
  position: absolute;
  right: -4px;
  bottom: -4px;
  min-width: 14px;
  height: 14px;
  padding: 0 2px;
  border-radius: 4px;
  background: #fff;
  color: #0b6bcb;
  border: 1px solid #0b6bcb;
  font-size: 10px;
  font-style: normal;
  line-height: 12px;
  text-align: center;
}
.group-no {
  margin-left: 6px;
  color: #646a73;
  font-size: 12px;
  font-style: normal;
  font-weight: 400;
}
.member-strip {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  padding: 6px 12px;
  background: #fff;
  border-bottom: 1px solid #e6e8eb;
  overflow: hidden;
}
.member-strip .avatar {
  border: 0;
  padding: 0;
  cursor: pointer;
}
.member-strip .avatar.sm.owner {
  box-shadow: 0 0 0 2px #fff, 0 0 0 3px #0b6bcb;
}
.strip-more {
  height: 28px;
  border: 0;
  background: transparent;
  color: #646a73;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.members {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 5;
  width: 240px;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-left: 1px solid #e6e8eb;
  box-shadow: -8px 0 24px rgba(31, 35, 41, 0.08);
}
.members-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 12px;
  border-bottom: 1px solid #e6e8eb;
}
.members-head strong {
  display: block;
  font-size: 14px;
}
.members-head span {
  color: #646a73;
  font-size: 12px;
}
.member-list {
  min-height: 0;
  flex: 1;
  overflow: auto;
}
.member-row,
.person {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border: 0;
  border-bottom: 1px solid #e6e8eb;
  background: #fff;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.member-copy {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
}
.member-copy .name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
}
.member-copy .sub {
  color: #646a73;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.member-copy em {
  font-style: normal;
  font-weight: 400;
  font-size: 12px;
  color: #0b6bcb;
}
.avatar.pendant-ring {
  box-shadow: 0 0 0 2px #fff, 0 0 0 4px #0b6bcb;
}
.avatar.pendant-star::after,
.avatar.pendant-leaf::after,
.avatar.pendant-crown::after {
  position: absolute;
  top: -8px;
  right: -2px;
  font-size: 12px;
  line-height: 1;
}
.avatar.pendant-star::after { content: "★"; color: #f97316; }
.avatar.pendant-leaf::after { content: "🍃"; }
.avatar.pendant-crown::after { content: "♛"; color: #7c3aed; }
.avatar.lg {
  width: 48px;
  height: 48px;
  font-size: 18px;
}
.avatar img,
.photo {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.offline-avatar {
  filter: grayscale(1);
}
.row-body {
  min-width: 0;
  flex: 1;
}
.row-top,
.row-bottom,
.thread-head,
.profile-row,
.actions,
.tools,
.compose-row {
  display: flex;
  align-items: center;
}
.row-top,
.row-bottom {
  gap: 8px;
}
.name {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.time,
.preview,
.sub,
.stamp,
.who,
.muted,
.file-meta {
  color: #646a73;
  font-size: 12px;
}
.time {
  flex: none;
}
.preview {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.badge {
  flex: none;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 8px;
  background: #0b6bcb;
  color: #fff;
  font-size: 12px;
  line-height: 16px;
  text-align: center;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #22a06b;
  flex: none;
}
.empty {
  margin: 0;
  padding: 28px 12px;
  text-align: center;
  color: #646a73;
  font-size: 14px;
}
.thread {
  position: relative;
  min-width: 0;
  min-height: 0;
  max-width: 100%;
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: #f3f5f7;
}
.thread-main {
  min-width: 0;
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.thread.drag {
  background: #e7f0fa;
}
.thread-head {
  min-height: 56px;
  height: auto;
  flex-shrink: 0;
  flex-wrap: wrap;
  gap: 8px 10px;
  padding: 8px 12px;
  background: #fff;
  border-bottom: 1px solid #e6e8eb;
}
.thread-title {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
}
.thread-title .name {
  font-size: 16px;
  font-weight: 600;
}
.msgs {
  padding: 40px 20px 8px;
}
.pin {
  flex: none;
  color: #0b6bcb;
  font-size: 12px;
}
.mute {
  flex: none;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  border-radius: 9px;
  background: #e6e8eb;
  color: #646a73;
  font-size: 12px;
  line-height: 18px;
  text-align: center;
}
.fresh,
.recall {
  margin: 8px 0 16px;
  color: #646a73;
  font-size: 12px;
  text-align: center;
}
.msg {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  max-width: 100%;
  min-width: 0;
  margin-bottom: 16px;
}
.bubble-wrap {
  position: relative;
  max-width: 100%;
  min-width: 0;
}
.toolbar {
  position: absolute;
  top: -34px;
  right: 0;
  z-index: 2;
  display: none;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 4px 12px rgba(31, 35, 41, 0.08);
  white-space: nowrap;
}
.mine .toolbar {
  right: auto;
  left: 0;
}
.msg:hover .toolbar,
.toolbar:hover {
  display: flex;
}
.toolbar button,
.more button,
.emoji-panel button,
.mentions button,
.conv-menu button {
  height: 28px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: #1f2329;
  font: inherit;
  font-size: 12px;
  padding: 0 6px;
  cursor: pointer;
}
.toolbar button:hover,
.more button:hover,
.emoji-panel button:hover,
.mentions button:hover,
.conv-menu button:hover {
  background: #f3f5f7;
}
.more {
  position: absolute;
  top: 30px;
  right: 0;
  display: flex;
  flex-direction: column;
  min-width: 88px;
  padding: 4px;
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 8px 24px rgba(31, 35, 41, 0.08);
}
.quote {
  display: flex;
  gap: 6px;
  max-width: min(280px, 100%);
  min-width: 0;
  margin-bottom: 4px;
  padding: 4px 8px;
  border-left: 2px solid #0b6bcb;
  border-radius: 4px;
  background: rgba(11, 107, 203, 0.08);
  color: #646a73;
  font-size: 12px;
  overflow: hidden;
}
.quote span:last-child {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.quote-name {
  flex: none;
  color: #0b6bcb;
}
.code {
  max-width: 100%;
  min-width: 0;
  margin: 6px 0;
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  background: #f7f8fa;
  overflow: hidden;
}
.code-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  background: #f3f5f7;
  color: #646a73;
  font-size: 12px;
}
.code-bar button {
  height: 22px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: #0b6bcb;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.code-lang {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.code-lines {
  margin: 0;
  padding: 8px 12px 8px 2.6em;
  list-style: decimal;
  color: #1f2329;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}
.mine .code,
.mine .code-bar,
.mine .code-lang,
.mine .code-lines {
  color: #1f2329;
}
.mine .code-bar button {
  color: #0b6bcb;
}
.code-lines li {
  padding-left: 8px;
}
.code-lines li::marker {
  color: #8a919a;
}
.code-lines code {
  font: inherit;
}
.mention {
  color: #0b6bcb;
}
.mine .mention {
  color: #fff;
  text-decoration: underline;
}
.reacts {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 4px;
}
.react {
  height: 22px;
  border: 1px solid #e6e8eb;
  border-radius: 11px;
  background: #fff;
  color: #1f2329;
  font: inherit;
  font-size: 12px;
  padding: 0 8px;
  cursor: pointer;
}
.react.on {
  border-color: #0b6bcb;
  background: #e7f0fa;
  color: #0b6bcb;
}
.replying,
.mentions,
.emoji-panel {
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
  min-width: 0;
  margin: 4px 0;
  color: #646a73;
  font-size: 12px;
}
.replying span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.replying button {
  margin-left: auto;
}
.mentions,
.emoji-panel {
  flex-wrap: wrap;
  padding: 6px;
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  background: #fff;
}
.conv-menu {
  position: fixed;
  z-index: 20;
  display: flex;
  flex-direction: column;
  min-width: 120px;
  padding: 4px;
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 8px 24px rgba(31, 35, 41, 0.12);
}
.lightbox {
  background: rgba(31, 35, 41, 0.72);
}
.lightbox img {
  max-width: min(880px, 92vw);
  max-height: 86vh;
  border-radius: 8px;
}
.msg.mine {
  flex-direction: row-reverse;
}
.msg-body {
  max-width: min(560px, 72%);
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.msg.mine .msg-body {
  align-items: flex-end;
}
.who {
  margin: 0 0 4px;
}
.bubble {
  max-width: 100%;
  min-width: 0;
  margin: 0;
  padding: 8px 12px;
  box-shadow: 0 1px 2px rgba(31, 35, 41, 0.06);
  background: #f2f3f5;
  color: #1f2329;
  border-radius: 2px 8px 8px 8px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
  line-height: 1.5;
}
.mine .bubble {
  background: #0b6bcb;
  color: #fff;
  border-radius: 8px 2px 8px 8px;
}
.photo {
  width: auto;
  height: auto;
  max-width: min(320px, 100%);
  border-radius: 8px;
}
.file {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: min(240px, 100%);
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 10px 12px;
  border-radius: 8px;
  background: #fff;
  border: 1px solid #e6e8eb;
  text-decoration: none;
  color: inherit;
}
.file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stamp {
  margin: 4px 0 0;
}
.composer {
  position: relative;
  flex-shrink: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-top: 1px solid #e6e8eb;
  padding: 0 12px 12px;
}
.composer-resize {
  flex: none;
  height: 12px;
  margin: 0 -12px;
  cursor: ns-resize;
  touch-action: none;
}
.composer-resize::before {
  content: "";
  position: absolute;
  left: 50%;
  top: 4px;
  width: 32px;
  height: 3px;
  margin-left: -16px;
  border-radius: 2px;
  background: #c4c7cc;
}
.composer-resize:hover::before {
  background: #0b6bcb;
}
.composer.sized {
  min-height: 0;
}
.composer.sized .compose-main {
  flex: 1;
  min-height: 0;
  align-items: stretch;
}
.composer.sized .editor {
  height: auto;
  max-height: none;
  min-height: 36px;
}
.composer.sized .composer-icons {
  align-self: flex-end;
}
.composer.expanded {
  flex: 1 1 auto;
  min-height: 0;
  max-height: 70%;
}
.composer.expanded .compose-main {
  flex: 1;
  min-height: 0;
  align-items: stretch;
}
.composer.expanded .editor {
  min-height: 0;
  max-height: none;
}
.composer.expanded .composer-icons {
  align-self: flex-end;
}
.format,
.tools {
  flex-wrap: wrap;
  gap: 2px;
}
.format {
  display: flex;
  align-items: center;
  min-width: 0;
  margin-bottom: 4px;
  padding: 3px;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  background: #f7f8fa;
}
.fmt-select {
  position: relative;
  display: flex;
  align-items: center;
}
.fmt-select::after {
  content: "";
  position: absolute;
  right: 6px;
  width: 0;
  height: 0;
  border-left: 3px solid transparent;
  border-right: 3px solid transparent;
  border-top: 4px solid #646a73;
  pointer-events: none;
}
.format select {
  height: 26px;
  border: 0;
  border-radius: 4px;
  background: #fff;
  color: #1f2329;
  font: inherit;
  font-size: 12px;
  padding: 0 16px 0 8px;
  appearance: none;
}
.fmt-select.font select {
  width: 76px;
}
.fmt-select.size select {
  width: 54px;
}
.fmt-gap {
  width: 1px;
  height: 16px;
  margin: 0 3px;
  background: #e6e8eb;
  flex: none;
}
.fmt {
  width: 26px;
  height: 26px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: #1f2329;
  font: inherit;
  font-size: 13px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  cursor: pointer;
}
.fmt svg {
  width: 16px;
  height: 16px;
}
.fmt.italic {
  font-style: italic;
  font-weight: 600;
}
.fmt.under {
  text-decoration: underline;
}
.fmt.strike {
  text-decoration: line-through;
}
.fmt.on,
.fmt:hover:not(:disabled) {
  background: #fff;
  color: #0b6bcb;
}
.fmt:disabled {
  opacity: 0.4;
  cursor: default;
}
.fmt-pop {
  position: relative;
}
.color-btn {
  flex-direction: column;
  gap: 1px;
  font-weight: 700;
  line-height: 1;
}
.color-btn i,
.mark-bar {
  width: 14px;
  height: 3px;
  border-radius: 1px;
}
.mark-letter {
  background: #fde68a;
  border-radius: 2px;
  padding: 0 1px;
}
.swatch-panel {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 8;
  display: grid;
  grid-template-columns: repeat(3, 18px);
  gap: 6px;
  width: max-content;
  padding: 8px;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 8px 24px rgba(31, 35, 41, 0.12);
}
.swatch-hex {
  grid-column: 1 / -1;
  color: #646a73;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.02em;
}
.swatch {
  width: 18px;
  height: 18px;
  border: 1px solid rgba(31, 35, 41, 0.12);
  border-radius: 4px;
  padding: 0;
  cursor: pointer;
}
.swatch.on {
  outline: 2px solid #0b6bcb;
  outline-offset: 1px;
}
.rich-line {
  min-height: 1.5em;
}
.rich-list {
  margin: 0;
  padding-left: 1.25em;
}
ul.rich-list {
  list-style: disc;
}
ol.rich-list {
  list-style: decimal;
}
.ink-readout {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-variant-numeric: tabular-nums;
}
.ink-readout i {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 1px solid #e6e8eb;
}
.mentions,
.emoji-panel {
  position: absolute;
  left: 12px;
  right: 72px;
  bottom: calc(100% - 8px);
  z-index: 7;
  margin: 0;
  box-shadow: 0 8px 24px rgba(31, 35, 41, 0.12);
}
.mentions {
  flex-direction: column;
  align-items: stretch;
  max-height: 220px;
  overflow: auto;
}
.mentions button {
  margin-left: 0;
  text-align: left;
}
.emoji-panel {
  flex-direction: column;
  align-items: stretch;
  max-height: 280px;
  overflow: auto;
}
.emoji-label {
  margin: 2px 4px 4px;
  color: #8a9099;
  font-size: 12px;
}
.emoji-grid {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 2px;
}
.emoji-grid button {
  height: 32px;
  padding: 0;
  font-size: 20px;
  border-radius: 6px;
}
.tools button,
.btn {
  height: 32px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #646a73;
  font: inherit;
  padding: 0 10px;
  cursor: pointer;
}
.tools button:hover:not(:disabled),
.btn:hover:not(:disabled) {
  background: #f3f5f7;
}
.btn {
  border: 1px solid #e6e8eb;
  background: #fff;
  color: #1f2329;
}
.btn.primary,
.send {
  border: 0;
  background: #0b6bcb;
  color: #fff;
}
.btn.primary:hover:not(:disabled),
.send:hover:not(:disabled) {
  background: #0a61b8;
}
.btn.primary:active:not(:disabled),
.send:active:not(:disabled) {
  background: #0958a8;
}
button:disabled {
  opacity: 0.45;
  cursor: default;
}
.compose-main {
  display: flex;
  align-items: flex-end;
  gap: 4px;
  min-width: 0;
}
.editor {
  flex: 1;
  min-width: 0;
  width: auto;
  min-height: 36px;
  max-height: 120px;
  overflow-x: hidden;
  overflow-y: auto;
  border: 0;
  border-radius: 0;
  outline: none;
  padding: 8px 0;
  font: inherit;
  color: #1f2329;
  background: transparent;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}
.editor:focus {
  background: transparent;
}
.editor.empty:before {
  content: attr(data-placeholder);
  color: #8a9099;
  pointer-events: none;
}
.editor blockquote {
  margin: 4px 0;
  padding-left: 8px;
  border-left: 3px solid #c4c7cc;
  color: #646a73;
}
.editor a {
  color: #0b6bcb;
}
.draft-code {
  margin: 6px 0;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  background: #f7f8fa;
  overflow: hidden;
}
.draft-code-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  padding: 0 8px;
  color: #646a73;
  font-size: 12px;
}
.draft-code-bar select {
  border: 0;
  background: transparent;
  color: #1f2329;
  font: inherit;
}
.draft-spacer {
  flex: 1;
}
.draft-code-bar button {
  border: 0;
  background: transparent;
  color: #646a73;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  padding: 0;
}
.draft-code-scroll {
  display: flex;
  align-items: flex-start;
  background: #fff;
  border-top: 1px solid #eef0f2;
}
.draft-gutter {
  margin: 0;
  padding: 8px 0 8px 12px;
  min-width: 28px;
  list-style: decimal;
  color: #8a919a;
  font: 13px/1.6 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  user-select: none;
}
.draft-gutter li {
  padding-right: 8px;
}
.draft-code-body {
  flex: 1;
  min-width: 0;
  margin: 0;
  padding: 8px 10px;
  min-height: 1.6em;
  font: 13px/1.6 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  color: #1f2329;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  outline: none;
}
.draft-code-body[data-wrap="off"] {
  white-space: pre;
  overflow-x: auto;
}
.composer-icons {
  display: flex;
  align-items: center;
  gap: 2px;
  flex: none;
}
.icon-btn {
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #646a73;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  cursor: pointer;
}
.icon-btn.aa {
  width: auto;
  padding: 0 6px;
  font-size: 15px;
  font-weight: 600;
}
.icon-btn svg {
  width: 20px;
  height: 20px;
}
.icon-btn.on,
.icon-btn:hover:not(:disabled) {
  background: #f3f5f7;
  color: #1f2329;
}
.icon-pop,
.send-split {
  position: relative;
}
.pop-menu {
  position: absolute;
  right: 0;
  bottom: calc(100% + 6px);
  z-index: 8;
  display: flex;
  flex-direction: column;
  min-width: 132px;
  padding: 4px;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 8px 24px rgba(31, 35, 41, 0.12);
}
.pop-menu button {
  height: 32px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #1f2329;
  font: inherit;
  font-size: 13px;
  text-align: left;
  padding: 0 10px;
  cursor: pointer;
  white-space: nowrap;
}
.pop-menu button.on,
.pop-menu button:hover:not(:disabled) {
  background: #f3f5f7;
}
.send-split {
  display: flex;
  align-items: center;
  margin-left: 4px;
}
.send-icon,
.send-caret {
  height: 32px;
  border: 0;
  background: transparent;
  color: #c4c7cc;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}
.send-icon {
  width: 32px;
  border-radius: 6px;
}
.send-icon svg,
.send-caret svg {
  width: 20px;
  height: 20px;
}
.send-icon.ready {
  color: #0b6bcb;
}
.send-caret {
  width: 18px;
  color: #8a9099;
}
.send-menu {
  min-width: 168px;
}
.capture-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin: 4px 0;
  padding: 6px 10px;
  border-radius: 6px;
  background: #e7f0fa;
  color: #0b6bcb;
  font-size: 12px;
}
.capture-bar button {
  border: 0;
  background: #0b6bcb;
  color: #fff;
  border-radius: 4px;
  height: 24px;
  padding: 0 8px;
  font: inherit;
  cursor: pointer;
}
.send-side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  flex: none;
}
.send-mode {
  display: flex;
  align-items: center;
  gap: 4px;
  color: #646a73;
  font-size: 12px;
  white-space: nowrap;
}
.send {
  height: 32px;
  padding: 0 16px;
  border-radius: 6px;
  font: inherit;
  cursor: pointer;
}
.history-hit {
  align-items: flex-start;
  flex-wrap: wrap;
  cursor: default;
}
.history-hit .preview {
  flex-basis: 100%;
  white-space: normal;
}
.hidden-input,
.hidden {
  display: none;
}
.mask {
  position: fixed;
  inset: 0;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: rgba(31, 35, 41, 0.45);
}
.card {
  width: 100%;
  max-width: 400px;
  max-height: calc(100dvh - 32px - env(safe-area-inset-top) - env(safe-area-inset-bottom));
  overflow: auto;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 12px 32px rgba(31, 35, 41, 0.16);
  padding: 20px 20px 16px;
}
.banned-card h1,
.card h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}
.banned-card .muted {
  margin: 8px 0 0;
}
.banned-card .block {
  width: 100%;
  margin-top: 16px;
}
.profile-row {
  gap: 12px;
  margin: 16px 0 8px;
}
.card label {
  display: block;
  margin: 12px 0 6px;
  color: #646a73;
  font-size: 12px;
}
.card input[readonly] {
  color: #646a73;
}
.form-error {
  margin: 8px 0 0;
  font-size: 12px;
}
.actions {
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
.picker {
  max-height: 80vh;
  display: flex;
  flex-direction: column;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}
.chip {
  height: 24px;
  border: 0;
  border-radius: 6px;
  background: #e7f0fa;
  color: #0b6bcb;
  font: inherit;
  font-size: 12px;
  padding: 0 8px;
  cursor: pointer;
}
.hits {
  margin-top: 8px;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
}
.hit {
  border-bottom: 1px solid #e6e8eb;
}
.card.settings {
  max-width: 440px;
  display: flex;
  flex-direction: column;
  padding: 0;
  overflow: hidden;
}
.settings-head,
.settings-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  padding: 16px 20px;
}
.settings-head {
  border-bottom: 1px solid #e6e8eb;
}
.settings-head p,
.settings-id span,
.switch-row em,
.info-row span {
  margin: 2px 0 0;
  color: #646a73;
  font-size: 12px;
  font-style: normal;
  font-weight: 400;
}
.settings-close {
  width: 28px;
  height: 28px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #646a73;
  font: inherit;
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
}
.settings-close:hover {
  background: #f3f5f7;
  color: #1f2329;
}
.settings-body {
  min-height: 0;
  overflow: auto;
  padding: 16px 20px 8px;
}
.settings-hero {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 16px;
}
.settings .avatar.lg {
  width: 64px;
  height: 64px;
  font-size: 22px;
}
.settings-id {
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
}
.settings-id strong {
  font-size: 16px;
}
.text-btn {
  margin-top: 4px;
  border: 0;
  background: transparent;
  color: #0b6bcb;
  font: inherit;
  font-size: 12px;
  padding: 0;
  cursor: pointer;
}
.text-btn:disabled {
  opacity: 0.5;
  cursor: default;
}
.settings-block {
  padding-top: 14px;
  border-top: 1px solid #e6e8eb;
}
.settings-block h3,
.field-label {
  margin: 0;
  color: #646a73;
  font-size: 12px;
  font-weight: 600;
}
.settings-block h3 {
  margin-bottom: 8px;
}
.field-label {
  display: block;
  margin: 12px 0 8px;
}
.settings .pendant-pick {
  display: flex;
  gap: 8px;
}
.settings .pendant-option {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: #f7f8fa;
  color: #646a73;
  font: inherit;
  font-size: 12px;
  padding: 8px 4px;
  cursor: pointer;
}
.settings .pendant-option.on {
  border-color: #0b6bcb;
  color: #0b6bcb;
  background: #e7f0fa;
}
.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 36px;
}
.info-row code {
  min-width: 0;
  color: #1f2329;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.settings .switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 0;
  padding: 10px 0;
}
.settings .switch-row span {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.settings .switch-row strong {
  font-size: 14px;
  font-weight: 600;
}
.settings input.switch {
  appearance: none;
  width: 36px;
  height: 22px;
  flex: none;
  margin: 0;
  padding: 0;
  border: 0;
  border-radius: 11px;
  background: #e6e8eb;
  position: relative;
  cursor: pointer;
}
.settings input.switch::after {
  content: "";
  position: absolute;
  top: 2px;
  left: 2px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 1px 2px rgba(31, 35, 41, 0.2);
}
.settings input.switch:checked {
  background: #0b6bcb;
}
.settings input.switch:checked::after {
  transform: translateX(14px);
}
.settings input.switch:focus {
  box-shadow: none;
  outline: 2px solid #0b6bcb;
  outline-offset: 2px;
}
.settings-foot {
  border-top: 1px solid #e6e8eb;
}
.btn.ghost {
  border: 0;
  background: transparent;
  color: #e11d48;
}
.btn.ghost:hover:not(:disabled) {
  background: #fff1f2;
}
.hint {
  margin: 4px 0 0;
  color: #646a73;
  font-size: 12px;
}
.shot-card,
.dropper-card {
  max-width: 640px;
}
.group-dialog {
  max-width: 480px;
}
.group-dialog-head {
  display: flex;
  align-items: center;
  gap: 16px;
}
.avatar.preview .group-mark {
  right: 2px;
  bottom: 2px;
}
.group-dialog-copy p {
  margin: 4px 0 0;
  color: #646a73;
  font-size: 12px;
}
.selected-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}
.selected-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 32px;
  padding: 0 8px 0 2px;
  border: 0;
  border-radius: 16px;
  background: #e7f0fa;
  color: #0b6bcb;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.selected-chip i {
  font-style: normal;
}
.hits.people {
  max-height: 280px;
}
.person.on {
  background: #e7f0fa;
}
.tick {
  flex: none;
  color: #0b6bcb;
  font-size: 12px;
}
.picked {
  margin-right: auto;
  color: #646a73;
  font-size: 12px;
}
.dropper-readout {
  display: flex;
  align-items: center;
  gap: 12px;
}
.dropper-readout i {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  border: 1px solid #e6e8eb;
}
.dropper-readout strong {
  display: block;
  font-size: 20px;
  font-variant-numeric: tabular-nums;
}
.dropper-readout span {
  color: #646a73;
  font-size: 12px;
}
.dropper-stage {
  position: relative;
  margin-top: 8px;
  height: 280px;
  border-radius: 8px;
  overflow: hidden;
  background: #1f2329;
  cursor: crosshair;
}
.dropper-stage video {
  width: 100%;
  height: 100%;
  object-fit: contain;
  pointer-events: none;
}
.loupe {
  position: absolute;
  width: 18px;
  height: 18px;
  margin: -9px 0 0 -9px;
  border: 2px solid #fff;
  border-radius: 50%;
  box-shadow: 0 0 0 1px #1f2329;
  pointer-events: none;
}
.shot {
  display: block;
  width: 100%;
  max-height: 60vh;
  object-fit: contain;
  margin-top: 12px;
  border-radius: 8px;
  background: #f3f5f7;
}

@media (max-width: 900px) {
  .rail {
    width: 56px;
  }
  .list {
    width: 220px;
  }
}

@media (max-height: 520px) {
  .rail {
    width: 52px;
    padding: 4px 0;
  }
  .rail .avatar {
    width: 28px;
    height: 28px;
    font-size: 12px;
  }
  .rail-nav {
    margin-top: 4px;
  }
  .rail-top .rail-btn,
  .rail-help {
    height: 36px;
  }
  .rail-btn {
    height: 40px;
  }
  .search-wrap {
    padding: 6px 8px 2px;
  }
  .search-wrap input {
    height: 28px;
  }
  .row,
  .hit {
    padding: 6px 8px;
  }
  .thread-head {
    min-height: 40px;
    padding: 4px 8px;
  }
  .msgs {
    padding: 8px 10px 4px;
  }
  .composer {
    padding: 0 8px 6px;
  }
  .editor {
    min-height: 36px;
    max-height: 52px;
    padding: 4px 8px;
  }
  .fmt {
    width: 26px;
    height: 26px;
  }
  .format select,
  .tools button,
  .send {
    height: 26px;
  }
  .emoji-panel {
    max-height: 160px;
  }
  .composer-icons .icon-btn,
  .send-icon,
  .send-caret {
    height: 28px;
  }
  .send-side {
    gap: 4px;
  }
  .mask {
    align-items: flex-start;
    padding: 8px;
  }
  .card {
    padding: 12px;
  }
  .shot,
  .dropper-stage {
    max-height: 36vh;
    height: 28vh;
  }
  .members {
    width: 200px;
  }
}

@media (max-width: 719px) {
  .members {
    left: 0;
    width: auto;
  }
}

@media (orientation: landscape) and (max-height: 520px) {
  .list {
    width: 200px;
  }
}
</style>
