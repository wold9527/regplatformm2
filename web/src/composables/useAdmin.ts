/**
 * 管理后台逻辑 — 所有 admin tab 的状态和操作
 * 依赖 useDashboard 的共享状态（toast）
 */
import { ref, reactive, computed, watch } from 'vue'
import { adminApi, notificationApi } from '../api/client'
import { useDashboard } from './useDashboard'

// ── Admin 状态 ──
const showAdmin = ref(false)
const adminTab = ref('overview')
const adminStats = ref<any>({})
const adminUsers = ref<any[]>([])
const adminSettings = ref<any[]>([])
const adminSettingsLoading = ref(false)
const codeForm = reactive({ count: 10, credits_per_code: 50, batch_name: '' })
const generatedCodes = ref<string[]>([])
const adminCodes = ref<any[]>([])
const codePage = ref(1)
const codePageSize = ref(20)
const codeTotal = ref(0)
const codeTotalPages = computed(() => Math.max(1, Math.ceil(codeTotal.value / codePageSize.value)))
const adminAnnouncements = ref<any[]>([])
const annForm = reactive({ title: '', content: '' })

// 实时状态
const realtimeTasks = ref<any[]>([])
const realtimeOnlineUsers = ref(0)
let realtimeTimer: ReturnType<typeof setInterval> | null = null
let realtimeLoading = false
const notifyForm = reactive({ user_id: 0, title: '', content: '' })

// 最近注册活动
const recentActivity = ref<any[]>([])
const recentActivityTotal = ref(0)
const recentActivityPage = ref(1)
const recentActivityPageSize = ref(20)
const recentActivityLoading = ref(false)
const recentActivityTotalPages = computed(() =>
  Math.max(1, Math.ceil(recentActivityTotal.value / recentActivityPageSize.value))
)

// 用户通知（铃铛）
const userNotifications = ref<any[]>([])
const unreadCount = ref(0)
const showNotifPanel = ref(false)
let notifPollTimer: ReturnType<typeof setInterval> | null = null
let notifLoading = false

// 用户分页
const userPage = ref(1)
const userPageSize = ref(20)
const userTotal = ref(0)

// 用户详情
const expandedUserId = ref<number | null>(null)
const userDetailLoading = ref(false)
const userDetailData = ref<any>(null)
const userSuccessRate = computed(() => {
  if (!userDetailData.value?.task_summary) return 0
  const s = userDetailData.value.task_summary.total_success || 0
  const f = userDetailData.value.task_summary.total_fail || 0
  return s + f > 0 ? (s / (s + f)) * 100 : 0
})
const adminUserSearch = ref('')
// 用户列表排序
const userSortField = ref<'credits' | 'newapi_quota' | 'created_at' | ''>('')
const userSortOrder = ref<'asc' | 'desc'>('desc')
function toggleUserSort(field: 'credits' | 'newapi_quota' | 'created_at') {
  if (userSortField.value === field) {
    userSortOrder.value = userSortOrder.value === 'desc' ? 'asc' : 'desc'
  } else {
    userSortField.value = field
    userSortOrder.value = 'desc'
  }
  userPage.value = 1
}
const filteredAdminUsers = computed(() => adminUsers.value)
const userTotalPages = computed(() => Math.max(1, Math.ceil(userTotal.value / userPageSize.value)))
const paginationRange = computed(() => {
  const total = userTotalPages.value
  const cur = userPage.value
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  const pages: (number | string)[] = [1]
  if (cur > 3) pages.push('...')
  for (let i = Math.max(2, cur - 1); i <= Math.min(total - 1, cur + 1); i++) pages.push(i)
  if (cur < total - 2) pages.push('...')
  pages.push(total)
  return pages
})

// 数据清理
const dataStats = ref<any>({})
const dataStatsLoading = ref(false)
const cleanupForm = reactive({ days: 90, clean_results: true, clean_tasks: true, clean_tx: false, clean_archived_only: false })
const cleanupProcessing = ref(false)
const cleanupResult = ref<any>(null)

// 设置搜索 & 分组折叠
const settingsSearch = ref('')
const collapsedGroups = ref<Set<string>>(new Set())

const _groupLabels: Record<string, string> = {
  infra: '基础设施', email: '邮箱服务', captcha: '验证码',
  platform_grok: 'Grok 平台', platform_openai: 'OpenAI 平台', platform_kiro: 'Kiro 平台', platform_gemini: 'Gemini 平台',
  user_policy: '用户策略', hfspace: 'HF 空间',
}

