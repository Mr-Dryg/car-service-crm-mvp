/**
 * Клиент REST API CRM.
 * Токен хранится в localStorage и автоматически подставляется в запросы.
 */
import { ref } from 'vue'

const TOKEN_KEY = 'sto_crm_token'
const USER_KEY = 'sto_crm_user'

/** Реактивный профиль пользователя: обновляется при входе и выходе. */
const userState = ref(readStoredUser())

// Экспорт наружу: потребители читают userState.value напрямую.
export { userState }

function readStoredUser() {
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw)
  } catch {
    return null
  }
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setSession(token, user) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
  userState.value = user
}

export function getUser() {
  return userState.value
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  userState.value = null
}

/** Ошибка API с полями валидации и HTTP-статусом. */
export class ApiError extends Error {
  constructor(message, status, fields) {
    super(message)
    this.status = status
    this.fields = fields || {}
  }
}

async function request(path, { method = 'GET', body } = {}) {
  const headers = {}
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  const res = await fetch(path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  const text = await res.text()
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = text
    }
  }

  if (!res.ok) {
    const message = (data && data.error) || `Ошибка запроса (${res.status})`
    const fields = (data && data.fields) || {}
    if (res.status === 401) clearSession()
    throw new ApiError(message, res.status, fields)
  }
  return data
}

export const api = {
  // Публичные
  branches: () => request('/api/branches'),
  createPublicOrder: (payload) => request('/api/orders/public', { method: 'POST', body: payload }),
  login: (phone, password) => request('/api/auth/login', { method: 'POST', body: { phone, password } }),

  // Клиент
  clientOrders: () => request('/api/client/orders'),
  cancelOrder: (id) => request(`/api/client/orders/${id}/cancel`, { method: 'POST' }),
  enRoute: (id) => request(`/api/client/orders/${id}/en-route`, { method: 'POST' }),

  // Менеджер
  managerOrders: (params = {}) => {
    const q = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== null && v !== '') q.set(k, v)
    })
    const query = q.toString()
    return request(`/api/manager/orders${query ? `?${query}` : ''}`)
  },
  managerToday: () => request('/api/manager/orders/today'),
  managerSlots: (date) => request(`/api/manager/slots?date=${encodeURIComponent(date)}`),
  changeStatus: (id, status) =>
    request(`/api/manager/orders/${id}/status`, { method: 'PATCH', body: { status } }),
  restoreOrder: (id, payload) => request(`/api/manager/orders/${id}/restore`, { method: 'POST', body: payload }),
  deleteOrder: (id) => request(`/api/manager/orders/${id}`, { method: 'DELETE' }),

  // Админ
  dashboard: () => request('/api/admin/dashboard'),
  createBranch: (address) => request('/api/admin/branches', { method: 'POST', body: { address } }),
  createManager: (payload) => request('/api/admin/managers', { method: 'POST', body: payload }),
}
