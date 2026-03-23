/**
 * 核心仪表盘状态 — 模块级单例
 * 所有组件共享同一份 reactive 状态，避免 prop drilling
 */
import { ref, reactive, computed, watch } from 'vue'
import JSZip from 'jszip'
import {
  authApi, taskApi, creditApi, resultApi, emailApi,
  announcementApi, initApi, statsApi,
} from '../api/client'

// ══════════ 完成通知队列（首页右下角 toast） ══════════

interface CompletionToast {
  id: number
  taskId: number
  username: string
  platform: string
  success: number
  time: string
}

const completionQueue = ref<CompletionToast[]>([])
const activeCompletion = ref<CompletionToast | null>(null)
const completionVisible = ref(false)
let completionLastTs = Math.floor(Date.now() / 1000)
let completionSeenIds = new Set<number>()
let completionPollTimer: ReturnType<typeof setInterval> | null = null
let completionShowTimer: ReturnType<typeof setTimeout> | null = null
let completionAnimTimer: ReturnType<typeof setTimeout> | null = null
let completionToastId = 0

// ══════════ 全站动态（LogPanel 公共展示） ══════════

interface RecentCompletion {
  task_id: number
  username: string
  name?: string        // 昵称（优先显示）
  avatar_url?: string  // 头像 URL（可选，无则显示默认头像）
  platform: string
  success_count: number
  target_count: number
  duration_sec: number
  stopped_at: number
}

const recentCompletions = ref<RecentCompletion[]>([])

// 按用户去重，同一用户只保留最新一条
function dedupeByUser(items: RecentCompletion[]): RecentCompletion[] {
  const seen = new Set<string>()
  return items.filter(item => {
    const key = item.username || `__task_${item.task_id}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

async function loadRecentCompletions() {
  try {
    const { data } = await statsApi.recentCompletions()
    recentCompletions.value = dedupeByUser(data.items || [])
  } catch { /* ignore */ }
}

function processCompletionQueue() {
  if (activeCompletion.value || completionQueue.value.length === 0) return
  activeCompletion.value = completionQueue.value.shift()!
  completionVisible.value = true
  completionShowTimer = setTimeout(() => {
    completionVisible.value = false
    // 等动画结束后清除当前，显示下一个
    completionAnimTimer = setTimeout(() => {
      activeCompletion.value = null
      processCompletionQueue()
    }, 400)
  }, 4000)
}

async function pollLatestCompletions() {
  try {
    const { data } = await statsApi.latestCompletions(completionLastTs)
    const items = data.items || []
    if (items.length === 0) return
    // 更新时间戳
    for (const item of items) {
      if (item.stopped_at > completionLastTs) completionLastTs = item.stopped_at
    }
    // 去重：只添加未见过的
    let hasNew = false
    for (const item of items) {
      if (completionSeenIds.has(item.task_id)) continue
      hasNew = true
      completionSeenIds.add(item.task_id)
      // 限制 seen set 大小
      if (completionSeenIds.size > 200) {
        const arr = [...completionSeenIds]
        completionSeenIds = new Set(arr.slice(-100))
      }
      const platformLabel = ({ grok: 'Grok', openai: 'OpenAI', kiro: 'Kiro', gemini: 'Gemini' } as Record<string, string>)[item.platform] || item.platform
      const now = new Date()
      const timeStr = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}`
      completionQueue.value.push({
        id: ++completionToastId,
        taskId: item.task_id,
        username: item.username,
        platform: platformLabel,
        success: item.success_count,
        time: timeStr,
      })
    }
    processCompletionQueue()
    // 有新完成任务时刷新全站动态面板
    if (hasNew) loadRecentCompletions()
  } catch { /* ignore */ }
}

function startCompletionPolling() {
  completionLastTs = Math.floor(Date.now() / 1000)
  completionSeenIds.clear()
  pollLatestCompletions()
  if (completionPollTimer) clearInterval(completionPollTimer)
  completionPollTimer = setInterval(pollLatestCompletions, 5000)
}

function stopCompletionPolling() {
  if (completionPollTimer) { clearInterval(completionPollTimer); completionPollTimer = null }
  if (completionShowTimer) { clearTimeout(completionShowTimer); completionShowTimer = null }
  if (completionAnimTimer) { clearTimeout(completionAnimTimer); completionAnimTimer = null }
  completionQueue.value = []
  activeCompletion.value = null
  completionVisible.value = false
}

// ══════════ 响应式状态 ══════════

const user = reactive({
  id: 0, username: '', name: '', credits: 0, role: 1,
  avatar_url: '', is_admin: false, quota: 0,
  balance_display: '$0.00', registrations_available: 0,
  unit_price: 0, unit_price_display: '$0',
  newapi_balance: 0, newapi_balance_display: '$0',
  newapi_available: 0, mode: 'local',
  platform_prices: {} as Record<string, number>,
})

const limits = reactive({
  max_target: 0, max_threads: 0,
  daily_reg_limit: 0, daily_used: 0, daily_remaining: -1,
  platform_grok_enabled: true, platform_openai_enabled: true, platform_kiro_enabled: true, platform_gemini_enabled: true,
  thread_tiers: '' as string,
  platform_grok_free_until: '', platform_openai_free_until: '', platform_kiro_free_until: '', platform_gemini_free_until: '',
  free_mode: null as Record<string, any> | null,
  platform_limits: null as Record<string, any> | null,
})

const taskForm = reactive({ platform: 'grok', target_count: 5, thread_count: 2 })
const taskMode = ref<'free' | 'paid'>('free')
const taskStatus = reactive({
  success_count: 0, fail_count: 0, target: 0, threads: 0,
  credits_reserved: 0,
  is_done: true, task_id: null as number | null, platform: 'grok',
})
const isRunning = ref(false)
const isQueued = ref(false)
const elapsedSeconds = ref(0)
const processing = ref(false)
const logs = ref<string[]>([])
const logPanel = ref<HTMLElement>()
const lastLogAt = ref(0) // 最后一条日志到达的时间戳（ms），用于前端活跃指示器

const results = ref<any[]>([])
const platformArchivedCount = reactive<Record<string, number>>({ grok: 0, openai: 0, kiro: 0, gemini: 0 })
const archivedCountLoaded = ref(false)
const lastArchivedCount = reactive<Record<string, number>>({ grok: 0, openai: 0, kiro: 0, gemini: 0 })
const otpLoading = reactive<Record<string, boolean>>({})
const txHistory = ref<any[]>([])
const txHistoryLoaded = ref(false)
const txDisplayCount = ref(10)
const announcements = ref<any[]>([])
const freeTrial = reactive({ eligible: false, remaining: 0, total: 0, claimed: false })
const smartInput = ref('')
const purchaseProcessing = ref(false)
const exportingMap = reactive<Record<string, boolean>>({})
const exporting = computed(() => Object.values(exportingMap).some(Boolean))
const toasts = ref<{ id: number; msg: string; type: string }[]>([])
let toastId = 0
const taskNotice = reactive({ msg: '', type: 'info', time: '' })
const queueModal = reactive({ show: false, message: '', position: 0, waitTime: '' })

