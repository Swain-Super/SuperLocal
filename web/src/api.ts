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

export async function me() {
  const response = await http.get('/api/me')
  return response.data
}

export async function adminLogin(username: string, password: string) {
  const response = await http.post('/api/admin/session', { username, password })
  return response.data
}
