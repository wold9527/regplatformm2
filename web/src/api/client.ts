import axios from 'axios'

const client = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

// 请求拦截：附加 token
client.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers['X-Auth-Token'] = token
  }
  return config
})

// 响应拦截：401 跳转登录（排除登录页本身，避免死循环）
client.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401 && window.location.pathname !== '/') {
      localStorage.removeItem('token')
      window.location.href = '/'
    }
    return Promise.reject(err)
  }
)

export default client

// ── Init API（批量加载）──
export const initApi = {
  load: () => client.get('/init'),
}

// ── Auth API ──
export const authApi = {
  register: (username: string, password: string) =>
    client.post('/auth/register', { username, password }),
  login: (username: string, password: string) =>
    client.post('/auth/login', { username, password }),
  me: () => client.get('/auth/me'),
  logout: () => client.post('/auth/logout'),
}

// ── Task API ──
export const taskApi = {
  create: (data: { platform: string; target: number; threads?: number; mode?: string; proxy_id?: number }) =>
    client.post('/tasks', data),
  start: (id: number) => client.post(`/tasks/${id}/start`),
  stop: (id: number) => client.post(`/tasks/${id}/stop`),
  current: (platform?: string) =>
    client.get('/tasks/current', { params: { platform } }),
  history: () => client.get('/tasks/history'),
}

// ── Proxy API ──
export const proxyApi = {
  list: () => client.get('/proxies'),
  create: (data: { name: string; protocol: string; host: string; port: number; username?: string; password?: string }) =>
    client.post('/proxies', data),
  update: (id: number, data: { name: string; protocol: string; host: string; port: number; username?: string; password?: string }) =>
    client.put(`/proxies/${id}`, data),
  delete: (id: number) => client.delete(`/proxies/${id}`),
  test: (data: { proxy_id?: number; protocol?: string; host?: string; port?: number; username?: string; password?: string }) =>
    client.post('/proxies/test', data),
}

// ── Credit API ──
export const creditApi = {
  balance: () => client.get('/credits/balance'),
  history: () => client.get('/credits/history'),
  redeem: (code: string) => client.post('/credits/redeem', { code }),
  claimFreeTrial: () => client.post('/credits/free-trial'),
  purchase: (amount: number, platform?: string) => client.post('/credits/purchase', { amount, platform }),
}

// ── Result API ──
export const resultApi = {
  get: (taskId: number) => client.get(`/results/${taskId}`),
  export: (taskId: number) => client.get(`/results/${taskId}/export`, { responseType: 'blob' }),
  list: (platform?: string, page = 1, pageSize = 100) =>
    client.get('/results', { params: { platform, page, page_size: pageSize } }),
  archive: (platform?: string) => client.post('/results/archive', platform ? { platform } : {}),
  archivedCount: (platform?: string) => client.get('/results/archived', { params: { platform, count_only: 'true' } }),
  archived: (platform?: string, pageSize = -1) => client.get('/results/archived', { params: { platform, page_size: pageSize } }),
  // 软禁用：恢复被禁用的账号
  reEnable: (id: number) => client.post(`/results/${id}/re-enable`),
}

// ── Proxy Pool API（管理后台） ──
export const proxyPoolApi = {
  list: (params?: { page?: number; page_size?: number; filter?: string }) =>
    client.get('/admin/proxy-pool', { params }),
  stats: () => client.get('/admin/proxy-pool/stats'),
  add: (data: { name: string; protocol: string; host: string; port: number; username?: string; password?: string; country?: string }) =>
    client.post('/admin/proxy-pool', data),
  delete: (id: number) => client.delete(`/admin/proxy-pool/${id}`),
  batchDelete: (ids: number[]) => client.post('/admin/proxy-pool/batch-delete', { ids }),
  import: (proxies: string, protocol: string) =>
    client.post('/admin/proxy-pool/import', { proxies, protocol }),
  healthCheck: (ids?: number[]) =>
    client.post('/admin/proxy-pool/health-check', ids?.length ? { ids } : {}),
  purge: () => client.post('/admin/proxy-pool/purge'),
  reset: (id: number) => client.post(`/admin/proxy-pool/${id}/reset`),
  // 从 URL 抓取代理
  fetchURL: (url: string, protocol: string) =>
    client.post('/admin/proxy-pool/fetch-url', { url, protocol }),
}

// ── Card Pool API（管理后台） ──
export const cardPoolApi = {
  list: (params?: { page?: number; page_size?: number; filter?: string }) =>
    client.get('/admin/card-pool', { params }),
  stats: () => client.get('/admin/card-pool/stats'),
  add: (data: { name?: string; card_number: string; exp_month: number; exp_year: number; cvc: string; billing_name?: string; billing_email?: string; billing_country?: string; billing_city?: string; billing_line1?: string; billing_zip?: string; provider?: string }) =>
    client.post('/admin/card-pool', data),
  delete: (id: number) => client.delete(`/admin/card-pool/${id}`),
  batchDelete: (ids: number[]) => client.post('/admin/card-pool/batch-delete', { ids }),
  import: (cards: string, provider: string) =>
    client.post('/admin/card-pool/import', { cards, provider }),
  validate: (ids?: number[]) =>
    client.post('/admin/card-pool/validate', ids?.length ? { ids } : {}),
  purge: () => client.post('/admin/card-pool/purge'),
}