const globalStats = reactive({
  running_tasks: 0, queued_tasks: 0, active_users: 0,
  today_total: 0, today_platforms: {} as Record<string, number>,
  today_success: 0, today_fail: 0,
  success_rate: 0, total_results: 0,
})
const userStats = reactive({ total_success: 0, total_fail: 0 })
const byPlatformStats = reactive<Record<string, { success: number; fail: number }>>({})
const avgSecPerReg = reactive<Record<string, number>>({})

// ══════════ Computed ══════════

const smartInputMode = computed(() => {
  const v = smartInput.value.trim()
  return v && /^\d+$/.test(v) && user.mode === 'newapi' ? 'purchase' : 'redeem'
})
const smartInputAmount = computed(() => {
  if (smartInputMode.value !== 'purchase') return 0
  return parseInt(smartInput.value.trim()) || 0
})

const platformLabel = computed(() => ({ grok: 'Grok', openai: 'OpenAI', kiro: 'Kiro', gemini: 'Gemini' }[taskForm.platform] || 'Grok'))
const platformUserStats = computed(() => byPlatformStats[taskForm.platform] || { success: 0, fail: 0 })

const sliderMax = computed(() => {
  let cap = limits.max_target || 1000

  // 平台独立限制（付费/免费均生效）
  const pl = limits.platform_limits?.[taskForm.platform]
  if (pl) {
    if (pl.task_limit > 0) cap = Math.min(cap, pl.task_limit)
    if (pl.daily_remaining >= 0) cap = Math.min(cap, pl.daily_remaining)
  }

  if (currentPlatformFree.value && taskMode.value === 'free') {
    const taskLimit = currentFreeMode.value?.task_limit
    if (taskLimit && taskLimit > 0) cap = Math.min(cap, taskLimit)
    return Math.max(1, cap)
  }
  const available = (user.registrations_available || 0) + (freeTrial.remaining || 0)
  if (available > 0) cap = Math.min(cap, available)
  return Math.max(1, cap)
})

const quickTargets = computed(() => {
  const max = sliderMax.value
  const base = [1, 10, 50, 100, 500, 1000].filter(q => q <= max)
  // 确保 sliderMax 本身在列表中（如上限是 2 时显示 [1, 2]）
  if (max > 0 && !base.includes(max)) base.push(max)
  return base
})

const estimatedTime = computed(() => {
  const avg = avgSecPerReg[taskForm.platform]
  if (!avg || avg <= 0) return ''
  const threads = taskForm.thread_count || 1
  const totalSec = Math.round((avg * taskForm.target_count) / threads)
  if (totalSec < 60) return `~${totalSec}秒`
  const min = Math.floor(totalSec / 60)
  const sec = totalSec % 60
  return sec > 0 ? `~${min}分${sec}秒` : `~${min}分`
})

const estimatedRemaining = computed(() => {
  if (!isRunning.value || elapsedSeconds.value <= 0 || taskStatus.success_count <= 0) return ''
  const secPerReg = elapsedSeconds.value / taskStatus.success_count
  const remaining = Math.max(0, taskStatus.target - taskStatus.success_count)
  const remSec = Math.round(secPerReg * remaining)
  if (remSec < 60) return `剩余 ~${remSec}秒`
  const min = Math.floor(remSec / 60)
  const sec = remSec % 60
  return sec > 0 ? `剩余 ~${min}分${sec}秒` : `剩余 ~${min}分`
})

const currentPlatformEnabled = computed(() => {
  const key = `platform_${taskForm.platform}_enabled` as keyof typeof limits
  return limits[key] !== false
})

const currentPlatformFree = computed(() => isPlatformFreeByKey(taskForm.platform))
const currentFreeMode = computed(() => limits.free_mode?.[taskForm.platform] || null)
const currentFreeModeAvailable = computed(() => currentFreeMode.value?.available ?? false)

const currentPlatformPrice = computed(() => user.platform_prices[taskForm.platform] || user.unit_price || 0)
const currentPlatformPriceDisplay = computed(() => {
  const price = currentPlatformPrice.value
  return price > 0 ? `$${price}` : ''
})

// 当前平台独立限制（付费/免费均生效）
const currentPlatformLimit = computed(() => limits.platform_limits?.[taskForm.platform] || null)
// 当前平台每日注册状态文本（如 "82/3000"，不含后缀）
const currentPlatformDailyText = computed(() => {
  const pl = currentPlatformLimit.value
  if (!pl || pl.daily_limit <= 0) {
    // 回退全局每日限制
    if (limits.daily_reg_limit > 0) {
      return `${limits.daily_used}/${limits.daily_reg_limit}`
    }
    return ''
  }
  return `${pl.daily_used}/${pl.daily_limit}`
})
// 当前平台有效的每日上限（用于上限卡片显示）
const currentPlatformMaxDisplay = computed(() => {
  const pl = currentPlatformLimit.value
  // 优先平台级每日上限，回退全局每日上限
  if (pl?.daily_limit && pl.daily_limit > 0) return pl.daily_limit
  if (limits.daily_reg_limit > 0) return limits.daily_reg_limit
  return 0  // 0 = 不限
})

// sliderMax 变化时自动 clamp target_count（切平台、限制变更等）
watch(sliderMax, (max) => {
  if (taskForm.target_count > max) taskForm.target_count = max
  if (taskForm.target_count < 1) taskForm.target_count = 1
})

const platformTitleClass = computed(() => ({
  grok: 'from-blue-400 to-cyan-400',
  openai: 'from-green-400 to-emerald-400',
  kiro: 'from-amber-400 to-orange-400',
  gemini: 'from-purple-400 to-fuchsia-400',
}[taskForm.platform] || 'from-blue-400 to-cyan-400'))

const refundAmount = computed(() => {
  if (!taskStatus.is_done || isRunning.value) return 0
  const reserved = taskStatus.credits_reserved || taskStatus.target
  return Math.max(0, reserved - taskStatus.success_count)
})

const vipLabel = computed(() => vipLabelFor(user.role))
const vipBadgeClass = computed(() => vipBadgeClassFor(user.role))
const vipCardGlow = computed(() => ({ 10: 'card-glow-vvip', 100: 'card-glow-svip' }[user.role as 10 | 100] || ''))
const vipAvatarRing = computed(() => ({ 1: 'avatar-ring-normal', 10: 'avatar-ring-vvip', 100: 'avatar-ring-svip' }[user.role as 1 | 10 | 100] || 'avatar-ring-normal'))

