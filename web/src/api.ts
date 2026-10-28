import axios from 'axios'

const http = axios.create({
  baseURL: '',
  withCredentials: true,
})

const keyPattern = /^[A-Za-z0-9_-]{8,64}$/

export const keyRuleMessage = 'Key 须为 8–64 位字母、数字、下划线或连字符'

// 与后端 NormalizeKey 相同：先去掉首尾空格，再检查长度和字符。
export function checkedKey(raw: string): string | null {
  const key = raw.trim()
  if (!keyPattern.test(key)) {
    return null
  }
  return key
}

type ErrorBody = {
  error?: string
}

export function apiErrorMessage(err: unknown): string {
  if (axios.isAxiosError<ErrorBody>(err)) {
    const message = err.response?.data?.error
    if (typeof message === 'string' && message !== '') {
      return message
    }
  }
  return '请求失败'
}

export async function login(key: string, nickname: string) {
  const response = await http.post('/api/session', { key, nickname })
  return response.data
}

export type Me = {
  id: string
  key: string
  nickname: string
  avatarUrl: string
  canSendMessage: boolean
  canSendFile: boolean
  canCreateGroup: boolean
}

export type PublicUser = {
  id: string
  nickname: string
  avatarUrl: string
  online: boolean
}

export type Conversation = {
  id: string
  type: string
  ownerId: string
  title: string
  peer?: PublicUser
  memberCount: number
  unread: number
  lastBody: string
  lastAt: string
}

export type ChatMessage = {
  id: string
  conversationId: string
  senderId: string
  senderNickname: string
  kind: string
  body: string
  fileId: string
  fileName: string
  fileSize: number
  mime: string
  createdAt: string
}

export type UploadedFile = {
  id: string
  name: string
  size: number
  mime: string
}

const imageMimes = new Set(['image/jpeg', 'image/png', 'image/webp', 'image/gif'])

// 图片消息只认这四种 MIME，其余上传结果按文件消息发送。
export function isImageMime(mime: string): boolean {
  const base = mime.split(';', 1)[0]?.trim().toLowerCase() ?? ''
  return imageMimes.has(base)
}

export async function me(): Promise<Me> {
  const response = await http.get<Me>('/api/me')
  return response.data
}

export async function logout() {
  await http.post('/api/logout')
}

export async function onlineUsers(): Promise<PublicUser[]> {
  const response = await http.get<PublicUser[]>('/api/users/online')
  return response.data ?? []
}

export async function searchUsers(q: string): Promise<PublicUser[]> {
  const response = await http.get<PublicUser[]>('/api/users', { params: { q } })
  return response.data ?? []
}

export async function conversations(): Promise<Conversation[]> {
  const response = await http.get<Conversation[]>('/api/conversations')
  return response.data ?? []
}

export async function openDirect(userId: string): Promise<string> {
  const response = await http.post<{ id: string }>('/api/conversations/direct', { userId })
  return response.data.id
}

export async function createGroup(name: string, memberIds: string[]): Promise<string> {
  const response = await http.post<{ id: string }>('/api/groups', { name, memberIds })
  return response.data.id
}

export async function addMembers(id: string, memberIds: string[]) {
  await http.post(`/api/groups/${encodeURIComponent(id)}/members`, { memberIds })
}

export async function leave(id: string) {
  await http.post(`/api/groups/${encodeURIComponent(id)}/leave`)
}

export async function messages(id: string, after?: string): Promise<ChatMessage[]> {
  const response = await http.get<ChatMessage[]>(
    `/api/conversations/${encodeURIComponent(id)}/messages`,
    { params: after ? { after } : {} },
  )
  return response.data ?? []
}

export async function markRead(id: string, messageId: string) {
  await http.post(`/api/conversations/${encodeURIComponent(id)}/read`, { messageId })
}

export async function upload(file: File): Promise<UploadedFile> {
  const form = new FormData()
  form.append('file', file)
  const response = await http.post<UploadedFile>('/api/files', form)
  return response.data
}

export async function patchMe(nickname: string): Promise<Me> {
  const response = await http.patch<Me>('/api/me', { nickname })
  return response.data
}

export async function uploadAvatar(file: File): Promise<Me> {
  const form = new FormData()
  form.append('file', file)
  const response = await http.post<Me>('/api/me/avatar', form)
  return response.data
}

export async function adminLogin(username: string, password: string) {
  const response = await http.post('/api/admin/session', { username, password })
  return response.data
}