// ── Email API ──
export const emailApi = {
  fetchOTP: (email: string) => client.get('/email/otp', { params: { email } }),
}

// ── Announcement API ──
export const announcementApi = {
  list: () => client.get('/announcements'),
}

// ── Stats API ──
export const statsApi = {
  global: () => client.get('/stats/global'),
  latestCompletions: (after?: number) =>
    client.get('/stats/latest-completions', { params: { after } }),
  recentCompletions: () => client.get('/stats/recent-completions'),
}

// ── Admin API ──
export const adminApi = {
  users: (params?: { page?: number; page_size?: number; search?: string; sort_by?: string; sort_order?: string }) =>
    client.get('/admin/users', { params }),
  recharge: (userId: number, credits: number) =>
    client.post('/admin/credits/recharge', { user_id: userId, credits }),
  toggleAdmin: (userId: number) =>
    client.post(`/admin/users/${userId}/toggle-admin`),
  generateCodes: (count: number, credits: number, batchName?: string) =>
    client.post('/admin/codes', { count, credits, batch_name: batchName }),
  codes: (params?: { page?: number; page_size?: number; batch?: string }) =>
    client.get('/admin/codes', { params }),
  stats: () => client.get('/admin/stats'),
  settings: () => client.get('/admin/settings'),
  settingRaw: (key: string) => client.get('/admin/settings/raw', { params: { key } }),
  saveSetting: (key: string, value: string) =>
    client.post('/admin/settings', { key, value }),
  announcements: () => client.get('/admin/announcements'),
  createAnnouncement: (title: string, content: string) =>
    client.post('/admin/announcements', { title, content }),
  deleteAnnouncement: (id: number) =>
    client.delete(`/admin/announcements/${id}`),
  userDetail: (userId: number) =>
    client.get(`/admin/users/${userId}/detail`),
  dataStats: () => client.get('/admin/data-stats'),
  cleanup: (data: { days: number; clean_results: boolean; clean_tasks: boolean; clean_tx: boolean; clean_archived_only: boolean }) =>
    client.post('/admin/cleanup', data),
  runningTasks: () => client.get('/admin/running-tasks'),
  recentActivity: (params?: { page?: number; page_size?: number }) =>
    client.get('/admin/recent-activity', { params }),
  stopTask: (taskId: number) => client.post(`/admin/tasks/${taskId}/stop`),
  deleteTask: (taskId: number) => client.delete(`/admin/tasks/${taskId}`),
  sendNotification: (data: { user_id: number; title: string; content: string }) =>
    client.post('/admin/notifications', data),
  listNotifications: () => client.get('/admin/notifications'),
  deleteNotification: (id: number) => client.delete(`/admin/notifications/${id}`),
}

// ── Notification API ──
export const notificationApi = {
  list: () => client.get('/notifications'),
  markRead: (id: number | string) => client.patch(`/notifications/${id}/read`),
}

// ── HF Space API ──
export const hfSpaceApi = {
  listTokens: () => client.get('/admin/hf/tokens'),
  createToken: (data: { label: string; token: string }) => client.post('/admin/hf/tokens', data),
  deleteToken: (id: number) => client.delete(`/admin/hf/tokens/${id}`),
  validateToken: (id: number) => client.post(`/admin/hf/tokens/${id}/validate`),
  validateAllTokens: () => client.post('/admin/hf/tokens/validate-all'),
  listSpaces: (service?: string, page?: number, pageSize?: number, status?: string) => client.get('/admin/hf/spaces', { params: { service, page, page_size: pageSize, status } }),
  addSpace: (data: { service: string; url: string; repo_id: string; token_id: number }) => client.post('/admin/hf/spaces', data),
  deleteSpace: (id: number) => client.delete(`/admin/hf/spaces/${id}`),
  checkHealth: (service?: string) => client.post('/admin/hf/spaces/health', null, { params: { service } }),
  purgeSpaces: (service?: string) => client.post('/admin/hf/spaces/purge', null, { params: { service } }),
  deploySpaces: (data: { service: string; count: number; release_url: string; token_id?: number }) => client.post('/admin/hf/spaces/deploy', data),
  updateSpaces: (data: { service: string }) => client.post('/admin/hf/spaces/update', data),
  autoscale: (data: { service: string; target: number; dry_run: boolean }) => client.post('/admin/hf/autoscale', data),
  syncCF: (service?: string) => client.post('/admin/hf/sync-cf', null, { params: { service } }),
  overview: () => client.get('/admin/hf/overview'),
  discover: (defaultService?: string) => client.post('/admin/hf/discover', null, { params: { default_service: defaultService } }),
  redetect: () => client.post('/admin/hf/redetect'),
}