// 平台列配置（模板渲染用）
const platformCols = [
  { key: 'grok',   label: 'Grok',   dotClass: 'bg-blue-500',   textClass: 'text-info',      hoverClass: 'text-info',      exportClass: 'text-info',      archiveBadge: 'bg-blue-500/15 text-info',      countBadgeClass: 'bg-blue-500/15 text-info' },
  { key: 'openai', label: 'OpenAI', dotClass: 'bg-green-500',  textClass: 'text-ok',        hoverClass: 'text-ok',        exportClass: 'text-ok',        archiveBadge: 'bg-green-500/15 text-ok',       countBadgeClass: 'bg-green-500/15 text-ok' },
  { key: 'kiro',   label: 'Kiro',   dotClass: 'bg-amber-500',  textClass: 'text-warn',      hoverClass: 'text-warn',      exportClass: 'text-warn',      archiveBadge: 'bg-amber-500/15 text-warn',     countBadgeClass: 'bg-amber-500/15 text-warn' },
  { key: 'gemini', label: 'Gemini', dotClass: 'bg-purple-500', textClass: 'text-purple-400', hoverClass: 'text-purple-400', exportClass: 'text-purple-400', archiveBadge: 'bg-purple-500/15 text-purple-400', countBadgeClass: 'bg-purple-500/15 text-purple-400' },
]

const platformResults = computed<Record<string, any[]>>(() => ({
  grok: results.value.filter(r => (r.platform || 'grok') === 'grok'),
  openai: results.value.filter(r => r.platform === 'openai'),
  kiro: results.value.filter(r => r.platform === 'kiro'),
  gemini: results.value.filter(r => r.platform === 'gemini'),
}))

// 按需渲染：每平台默认只渲染前 N 条，防止 DOM 过多卡顿
const platformDisplayLimit = reactive<Record<string, number>>({ grok: 20, openai: 20, kiro: 20, gemini: 20 })
const displayedPlatformResults = computed<Record<string, any[]>>(() => ({
  grok: platformResults.value.grok.slice(0, platformDisplayLimit.grok),
  openai: platformResults.value.openai.slice(0, platformDisplayLimit.openai),
  kiro: platformResults.value.kiro.slice(0, platformDisplayLimit.kiro),
  gemini: platformResults.value.gemini.slice(0, platformDisplayLimit.gemini),
}))
function showMoreResults(platform: string) {
  platformDisplayLimit[platform] += 30
}

// ══════════ Helpers ══════════

function vipLabelFor(r: number) { return ({ 1: '用户', 10: '管理员', 100: '超管' } as any)[r] || '用户' }
function vipBadgeClassFor(r: number) { return ({ 1: 'badge-vip', 10: 'badge-vvip', 100: 'badge-svip' } as any)[r] || 'badge-normal' }

function isPlatformFreeByKey(platform: string): boolean {
  const key = `platform_${platform}_free_until` as keyof typeof limits
  const dateStr = limits[key] as string
  if (!dateStr) return false
  return new Date() <= new Date(dateStr + 'T23:59:59')
}

function platformHealthRate(platform: string): number {
  const ps = byPlatformStats[platform]
  if (!ps || (ps.success + ps.fail) === 0) return -1
  return (ps.success / (ps.success + ps.fail)) * 100
}

function platformHealthColor(platform: string): string {
  const rate = platformHealthRate(platform)
  if (rate < 0) return 'bg-t-faint'
  if (rate >= 80) return 'bg-green-500'
  if (rate >= 50) return 'bg-amber-500'
  return 'bg-red-500'
}

function toast(msg: string, type = 'info') {
  if (toasts.value.some(t => t.msg === msg)) return
  const id = ++toastId
  toasts.value.push({ id, msg, type })
  setTimeout(() => { toasts.value = toasts.value.filter(t => t.id !== id) }, 3000)
}

function setTaskNotice(msg: string, type = 'info') {
  taskNotice.msg = msg; taskNotice.type = type
  taskNotice.time = new Date().toLocaleTimeString()
}

function clearTaskNotice() { taskNotice.msg = ''; taskNotice.type = 'info'; taskNotice.time = '' }

function getPassword(r: any): string {
  const cred = r.credential_data
  return (cred && typeof cred === 'object') ? cred.password || '' : ''
}

function copyText(text: string) {
  navigator.clipboard.writeText(text)
    .then(() => toast('已复制', 'success'))
    .catch(() => toast('复制失败', 'error'))
}

function copyEmailPassword(r: any) {
  const cred = r.credential_data
  const email = cred?.email || r.email || ''
  const password = cred?.password || ''
  copyText(`${email}----${password}`)
}

function formatElapsed(secs: number): string {
  const m = Math.floor(secs / 60)
  const s = secs % 60
  return m > 0 ? `${m}分${s.toString().padStart(2, '0')}秒` : `${s}秒`
}

// ══════════ API 调用 ══════════

async function loadUser() {
  try { const { data } = await authApi.me(); Object.assign(user, data) }
  catch { window.location.href = '/' }
}

async function loadBalance() {
  try {
    const { data } = await creditApi.balance()
    const costPerReg = data.cost_per_reg || 1
    user.quota = data.quota || 0
    user.balance_display = data.display || ''
    user.registrations_available = data.registrations_available ?? Math.floor((data.credits || 0) / costPerReg)
    user.unit_price = data.unit_price || 0
    user.unit_price_display = data.unit_price_display || ''
    user.newapi_balance = data.newapi_balance || 0
    user.newapi_balance_display = data.newapi_balance_display || '$0'
    user.newapi_available = data.newapi_available || 0
    user.mode = data.mode || 'local'
    user.platform_prices = data.platform_prices || {}
    Object.assign(limits, data.limits || {})
    // 当前选中平台被关闭时自动切换
    const currentKey = `platform_${taskForm.platform}_enabled` as keyof typeof limits
    if (limits[currentKey] === false) {
      const first = (['grok', 'openai', 'kiro', 'gemini'] as const).find(
        p => limits[`platform_${p}_enabled` as keyof typeof limits] !== false
      )
      if (first) taskForm.platform = first
    }
    if (data.free_trial) {
      freeTrial.eligible = data.free_trial.eligible
      freeTrial.total = data.free_trial.total || data.free_trial.remaining
      if (!freeTrial.claimed) freeTrial.remaining = data.free_trial.remaining
    } else if (data.free_trial_remaining !== undefined) {
      freeTrial.eligible = !data.free_trial_used
      if (!freeTrial.claimed) freeTrial.remaining = data.free_trial_remaining || 0
      freeTrial.total = freeTrial.total || freeTrial.remaining
    }
  } catch { /* ignore */ }
}