const _groupIcons: Record<string, string> = {
  infra: '🔧', email: '📧', captcha: '🛡️',
  platform_grok: '🤖', platform_openai: '🧠', platform_kiro: '⚡', platform_gemini: '✨',
  user_policy: '👥', hfspace: '☁️',
}

export function useAdmin() {
  const d = useDashboard()

  // ── 基础加载 ──
  async function openAdmin() {
    showAdmin.value = !showAdmin.value
    if (showAdmin.value) { loadAdminStats(); loadAdminUsers() }
  }
  async function loadAdminStats() {
    try { const { data } = await adminApi.stats(); adminStats.value = data } catch { /* ignore */ }
  }
  async function loadAdminUsers() {
    try {
      const { data } = await adminApi.users({
        page: userPage.value,
        page_size: userPageSize.value,
        search: adminUserSearch.value.trim() || undefined,
        sort_by: userSortField.value || undefined,
        sort_order: userSortField.value ? userSortOrder.value : undefined,
      })
      adminUsers.value = data.items || []
      userTotal.value = data.total || 0
    } catch { /* ignore */ }
  }

  // ── 设置 ──
  async function loadAdminSettings() {
    adminSettingsLoading.value = true
    try {
      const { data } = await adminApi.settings()
      adminSettings.value = data.map((s: any) => ({ ...s, _dirty: false, _editValue: undefined, _showRaw: false, _rawValue: null }))
    } catch { /* ignore */ }
    adminSettingsLoading.value = false
  }
  function settingsGroups(): string[] {
    const groups: string[] = []
    for (const s of adminSettings.value) { if (!groups.includes(s.group)) groups.push(s.group) }
    return groups
  }
  function groupLabel(group: string) { return _groupLabels[group] || group }
  function groupIcon(group: string) { return _groupIcons[group] || '⚙️' }
  function settingsInGroup(group: string) { return adminSettings.value.filter(s => s.group === group) }

  /** 按搜索词过滤后的分组设置 */
  function filteredSettingsInGroup(group: string) {
    const q = settingsSearch.value.trim().toLowerCase()
    const items = settingsInGroup(group)
    if (!q) return items
    return items.filter(s =>
      s.label.toLowerCase().includes(q) ||
      s.key.toLowerCase().includes(q) ||
      (s.description || '').toLowerCase().includes(q)
    )
  }

  /** 分组配置状态统计 */
  function groupStats(group: string) {
    const items = settingsInGroup(group)
    const configured = items.filter(s => s.has_value).length
    return { total: items.length, configured }
  }

  /** 当前有多少项被修改 */
  const dirtyCount = computed(() => adminSettings.value.filter(s => s._dirty).length)

  function toggleGroup(group: string) {
    if (collapsedGroups.value.has(group)) collapsedGroups.value.delete(group)
    else collapsedGroups.value.add(group)
  }
  function isGroupCollapsed(group: string) { return collapsedGroups.value.has(group) }

  async function toggleSettingVisibility(s: any) {
    if (s._showRaw) { s._showRaw = false; return }
    try { const { data } = await adminApi.settingRaw(s.key); s._rawValue = data.value; s._showRaw = true } catch { /* ignore */ }
  }
  async function saveSetting(s: any) {
    if (!s._dirty) return
    const value = s._editValue !== undefined ? s._editValue : ''
    try {
      await adminApi.saveSetting(s.key, value)
      s._dirty = false; s._editValue = undefined; s.has_value = !!value
      if (s.is_secret && value) s.value = value.slice(0, 4) + '****' + value.slice(-4)
      else s.value = value
      // 刷新前端限制状态（上限、每日限制、平台开关等）
      d.loadBalance()
    } catch (e: any) { throw e }
  }

  /** 批量保存所有修改项 */
  async function saveAllDirtySettings() {
    const dirty = adminSettings.value.filter(s => s._dirty)
    if (dirty.length === 0) return
    let ok = 0, fail = 0
    for (const s of dirty) {
      try { await saveSetting(s); ok++ }
      catch { fail++ }
    }
    if (fail === 0) d.toast(`已保存 ${ok} 项设置`, 'success')
    else d.toast(`保存完成：${ok} 成功，${fail} 失败`, 'error')
  }

  // ── 兑换码 ──
  async function generateCodes() {
    try {
      const { data } = await adminApi.generateCodes(codeForm.count, codeForm.credits_per_code, codeForm.batch_name)
      generatedCodes.value = data.codes || []
      d.toast(data.message, 'success')
      await loadAdminCodes()
    } catch (e: any) { d.toast(e.response?.data?.detail || '生成失败', 'error') }
  }
  async function loadAdminCodes() {
    try {
      const { data } = await adminApi.codes({ page: codePage.value, page_size: codePageSize.value })
      adminCodes.value = data.items || []
      codeTotal.value = data.total || 0
    } catch { /* ignore */ }
  }
  function codeGoPage(p: number) {
    if (p < 1 || p > codeTotalPages.value) return
    codePage.value = p
    loadAdminCodes()
  }
  function copyAllCodes() { d.copyText(generatedCodes.value.join('\n')); d.toast('已复制全部兑换码', 'success') }

  // ── 公告 ──
  async function loadAdminAnnouncements() {
    try { const { data } = await adminApi.announcements(); adminAnnouncements.value = data || [] } catch { /* ignore */ }
  }
  async function createAnnouncement() {
    if (!annForm.title.trim() || !annForm.content.trim()) return
    try {
      await adminApi.createAnnouncement(annForm.title, annForm.content)
      annForm.title = ''; annForm.content = ''
      d.toast('公告已发布', 'success')
      await loadAdminAnnouncements(); await d.loadAnnouncements()
    } catch (e: any) { d.toast(e.response?.data?.detail || '发布失败', 'error') }
  }
  async function deleteAnnouncement(id: number) {
    try {
      await adminApi.deleteAnnouncement(id); d.toast('已删除', 'success')
      await loadAdminAnnouncements(); await d.loadAnnouncements()
    } catch (e: any) { d.toast(e.response?.data?.detail || '删除失败', 'error') }
  }

  // ── 用户管理 ──
  function userGoPage(p: number) {
    if (p < 1 || p > userTotalPages.value) return
    userPage.value = p
    expandedUserId.value = null
    userDetailData.value = null
    loadAdminUsers()
  }

  async function adminRecharge(userId: number, username: string) {
    const amount = prompt(`给 ${username} 充值/扣除积分？\n正数充值，负数扣除（如 -100）`)
    if (!amount || isNaN(Number(amount)) || Number(amount) === 0) return
    const n = parseInt(amount)
    if (n < 0 && !confirm(`确认从 ${username} 扣除 ${-n} 积分？`)) return
    try {
      const { data } = await adminApi.recharge(userId, n)
      d.toast(data.message, 'success'); await loadAdminUsers()
    } catch (e: any) { d.toast(e.response?.data?.detail || '操作失败', 'error') }
  }
  async function toggleAdminRole(userId: number) {
    try {
      const { data } = await adminApi.toggleAdmin(userId)
      d.toast(data.message, 'success'); await loadAdminUsers()
    } catch (e: any) { d.toast(e.response?.data?.detail || '操作失败', 'error') }
  }

  // ── 用户详情 ──
  async function toggleUserDetail(userId: number) {
    if (expandedUserId.value === userId) { expandedUserId.value = null; userDetailData.value = null; return }
    expandedUserId.value = userId; userDetailLoading.value = true; userDetailData.value = null
    try { const { data } = await adminApi.userDetail(userId); userDetailData.value = data }
    catch (e: any) { d.toast(e.response?.data?.detail || '加载用户详情失败', 'error'); expandedUserId.value = null }
    finally { userDetailLoading.value = false }
  }
  function getUserDetailPassword(r: any): string {
    const cred = r.credential_data
    return (cred && typeof cred === 'object') ? cred.password || '' : ''
  }
  function copyUserDetailAccount(r: any) {
    const cred = r.credential_data
    d.copyText(`${cred?.email || r.email || ''}----${cred?.password || ''}`)
  }

  // ── 数据清理 ──
  async function loadDataStats() {
    dataStatsLoading.value = true
    try { const { data } = await adminApi.dataStats(); dataStats.value = data } catch { /* ignore */ }
    dataStatsLoading.value = false
  }
  async function executeCleanup() {
    if (cleanupProcessing.value) return
    if (!confirm(`确定要清理 ${cleanupForm.days} 天前的数据吗？此操作不可撤销！`)) return
    cleanupProcessing.value = true; cleanupResult.value = null
    try {
      const { data } = await adminApi.cleanup({ ...cleanupForm })
      cleanupResult.value = data; d.toast(data.message, 'success'); await loadDataStats()
    } catch (e: any) { d.toast(e.response?.data?.detail || '清理失败', 'error') }
    finally { cleanupProcessing.value = false }
  }

  // ── 实时任务 ──
  async function loadRealtimeTasks() {
    if (realtimeLoading) return; realtimeLoading = true
    try {
      const { data } = await adminApi.runningTasks()
      realtimeTasks.value = data.tasks || []; realtimeOnlineUsers.value = data.online_users || 0
    } catch (e: any) { d.toast(e.response?.data?.detail || '加载实时任务失败', 'error') } finally { realtimeLoading = false }
  }

  // ── 最近注册活动 ──
  async function loadRecentActivity() {
    recentActivityLoading.value = true
    try {
      const { data } = await adminApi.recentActivity({
        page: recentActivityPage.value,
        page_size: recentActivityPageSize.value,
      })
      recentActivity.value = data.items || []
      recentActivityTotal.value = data.total || 0
    } catch (e: any) { d.toast(e.response?.data?.detail || '加载注册活动失败', 'error') }
    recentActivityLoading.value = false
  }
  function recentActivityGoPage(p: number) {
    if (p < 1 || p > recentActivityTotalPages.value) return
    recentActivityPage.value = p
    loadRecentActivity()
  }
  function startRealtimePolling() {
    loadRealtimeTasks()
    loadRecentActivity()
    if (realtimeTimer) clearInterval(realtimeTimer)
    realtimeTimer = setInterval(loadRealtimeTasks, 3000)
  }
  function stopRealtimePolling() { if (realtimeTimer) { clearInterval(realtimeTimer); realtimeTimer = null } }
  async function adminStopTask(taskId: number) {
    try { const { data } = await adminApi.stopTask(taskId); d.toast(data.message, 'success'); await loadRealtimeTasks() }
    catch (e: any) { d.toast(e.response?.data?.detail || '停止失败', 'error') }
  }
  async function adminDeleteTask(taskId: number) {
    if (!confirm('确定要删除此任务及其所有结果吗？此操作不可撤销！')) return
    try { const { data } = await adminApi.deleteTask(taskId); d.toast(data.message, 'success'); await loadRealtimeTasks() }
    catch (e: any) { d.toast(e.response?.data?.detail || '删除失败', 'error') }
  }
  async function sendNotification() {
    if (!notifyForm.title.trim() || !notifyForm.content.trim()) return
    try {
      const { data } = await adminApi.sendNotification({ user_id: notifyForm.user_id, title: notifyForm.title, content: notifyForm.content })
      d.toast(data.message, 'success'); notifyForm.user_id = 0; notifyForm.title = ''; notifyForm.content = ''
    } catch (e: any) { d.toast(e.response?.data?.detail || '发送失败', 'error') }
  }
  function formatElapsedSec(sec: number): string {
    if (sec < 60) return `${Math.round(sec)}秒`
    const m = Math.floor(sec / 60); const s = Math.round(sec % 60)
    return s > 0 ? `${m}分${s}秒` : `${m}分`
  }

  // ── 相对时间格式化 ──
  function formatNotifTime(dateStr: string): string {
    const d = new Date(dateStr)
    const now = Date.now()
    const diff = now - d.getTime()
    if (diff < 0) return '刚刚'
    const sec = Math.floor(diff / 1000)
    if (sec < 60) return '刚刚'
    const min = Math.floor(sec / 60)
    if (min < 60) return `${min}分钟前`
    const hr = Math.floor(min / 60)
    if (hr < 24) return `${hr}小时前`
    const day = Math.floor(hr / 24)
    if (day === 1) return '昨天'
    if (day < 7) return `${day}天前`
    return d.toLocaleDateString()
  }

  // ── 用户通知（铃铛） ──
  async function loadUserNotifications() {
    if (notifLoading) return; notifLoading = true
    try {
      const { data } = await notificationApi.list()
      userNotifications.value = data.notifications || []; unreadCount.value = data.unread_count || 0
    } catch { /* ignore */ } finally { notifLoading = false }
  }
  function startNotifPolling() {
    loadUserNotifications()
    if (notifPollTimer) clearInterval(notifPollTimer)
    notifPollTimer = setInterval(loadUserNotifications, 6000)
  }
  function stopNotifPolling() { if (notifPollTimer) { clearInterval(notifPollTimer); notifPollTimer = null } }
  async function markNotifRead(id: number | string) {
    try { await notificationApi.markRead(id); await loadUserNotifications() } catch { /* ignore */ }
  }
  async function markAllNotifsRead() {
    try { await notificationApi.markRead('all'); await loadUserNotifications() } catch { /* ignore */ }
  }

  // ── 管理后台通知列表 ──
  const adminNotifications = ref<any[]>([])
  const adminNotifsLoading = ref(false)
  async function loadAdminNotifications() {
    adminNotifsLoading.value = true
    try {
      const { data } = await adminApi.listNotifications()
      adminNotifications.value = data || []
    } catch { /* ignore */ }
    adminNotifsLoading.value = false
  }
  async function deleteAdminNotification(id: number) {
    if (!confirm('确定要删除此通知吗？')) return
    try {
      await adminApi.deleteNotification(id)
      d.toast('通知已删除', 'success')
      await loadAdminNotifications()
    } catch (e: any) { d.toast(e.response?.data?.detail || '删除失败', 'error') }
  }

  // ── Watchers ──
  let searchDebounce: ReturnType<typeof setTimeout> | null = null
  function setupAdminWatchers() {
    watch(adminTab, (tab) => {
      if (tab === 'realtime') { startRealtimePolling() }
      else stopRealtimePolling()
    })
    watch(showAdmin, (open) => { if (!open) stopRealtimePolling() })
    watch(adminUserSearch, () => {
      if (searchDebounce) clearTimeout(searchDebounce)
      searchDebounce = setTimeout(() => { userPage.value = 1; loadAdminUsers() }, 300)
    })
    // 排序变更时重新从服务器加载
    watch([userSortField, userSortOrder], () => { loadAdminUsers() })
  }

  // ── 清理 ──
  function cleanup() { stopNotifPolling(); stopRealtimePolling() }

  return {
    // 状态
    showAdmin, adminTab, adminStats, adminUsers, adminSettings, adminSettingsLoading,
    settingsSearch, collapsedGroups, dirtyCount,
    codeForm, generatedCodes, adminCodes, codePage, codePageSize, codeTotal, codeTotalPages,
    adminAnnouncements, annForm,
    realtimeTasks, realtimeOnlineUsers, notifyForm,
    recentActivity, recentActivityTotal, recentActivityPage,
    recentActivityPageSize, recentActivityTotalPages, recentActivityLoading,
    userNotifications, unreadCount, showNotifPanel,
    adminNotifications, adminNotifsLoading,
    expandedUserId, userDetailLoading, userDetailData, userSuccessRate,
    adminUserSearch, filteredAdminUsers, userSortField, userSortOrder, toggleUserSort,
    userPage, userPageSize, userTotal, userTotalPages, paginationRange,
    dataStats, dataStatsLoading, cleanupForm, cleanupProcessing, cleanupResult,
    // 方法
    openAdmin, loadAdminStats, loadAdminUsers, userGoPage,
    loadAdminSettings, settingsGroups, groupLabel, groupIcon, settingsInGroup,
    filteredSettingsInGroup, groupStats, toggleGroup, isGroupCollapsed,
    toggleSettingVisibility, saveSetting, saveAllDirtySettings,
    generateCodes, loadAdminCodes, copyAllCodes, codeGoPage,
    loadAdminAnnouncements, createAnnouncement, deleteAnnouncement,
    adminRecharge, toggleAdminRole, toggleUserDetail,
    getUserDetailPassword, copyUserDetailAccount,
    loadDataStats, executeCleanup,
    loadRealtimeTasks, startRealtimePolling, stopRealtimePolling,
    loadRecentActivity, recentActivityGoPage,
    adminStopTask, adminDeleteTask, sendNotification, formatElapsedSec,
    loadUserNotifications, startNotifPolling, stopNotifPolling,
    formatNotifTime, markNotifRead, markAllNotifsRead,
    loadAdminNotifications, deleteAdminNotification,
    setupAdminWatchers, cleanup,
  }
}