async function loadResults() {
  try {
    const { data } = await resultApi.list(undefined, 1, -1)
    results.value = data.items || data || []
  } catch { /* ignore */ }
}

async function refreshResults() { await loadResults(); toast('已刷新', 'success') }

async function loadArchivedCount(platform: string) {
  try {
    const { data } = await resultApi.archivedCount(platform)
    platformArchivedCount[platform] = data.total || 0
  } catch { /* ignore */ }
}

async function loadAllArchivedCounts() {
  await Promise.all(['grok', 'openai', 'kiro', 'gemini'].map(p => loadArchivedCount(p)))
  archivedCountLoaded.value = true
}

/** Kiro 独立验活（SSE 实时日志）：仅检测账号状态，删除失效的，不归档 */
async function validateKiro() {
  if (exportingMap['kiro_validate']) return // 防重入
  const list = platformResults.value['kiro']
  if (!list || list.length === 0) return

  exportingMap['kiro_validate'] = true
  const token = localStorage.getItem('token')
  const es = new EventSource(`/ws/kiro/validate?token=${encodeURIComponent(token || '')}&action=validate`)

  return new Promise<void>((resolve) => {
    // 5 分钟超时保护，防止 SSE 挂起导致 UI 锁死
    const timeout = setTimeout(() => {
      es.close()
      toast('Kiro 验证超时', 'error')
      delete exportingMap['kiro_validate']
      loadResults()
      resolve()
    }, 5 * 60 * 1000)

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (data.type === 'log') {
          logs.value.push(data.message)
        } else if (data.type === 'complete') {
          clearTimeout(timeout)
          es.close()
          toast(`Kiro 检测完成: ${data.valid} 个正常, ${data.invalid} 个已禁用`, 'success')

          loadResults()
          delete exportingMap['kiro_validate']
          resolve()
        }
      } catch { /* 忽略解析错误 */ }
    }
    es.onerror = () => {
      clearTimeout(timeout)
      es.close()
      toast('Kiro 验证连接中断', 'error')
      delete exportingMap['kiro_validate']
      loadResults()
      resolve()
    }
  })
}

/** Kiro 验活+归档（SSE 实时日志）：检测账号状态，删除失效的，归档正常的 */
async function validateAndArchiveKiro() {
  if (exportingMap['kiro_validate']) return // 防重入
  const list = platformResults.value['kiro']
  if (!list || list.length === 0) return

  exportingMap['kiro_validate'] = true
  const token = localStorage.getItem('token')
  const es = new EventSource(`/ws/kiro/validate?token=${encodeURIComponent(token || '')}&action=archive`)

  return new Promise<void>((resolve) => {
    const timeout = setTimeout(() => {
      es.close()
      toast('Kiro 验证超时', 'error')
      delete exportingMap['kiro_validate']
      loadResults()
      resolve()
    }, 5 * 60 * 1000)

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (data.type === 'log') {
          logs.value.push(data.message)
        } else if (data.type === 'complete') {
          clearTimeout(timeout)
          es.close()
          toast(`Kiro 验证完成: ${data.valid} 个正常, ${data.invalid} 个已禁用`, 'success')

          lastArchivedCount['kiro'] = data.archived || 0
          loadResults()
          loadArchivedCount('kiro')
          delete exportingMap['kiro_validate']
          resolve()
        }
      } catch { /* 忽略解析错误 */ }
    }
    es.onerror = () => {
      clearTimeout(timeout)
      es.close()
      toast('Kiro 验证连接中断', 'error')
      delete exportingMap['kiro_validate']
      loadResults()
      resolve()
    }
  })
}

/** Kiro 验活+导出+归档（SSE 实时日志） */
async function validateKiroExport() {
  if (exportingMap['kiro_validate']) return // 防重入
  const list = platformResults.value['kiro']
  if (!list || list.length === 0) return

  exportingMap['kiro_validate'] = true
  const token = localStorage.getItem('token')
  const es = new EventSource(`/ws/kiro/validate?token=${encodeURIComponent(token || '')}&action=export_archive`)

  return new Promise<void>((resolve) => {
    const timeout = setTimeout(() => {
      es.close()
      toast('Kiro 验证超时', 'error')
      delete exportingMap['kiro_validate']
      loadResults()
      resolve()
    }, 5 * 60 * 1000)

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (data.type === 'log') {
          logs.value.push(data.message)
        } else if (data.type === 'complete') {
          clearTimeout(timeout)
          es.close()
          toast(`Kiro 验证完成: ${data.valid} 个正常, ${data.invalid} 个已禁用`, 'success')

          // 导出有效账号
          if (data.credentials?.length > 0) {
            doExport('kiro', data.credentials, false)
          }

          lastArchivedCount['kiro'] = data.archived || 0
          loadResults()
          loadArchivedCount('kiro')
          delete exportingMap['kiro_validate']
          resolve()
        }
      } catch { /* 忽略解析错误 */ }
    }
    es.onerror = () => {
      clearTimeout(timeout)
      es.close()
      toast('Kiro 验证连接中断', 'error')
      delete exportingMap['kiro_validate']
      loadResults()
      resolve()
    }
  })
}

/** Gemini 独立验活（SSE 实时日志）：仅检测账号状态，删除失效的，不归档 */
async function validateGemini() {
  if (exportingMap['gemini_validate']) return
  const list = platformResults.value['gemini']
  if (!list || list.length === 0) return

  exportingMap['gemini_validate'] = true
  const token = localStorage.getItem('token')
  const es = new EventSource(`/ws/gemini/validate?token=${encodeURIComponent(token || '')}&action=validate`)

  return new Promise<void>((resolve) => {
    const timeout = setTimeout(() => {
      es.close()
      toast('Gemini 验证超时', 'error')
      delete exportingMap['gemini_validate']
      loadResults()
      resolve()
    }, 5 * 60 * 1000)

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (data.type === 'log') {
          logs.value.push(data.message)
        } else if (data.type === 'complete') {
          clearTimeout(timeout)
          es.close()
          toast(`Gemini 检测完成: ${data.valid} 个正常, ${data.invalid} 个已禁用`, 'success')
          loadResults()
          delete exportingMap['gemini_validate']
          resolve()
        }
      } catch { /* 忽略解析错误 */ }
    }
    es.onerror = () => {
      clearTimeout(timeout)
      es.close()
      toast('Gemini 验证连接中断', 'error')
      delete exportingMap['gemini_validate']
      loadResults()
      resolve()
    }
  })
}

/** Gemini 验活+归档（SSE 实时日志） */
async function validateAndArchiveGemini() {
  if (exportingMap['gemini_validate']) return
  const list = platformResults.value['gemini']
  if (!list || list.length === 0) return

  exportingMap['gemini_validate'] = true
  const token = localStorage.getItem('token')
  const es = new EventSource(`/ws/gemini/validate?token=${encodeURIComponent(token || '')}&action=archive`)

  return new Promise<void>((resolve) => {
    const timeout = setTimeout(() => {
      es.close()
      toast('Gemini 验证超时', 'error')
      delete exportingMap['gemini_validate']
      loadResults()
      resolve()
    }, 5 * 60 * 1000)

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (data.type === 'log') {
          logs.value.push(data.message)
        } else if (data.type === 'complete') {
          clearTimeout(timeout)
          es.close()
          toast(`Gemini 验证完成: ${data.valid} 个正常, ${data.invalid} 个已禁用`, 'success')
          lastArchivedCount['gemini'] = data.archived || 0
          loadResults()
          loadArchivedCount('gemini')
          delete exportingMap['gemini_validate']
          resolve()
        }
      } catch { /* 忽略解析错误 */ }
    }
    es.onerror = () => {
      clearTimeout(timeout)
      es.close()
      toast('Gemini 验证连接中断', 'error')
      delete exportingMap['gemini_validate']
      loadResults()
      resolve()
    }
  })
}

/** Gemini 验活+导出+归档（SSE 实时日志） */
async function validateGeminiExport() {
  if (exportingMap['gemini_validate']) return
  const list = platformResults.value['gemini']
  if (!list || list.length === 0) return

  exportingMap['gemini_validate'] = true
  const token = localStorage.getItem('token')
  const es = new EventSource(`/ws/gemini/validate?token=${encodeURIComponent(token || '')}&action=export_archive`)

  return new Promise<void>((resolve) => {
    const timeout = setTimeout(() => {
      es.close()
      toast('Gemini 验证超时', 'error')
      delete exportingMap['gemini_validate']
      loadResults()
      resolve()
    }, 5 * 60 * 1000)

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (data.type === 'log') {
          logs.value.push(data.message)
        } else if (data.type === 'complete') {
          clearTimeout(timeout)
          es.close()
          toast(`Gemini 验证完成: ${data.valid} 个正常, ${data.invalid} 个已禁用`, 'success')
          if (data.credentials?.length > 0) {
            doExport('gemini', data.credentials, false)
          }
          lastArchivedCount['gemini'] = data.archived || 0
          loadResults()
          loadArchivedCount('gemini')
          delete exportingMap['gemini_validate']
          resolve()
        }
      } catch { /* 忽略解析错误 */ }
    }
    es.onerror = () => {
      clearTimeout(timeout)
      es.close()
      toast('Gemini 验证连接中断', 'error')
      delete exportingMap['gemini_validate']
      loadResults()
      resolve()
    }
  })
}

/** Gemini 归档区验活+导出 */
async function validateGeminiArchived() {
  if (exportingMap['gemini_validate']) return
  if (platformArchivedCount['gemini'] === 0) return

  exportingMap['gemini_validate'] = true
  exportingMap['gemini_archived'] = true
  const token = localStorage.getItem('token')
  const es = new EventSource(`/ws/gemini/validate?token=${encodeURIComponent(token || '')}&action=validate&scope=archived`)

  return new Promise<void>((resolve) => {
    const timeout = setTimeout(() => {
      es.close()
      toast('Gemini 归档验证超时', 'error')
      delete exportingMap['gemini_validate']
      delete exportingMap['gemini_archived']
      loadArchivedCount('gemini')
      resolve()
    }, 5 * 60 * 1000)

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (data.type === 'log') {
          logs.value.push(data.message)
        } else if (data.type === 'complete') {
          clearTimeout(timeout)
          es.close()
          toast(`Gemini 归档验证完成: ${data.valid} 个正常, ${data.invalid} 个已禁用`, 'success')
          if (data.credentials?.length > 0) {
            doExport('gemini', data.credentials, true)
          }
          loadArchivedCount('gemini')
          delete exportingMap['gemini_validate']
          delete exportingMap['gemini_archived']
          resolve()
        }
      } catch { /* 忽略解析错误 */ }
    }
    es.onerror = () => {
      clearTimeout(timeout)
      es.close()
      toast('Gemini 归档验证连接中断', 'error')
      delete exportingMap['gemini_validate']
      delete exportingMap['gemini_archived']
      loadArchivedCount('gemini')
      resolve()
    }
  })
}

async function archivePlatform(platform: string) {
  // Kiro/Gemini 特殊处理：先验活再归档
  if (platform === 'kiro') {
    return validateAndArchiveKiro()
  }
  if (platform === 'gemini') {
    return validateAndArchiveGemini()
  }
  const list = platformResults.value[platform]
  if (!list || list.length === 0) return
  try {
    const { data } = await resultApi.archive(platform)
    lastArchivedCount[platform] = data.archived_count || list.length
    await loadResults()
    await loadArchivedCount(platform)
    toast(`${platform} 已归档 ${lastArchivedCount[platform]} 个`, 'success')
  } catch (e: any) {
    toast(e.response?.data?.detail || '归档失败', 'error')
  }
}

async function exportArchivedPlatform(platform: string) {
  if (platformArchivedCount[platform] === 0) return
  // Kiro/Gemini 归档导出：先验活删除失效的，再导出有效的
  if (platform === 'kiro') {
    return validateKiroArchived()
  }
  if (platform === 'gemini') {
    return validateGeminiArchived()
  }
  const key = `${platform}_archived`
  exportingMap[key] = true
  try {
    toast(`正在导出 ${platform} 归档...`, 'info')
    const { data } = await resultApi.archived(platform)
    const list = data.items || data || []
    if (list.length === 0) { toast('无数据可导出', 'error'); return }
    await doExport(platform, list, true)
    toast(`已导出 ${list.length} 个 ${platform} 归档账号`, 'success')
  } catch { toast('导出失败', 'error') }
  finally { delete exportingMap[key] }
}

/** Kiro 归档区验活+导出（SSE）：检测归档账号，删除失效的，导出有效的 */
async function validateKiroArchived() {
  if (exportingMap['kiro_validate']) return // 防重入
  if (platformArchivedCount['kiro'] === 0) return

  exportingMap['kiro_validate'] = true
  exportingMap['kiro_archived'] = true
  const token = localStorage.getItem('token')
  const es = new EventSource(`/ws/kiro/validate?token=${encodeURIComponent(token || '')}&action=validate&scope=archived`)

  return new Promise<void>((resolve) => {
    const timeout = setTimeout(() => {
      es.close()
      toast('Kiro 归档验证超时', 'error')
      delete exportingMap['kiro_validate']
      delete exportingMap['kiro_archived']
      loadArchivedCount('kiro')
      resolve()
    }, 5 * 60 * 1000)

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (data.type === 'log') {
          logs.value.push(data.message)
        } else if (data.type === 'complete') {
          clearTimeout(timeout)
          es.close()
          toast(`Kiro 归档验证完成: ${data.valid} 个正常, ${data.invalid} 个已禁用`, 'success')

          // 导出有效的归档账号
          if (data.credentials?.length > 0) {
            doExport('kiro', data.credentials, true)
          }

          loadArchivedCount('kiro')
          delete exportingMap['kiro_validate']
          delete exportingMap['kiro_archived']
          resolve()
        }
      } catch { /* 忽略解析错误 */ }
    }
    es.onerror = () => {
      clearTimeout(timeout)
      es.close()
      toast('Kiro 归档验证连接中断', 'error')
      delete exportingMap['kiro_validate']
      delete exportingMap['kiro_archived']
      loadArchivedCount('kiro')
      resolve()
    }
  })
}

async function exportPlatformResults(platform: string) {
  const list = platformResults.value[platform]
  if (!list || list.length === 0) return
  exportingMap[platform] = true
  try {
    await doExport(platform, list, false)
    toast(`已导出 ${list.length} 个 ${platform} 账号`, 'success')
  } finally { delete exportingMap[platform] }
}

/** 导出后自动归档 */
async function exportAndArchive(platform: string) {
  // Kiro/Gemini 特殊处理：先验活再导出+归档
  if (platform === 'kiro') {
    return validateKiroExport()
  }
  if (platform === 'gemini') {
    return validateGeminiExport()
  }
  const list = platformResults.value[platform]
  if (!list || list.length === 0) return
  exportingMap[platform] = true
  try {
    await doExport(platform, list, false)
    // 导出完成后自动归档
    const { data } = await resultApi.archive(platform)
    lastArchivedCount[platform] = data.archived_count || list.length
    await loadResults()
    await loadArchivedCount(platform)
    toast(`已导出并归档 ${list.length} 个 ${platform} 账号`, 'success')
  } catch (e: any) {
    toast(`导出成功，但归档失败: ${e.response?.data?.detail || '未知错误'}`, 'error')
  } finally { delete exportingMap[platform] }
}

async function doExport(platform: string, list: any[], isArchive: boolean, _exportKey?: string) {
  const progressToastId = ++toastId
  // 立即弹出提示，让用户知道正在处理
  toasts.value.push({ id: progressToastId, msg: `正在打包 ${list.length} 个 ${platform} 账号...`, type: 'info' })
  try {
    const zip = new JSZip()
    const now = new Date()
    const dateStr = now.toISOString().slice(0, 10)
    const timeStr = now.toTimeString().slice(0, 8).replace(/:/g, '')
    if (platform === 'grok') {
      const tokens = list.map(r => r.credential_data?.auth_token || r.credential_data?.sso_token || r.auth_token || r.sso_token || '').filter(Boolean)
      zip.file('tokens.txt', tokens.join('\n'))
    } else if (platform === 'openai') {
      list.forEach(r => {
        const cred = r.credential_data
        const email = cred?.email || r.email || 'unknown'
        const planType = cred?.['https://api.openai.com/auth']?.chatgpt_plan_type || 'free'
        const data = (cred && typeof cred === 'object' && Object.keys(cred).length > 0) ? cred : { email: r.email, auth_token: r.auth_token || r.sso_token }
        zip.file(`codex-${email}-${planType}.json`, JSON.stringify(data, null, 2))
      })
    } else if (platform === 'kiro') {
      list.forEach(r => {
        const cred = r.credential_data
        const email = cred?.email || r.email || 'unknown'
        // 过滤敏感字段（email_meta 含邮箱服务 token，不应暴露给用户）
        const { email_meta, mail_provider, disabled, ...safeData } = (cred && typeof cred === 'object' && Object.keys(cred).length > 0) ? cred : { email: r.email }
        zip.file(`kiro-builder-id-${email.split('@')[0] || email}.json`, JSON.stringify(safeData, null, 2))
      })
    } else if (platform === 'gemini') {
      list.forEach(r => {
        const cred = r.credential_data
        const email = cred?.email || r.email || 'unknown'
        // 过滤敏感字段（email_meta 含邮箱服务 token，不应暴露给用户）
        const { email_meta, mail_provider, disabled, ...safeData } = (cred && typeof cred === 'object' && Object.keys(cred).length > 0) ? cred : { email: r.email }
        zip.file(`gemini-business-${email.split('@')[0] || email}.json`, JSON.stringify(safeData, null, 2))
      })
    }
    const suffix = isArchive ? '_archived' : ''
    // 带进度回调的 ZIP 生成，实时更新 toast 内容
    const blob = await zip.generateAsync(
      { type: 'blob' },
      (metadata) => {
        const pct = Math.round(metadata.percent)
        const existing = toasts.value.find(t => t.id === progressToastId)
        if (existing) existing.msg = `正在打包 ${list.length} 个账号... ${pct}%`
      },
    )
    // 移除进度 toast
    toasts.value = toasts.value.filter(t => t.id !== progressToastId)
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = `${platform}_${user.username}_${dateStr}_${timeStr}_${list.length}个${suffix}.zip`
    a.click()
    // 延迟释放，移动端浏览器可能尚未开始下载
    setTimeout(() => URL.revokeObjectURL(a.href), 10000)
  } catch {
    toasts.value = toasts.value.filter(t => t.id !== progressToastId)
    toast('导出失败', 'error')
  }
}

async function saveCurrentTaskResults() {
  if (!taskStatus.task_id) return
  exportingMap['save'] = true
  try {
    const { data } = await resultApi.get(taskStatus.task_id)
    const list = data || []
    if (list.length === 0) { toast('暂无注册结果可保存', 'info'); return }
    await doExport(taskStatus.platform || 'unknown', list, false, 'save')
    toast(`已保存 ${list.length} 个 ${taskStatus.platform} 账号`, 'success')
  } catch { toast('保存失败', 'error') }
  finally { delete exportingMap['save'] }
}

async function loadTxHistory() {
  try { const { data } = await creditApi.history(); txHistory.value = data || []; txHistoryLoaded.value = true }
  catch { /* ignore */ }
}

async function loadGlobalStats() {
  try { const { data } = await statsApi.global(); Object.assign(globalStats, data) }
  catch { /* ignore */ }
}

let globalStatsTimer: ReturnType<typeof setInterval> | null = null
function startGlobalStatsPolling() {
  if (globalStatsTimer) return // 防止重复启动导致旧定时器泄漏
  globalStatsTimer = setInterval(loadGlobalStats, 30000)
}
function stopGlobalStatsPolling() {
  if (globalStatsTimer) { clearInterval(globalStatsTimer); globalStatsTimer = null }
}

async function loadAnnouncements() {
  try { const { data } = await announcementApi.list(); announcements.value = data || [] }
  catch { /* ignore */ }
}

async function fetchOTP(email: string) {
  if (otpLoading[email]) return
  otpLoading[email] = true
  try {
    const { data } = await emailApi.fetchOTP(email)
    if (data.code) {
      await navigator.clipboard.writeText(data.code)
      toast(`验证码 ${data.code} 已复制`, 'success')
    } else {
      toast(data.error || '暂无验证码', 'info')
    }
  } catch { toast('获取验证码失败', 'error') }
  finally { otpLoading[email] = false }
}

async function claimFreeTrial(event: MouseEvent) {
  const btn = (event.target as HTMLElement)?.closest('button')
  try {
    const { data } = await creditApi.claimFreeTrial()
    freeTrial.claimed = true; freeTrial.eligible = false
    freeTrial.remaining = data.count
    toast(`已领取 ${data.count} 次免费注册！`, 'success')
    sprayConfetti(btn)
    await loadBalance()
  } catch (e: any) { toast(e.response?.data?.detail || '领取失败', 'error') }
}

async function smartAction(event?: Event) {
  const v = smartInput.value.trim()
  if (!v) return
  const btn = event ? (event.target as HTMLElement)?.closest('button') : null
  if (smartInputMode.value === 'purchase') {
    const amount = smartInputAmount.value
    if (purchaseProcessing.value || amount < 1) return
    purchaseProcessing.value = true
    try {
      await creditApi.purchase(amount, taskForm.platform)
      toast(`成功购买 ${amount} 次注册！`, 'success')
      smartInput.value = ''
      sprayConfetti(btn)
      await Promise.all([loadBalance(), loadTxHistory()])
    } catch (e: any) { toast(e.response?.data?.detail || '购买失败', 'error') }
    finally { purchaseProcessing.value = false }
  } else {
    try {
      const { data } = await creditApi.redeem(v)
      smartInput.value = ''
      toast(data.message, 'success')
      sprayConfetti(btn)
      await loadBalance()
    } catch (e: any) { toast(e.response?.data?.detail || '兑换失败', 'error') }
  }
}

async function logout() {
  stopGlobalStatsPolling()
  try { await authApi.logout() } catch { /* ignore */ }
  localStorage.removeItem('token')
  window.location.href = '/'
}

// ══════════ 音效 / 通知 / 彩纸 ══════════

let _audioCtx: AudioContext | null = null
function getAudioCtx(): AudioContext {
  if (!_audioCtx || _audioCtx.state === 'closed') _audioCtx = new AudioContext()
  return _audioCtx
}

function playNotificationSound() {
  try {
    const ctx = getAudioCtx()
    if (ctx.state === 'suspended') ctx.resume()
    const notes = [880, 1108.73, 1318.51]
    notes.forEach((freq, i) => {
      const osc = ctx.createOscillator(); const gain = ctx.createGain()
      osc.type = 'sine'; osc.frequency.value = freq
      gain.gain.setValueAtTime(0.15, ctx.currentTime + i * 0.12)
      gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + i * 0.12 + 0.3)
      osc.connect(gain).connect(ctx.destination)
      osc.start(ctx.currentTime + i * 0.12); osc.stop(ctx.currentTime + i * 0.12 + 0.3)
    })
  } catch { /* 静默 */ }
}

function showBrowserNotification(title: string, body: string) {
  if ('Notification' in window && Notification.permission === 'granted') {
    new Notification(title, { body, icon: '/favicon.ico' })
  }
}

function sprayConfetti(originEl: Element | null | undefined) {
  const colors = ['#fbbf24', '#f59e0b', '#ef4444', '#3b82f6', '#22d3ee', '#a78bfa', '#34d399', '#f472b6']
  const rect = originEl?.getBoundingClientRect() || { left: window.innerWidth / 2, top: window.innerHeight / 2, width: 0, height: 0 }
  const cx = rect.left + rect.width, cy = rect.top + rect.height / 2
  for (let i = 0; i < 48; i++) {
    const piece = document.createElement('div')
    piece.style.cssText = `position:fixed;z-index:9999;pointer-events:none;border-radius:${Math.random() > .5 ? '50%' : '2px'};width:${Math.random() * 6 + 4}px;height:${Math.random() * 6 + 4}px;background:${colors[Math.floor(Math.random() * colors.length)]};left:${cx}px;top:${cy}px;`
    document.body.appendChild(piece)
    const angle = (Math.random() - 0.5) * Math.PI * 0.65
    const speed = Math.random() * 220 + 100
    const vx = Math.cos(angle) * speed, vy = Math.sin(angle) * speed
    const gravity = 350 + Math.random() * 200, duration = 1000 + Math.random() * 700
    const start = performance.now()
    const animate = (now: number) => {
      const t = (now - start) / 1000
      if (t > duration / 1000) { piece.remove(); return }
      piece.style.left = (cx + vx * t) + 'px'
      piece.style.top = (cy + vy * t + 0.5 * gravity * t * t) + 'px'
      piece.style.opacity = String(Math.max(0, 1 - t / (duration / 1000)))
      piece.style.transform = `rotate(${t * 400}deg)`
      requestAnimationFrame(animate)
    }
    requestAnimationFrame(animate)
  }
}

/** 任务完成彩纸：从日志面板底部向上喷射，到达屏幕中间高处后自然飘落 */
function sprayTaskCompleteRibbons() {
  const colors = ['#fbbf24', '#f59e0b', '#ef4444', '#3b82f6', '#22d3ee', '#a78bfa', '#34d399', '#f472b6', '#fb923c', '#e879f9']
  const W = window.innerWidth, H = window.innerHeight
  // 从日志面板底部中央发射
  const panelRect = logPanel.value?.getBoundingClientRect()
  const cx = panelRect ? panelRect.left + panelRect.width / 2 : W * 0.75
  const cy = panelRect ? panelRect.bottom - 10 : H * 0.82
  const count = 60

  for (let i = 0; i < count; i++) {
    const ribbon = document.createElement('div')
    const w = Math.random() * 4 + 3     // 窄
    const h = Math.random() * 20 + 14   // 长条
    const color = colors[Math.floor(Math.random() * colors.length)]
    ribbon.style.cssText = `position:fixed;z-index:9999;pointer-events:none;border-radius:2px;width:${w}px;height:${h}px;background:${color};left:${cx}px;top:${cy}px;opacity:1;transform-origin:center center;`
    document.body.appendChild(ribbon)

    const spread = (Math.random() - 0.5) * W * 0.7
    const peakY = H * (0.08 + Math.random() * 0.22)
    const riseDur = 500 + Math.random() * 350
    const fallDur = 2000 + Math.random() * 1500 // 彩带飘落时间更长
    const totalDur = riseDur + fallDur
    const targetX = cx + spread
    const wobbleAmp = 30 + Math.random() * 50 // 飘摆幅度
    const wobbleFreq = 1.5 + Math.random() * 2 // 飘摆频率
    const phaseOffset = Math.random() * Math.PI * 2
    const flipSpeed = 300 + Math.random() * 500 // 翻转速度

    const start = performance.now()
    const animate = (now: number) => {
      const elapsed = now - start
      if (elapsed > totalDur) { ribbon.remove(); return }

      let x: number, y: number, opacity: number, scaleX: number
      if (elapsed < riseDur) {
        const p = elapsed / riseDur
        const ease = 1 - (1 - p) * (1 - p)
        x = cx + (targetX - cx) * ease
        y = cy + (peakY - cy) * ease
        opacity = 1
        scaleX = 1
      } else {
        const fp = (elapsed - riseDur) / fallDur
        // 缓慢加速下落
        const fallEase = fp * fp * 0.7 + fp * 0.3
        x = targetX + Math.sin(fp * Math.PI * wobbleFreq + phaseOffset) * wobbleAmp
        y = peakY + (H + 80 - peakY) * fallEase
        opacity = Math.max(0, 1 - fp * 0.6)
        // 3D 翻转效果：scaleX 在 -1~1 之间摆动，模拟彩带翻面
        scaleX = Math.cos((elapsed / 1000) * flipSpeed * (Math.PI / 180))
      }

      ribbon.style.left = x + 'px'
      ribbon.style.top = y + 'px'
      ribbon.style.opacity = String(opacity)
      ribbon.style.transform = `scaleX(${scaleX}) rotate(${Math.sin((elapsed / 1000) * 2 + phaseOffset) * 25}deg)`
      requestAnimationFrame(animate)
    }
    setTimeout(() => requestAnimationFrame(animate), Math.random() * 200)
  }
}

// ══════════ 批量初始化（单次 HTTP 请求） ══════════

async function initLoad() {
  const { data } = await initApi.load()
  if (data.user) Object.assign(user, data.user)
  if (data.balance) {
    const b = data.balance
    const costPerReg = b.cost_per_reg || 1
    user.quota = b.quota || 0
    user.balance_display = b.display || ''
    user.registrations_available = b.registrations_available ?? Math.floor((b.credits || 0) / costPerReg)
    user.unit_price = b.unit_price || 0; user.unit_price_display = b.unit_price_display || ''
    user.newapi_balance = b.newapi_balance || 0; user.newapi_balance_display = b.newapi_balance_display || '$0'
    user.newapi_available = b.newapi_available || 0; user.mode = b.mode || 'local'
    user.platform_prices = b.platform_prices || {}
    Object.assign(limits, b.limits || {})
    const currentKey = `platform_${taskForm.platform}_enabled` as keyof typeof limits
    if (limits[currentKey] === false) {
      const first = (['grok', 'openai', 'kiro', 'gemini'] as const).find(
        p => limits[`platform_${p}_enabled` as keyof typeof limits] !== false
      )
      if (first) taskForm.platform = first
    }
    if (b.free_trial) {
      freeTrial.eligible = b.free_trial.eligible
      freeTrial.total = b.free_trial.total || b.free_trial.remaining
      if (!freeTrial.claimed) freeTrial.remaining = b.free_trial.remaining
    }
  }
  if (data.results) results.value = data.results
  if (data.tx_history) { txHistory.value = data.tx_history; txHistoryLoaded.value = true }
  if (data.announcements) announcements.value = data.announcements
  if (data.global_stats) Object.assign(globalStats, data.global_stats)
  if (data.recent_completions) recentCompletions.value = dedupeByUser(data.recent_completions)
  if (data.user_stats) {
    Object.assign(userStats, data.user_stats)
    if (data.user_stats.avg_sec_per_reg) Object.assign(avgSecPerReg, data.user_stats.avg_sec_per_reg)
    if (data.user_stats.by_platform) Object.assign(byPlatformStats, data.user_stats.by_platform)
  }
  return data // 返回给调用者处理 current_task
}

// ══════════ 导出单例 ══════════

export function useDashboard() {
  return {
    // 状态
    user, limits, taskForm, taskMode, taskStatus,
    isRunning, isQueued, elapsedSeconds, processing, logs, logPanel, lastLogAt,
    results, platformArchivedCount, archivedCountLoaded, lastArchivedCount,
    otpLoading, txHistory, txHistoryLoaded, txDisplayCount, announcements, freeTrial,
    smartInput, purchaseProcessing, exporting, exportingMap, toasts, taskNotice,
    globalStats, userStats, byPlatformStats, avgSecPerReg, queueModal,
    // computed
    smartInputMode, smartInputAmount, platformLabel, platformUserStats,
    sliderMax, quickTargets, estimatedTime, estimatedRemaining,
    currentPlatformEnabled, currentPlatformFree, currentFreeMode,
    currentFreeModeAvailable, currentPlatformPrice, currentPlatformPriceDisplay,
    currentPlatformLimit, currentPlatformDailyText, currentPlatformMaxDisplay,
    platformTitleClass, refundAmount, vipLabel, vipBadgeClass,
    vipCardGlow, vipAvatarRing, platformCols, platformResults, displayedPlatformResults, showMoreResults,
    // helpers
    vipLabelFor, vipBadgeClassFor, isPlatformFreeByKey,
    platformHealthRate, platformHealthColor,
    toast, setTaskNotice, clearTaskNotice,
    getPassword, copyText, copyEmailPassword, formatElapsed,
    // API
    loadUser, loadBalance, loadResults, refreshResults,
    loadArchivedCount, loadAllArchivedCounts,
    archivePlatform, exportArchivedPlatform, exportPlatformResults, exportAndArchive,
    validateKiro, validateAndArchiveKiro,
    validateGemini, validateAndArchiveGemini,
    saveCurrentTaskResults, loadTxHistory, loadGlobalStats, loadAnnouncements,
    fetchOTP, claimFreeTrial, smartAction, logout,
    // effects
    playNotificationSound, showBrowserNotification, sprayConfetti, sprayTaskCompleteRibbons,
    // init
    initLoad,
    // polling
    startGlobalStatsPolling, stopGlobalStatsPolling,
    // completion toast
    completionQueue, activeCompletion, completionVisible,
    startCompletionPolling, stopCompletionPolling,
    // 全站动态
    recentCompletions, loadRecentCompletions,
  }
}
