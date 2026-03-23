/**
 * 代理池管理 — 管理后台数据层
 * 提供代理池的增删改查、健康检查、批量导入、URL 抓取等功能
 */
import { ref, reactive, computed } from 'vue'
import { proxyPoolApi, adminApi } from '../api/client'
import { useDashboard } from './useDashboard'

// ── 类型定义 ──

export interface ProxyEntry {
  id: number
  name: string
  protocol: string
  host: string
  port: number
  username?: string
  password?: string
  country?: string
  is_healthy: boolean
  latency_ms: number
  last_checked_at?: string
  fail_count: number
  source?: string
}

export interface ProxyPoolStats {
  total: number
  healthy: number
  unhealthy: number
  active: number
}

// ── 模块级单例状态 ──

const proxies = ref<ProxyEntry[]>([])
const proxyStats = reactive<ProxyPoolStats>({ total: 0, healthy: 0, unhealthy: 0, active: 0 })
const loading = ref(false)
const statsLoading = ref(false)
const healthCheckRunning = ref(false)
const purgeRunning = ref(false)
const filterMode = ref<'' | 'healthy' | 'unhealthy'>('')

// 选择状态
const selectedIds = ref<Set<number>>(new Set())

// 添加表单
const addForm = reactive({
  name: '',
  protocol: 'socks5',
  host: '',
  port: 1080,
  username: '',
  password: '',
  country: '',
})
const addDialogOpen = ref(false)
const addLoading = ref(false)

// 批量导入表单
const importDialogOpen = ref(false)
const importText = ref('')
const importProtocol = ref('socks5')
const importLoading = ref(false)

// 密码可见性
const showPasswordIds = ref<Set<number>>(new Set())

// 从 URL 抓取
const fetchUrlDialogOpen = ref(false)
const fetchUrlLoading = ref(false)
const fetchUrlForm = reactive({
  url: '',
  protocol: 'socks5',
})

// 分页状态
const currentPage = ref(1)
const pageSize = ref(50)
const totalCount = ref(0)

// ── 平台代理策略状态 ──

export type ProxyMode = 'pool' | 'fixed' | 'direct' | 'smart'

export interface PlatformProxyConfig {
  mode: ProxyMode
  fixedProxy: string
  saving: boolean
  dirty: boolean
}

const PLATFORMS = ['grok', 'openai', 'kiro', 'gemini'] as const
export type PlatformKey = typeof PLATFORMS[number]

const PLATFORM_LABELS: Record<PlatformKey, string> = {
  grok: 'Grok',
  openai: 'OpenAI',
  kiro: 'Kiro',
  gemini: 'Gemini',
}

const platformProxyConfigs = reactive<Record<PlatformKey, PlatformProxyConfig>>({
  grok:   { mode: 'pool', fixedProxy: '', saving: false, dirty: false },
  openai: { mode: 'pool', fixedProxy: '', saving: false, dirty: false },
  kiro:   { mode: 'pool', fixedProxy: '', saving: false, dirty: false },
  gemini: { mode: 'pool', fixedProxy: '', saving: false, dirty: false },
})
const proxyConfigLoading = ref(false)

export function useProxyPool() {
  const { toast } = useDashboard()

  // ── 加载 ──

  async function loadProxies() {
    loading.value = true
    try {
      const { data } = await proxyPoolApi.list({
        page: currentPage.value,
        page_size: pageSize.value,
        filter: filterMode.value || undefined,
      })
      proxies.value = data.items || []
      totalCount.value = data.total || 0
    } catch (e: any) {
      toast(e.response?.data?.detail || '加载代理池失败', 'error')
    } finally {
      loading.value = false
    }
  }

  async function loadStats() {
    statsLoading.value = true
    try {
      const { data } = await proxyPoolApi.stats()
      Object.assign(proxyStats, data)
    } catch (e: any) {
      console.warn('加载代理池统计失败:', e.response?.data?.detail || e.message)
    } finally {
      statsLoading.value = false
    }
  }

  async function loadAll() {
    await Promise.all([loadProxies(), loadStats()])
  }

  // ── 平台代理策略 ──

  async function loadProxyConfig() {
    proxyConfigLoading.value = true
    try {
      const { data } = await adminApi.settings()
      const settingsMap: Record<string, string> = {}
      for (const s of data) {
        settingsMap[s.key] = s.value || s.default_value || ''
      }
      for (const p of PLATFORMS) {
        const cfg = platformProxyConfigs[p]
        const mode = settingsMap[`${p}_proxy_mode`] || 'pool'
        cfg.mode = (['pool', 'fixed', 'direct', 'smart'].includes(mode) ? mode : 'pool') as ProxyMode
        cfg.fixedProxy = settingsMap[`${p}_proxy`] || ''
        cfg.dirty = false
      }
    } catch (e: any) {
      console.warn('加载平台代理策略失败:', e.response?.data?.detail || e.message)
    } finally {
      proxyConfigLoading.value = false
    }
  }

  async function saveProxyConfig(platform: PlatformKey) {
    const cfg = platformProxyConfigs[platform]
    cfg.saving = true
    try {
      await adminApi.saveSetting(`${platform}_proxy_mode`, cfg.mode)
      if (cfg.mode === 'fixed') {
        await adminApi.saveSetting(`${platform}_proxy`, cfg.fixedProxy)
      }
      cfg.dirty = false
      toast(`${PLATFORM_LABELS[platform]} 代理策略已保存`, 'success')
    } catch (e: any) {
      toast(e.response?.data?.detail || '保存失败', 'error')
    } finally {
      cfg.saving = false
    }
  }

  async function saveAllProxyConfigs() {
    const dirtyPlatforms = PLATFORMS.filter(p => platformProxyConfigs[p].dirty)
    if (dirtyPlatforms.length === 0) { toast('没有需要保存的更改', 'success'); return }
    for (const p of dirtyPlatforms) {
      await saveProxyConfig(p)
    }
  }

  // ── 操作 ──

  async function addProxy() {
    if (!addForm.host.trim() || !addForm.port) return
    addLoading.value = true
    try {
      await proxyPoolApi.add({
        name: addForm.name.trim() || `${addForm.protocol}://${addForm.host}:${addForm.port}`,
        protocol: addForm.protocol,
        host: addForm.host.trim(),
        port: Number(addForm.port),
        username: addForm.username.trim() || undefined,
        password: addForm.password.trim() || undefined,
        country: addForm.country.trim() || undefined,
      })
      toast('代理已添加', 'success')
      addDialogOpen.value = false
      resetAddForm()
      await loadAll()
    } catch (e: any) {
      toast(e.response?.data?.detail || '添加失败', 'error')
    } finally {
      addLoading.value = false
    }
  }

  function resetAddForm() {
    addForm.name = ''
    addForm.protocol = 'socks5'
    addForm.host = ''
    addForm.port = 1080
    addForm.username = ''
    addForm.password = ''
    addForm.country = ''
  }

  async function deleteProxy(id: number) {
    if (!confirm('确定要删除此代理吗？')) return
    try {
      await proxyPoolApi.delete(id)
      toast('已删除', 'success')
      selectedIds.value.delete(id)
      await loadAll()
    } catch (e: any) {
      toast(e.response?.data?.detail || '删除失败', 'error')
    }
  }

  async function batchDelete() {
    const ids = [...selectedIds.value]
    if (ids.length === 0) return
    if (!confirm(`确定要删除选中的 ${ids.length} 个代理吗？`)) return
    try {
      await proxyPoolApi.batchDelete(ids)
      toast(`已删除 ${ids.length} 个代理`, 'success')
      selectedIds.value.clear()
      await loadAll()
    } catch (e: any) {
      toast(e.response?.data?.detail || '批量删除失败', 'error')
    }
  }

  async function importProxies() {
    const text = importText.value.trim()
    if (!text) return
    importLoading.value = true
    try {
      const { data } = await proxyPoolApi.import(text, importProtocol.value)
      toast(data.message || `已导入 ${data.imported || 0} 个代理`, 'success')
      importDialogOpen.value = false
      importText.value = ''
      await loadAll()
    } catch (e: any) {
      toast(e.response?.data?.detail || '导入失败', 'error')
    } finally {
      importLoading.value = false
    }
  }

  async function fetchFromURL() {
    const url = fetchUrlForm.url.trim()
    if (!url) return
    fetchUrlLoading.value = true
    try {
      const { data } = await proxyPoolApi.fetchURL(url, fetchUrlForm.protocol)
      toast(data.message || `已导入 ${data.imported || 0} 个代理`, 'success')
      fetchUrlDialogOpen.value = false
      fetchUrlForm.url = ''
      await loadAll()
    } catch (e: any) {
      toast(e.response?.data?.detail || '抓取失败', 'error')
    } finally {
      fetchUrlLoading.value = false
    }
  }

  async function triggerHealthCheck() {
    if (healthCheckRunning.value) return
    healthCheckRunning.value = true
    let refreshTimer: ReturnType<typeof setTimeout> | undefined
    let cooldownTimer: ReturnType<typeof setTimeout> | undefined
    try {
      // 选中代理时仅检查选中的，否则检查当前页全部
      const ids = selectedIds.value.size > 0
        ? [...selectedIds.value]
        : proxies.value.map(p => p.id)
      const { data } = await proxyPoolApi.healthCheck(ids)
      toast(data.message || '健康检查已完成', 'success')
      // 延迟刷新，等待后端检查完成
      refreshTimer = setTimeout(() => loadAll(), 3000)
    } catch (e: any) {
      toast(e.response?.data?.detail || '触发健康检查失败', 'error')
    } finally {
      cooldownTimer = setTimeout(() => { healthCheckRunning.value = false }, 3000)
    }
    void refreshTimer
    void cooldownTimer
  }

  async function purgeUnhealthy() {
    if (!confirm('确定要清除所有不健康的代理吗？')) return
    purgeRunning.value = true
    try {
      const { data } = await proxyPoolApi.purge()
      toast(data.message || '不健康代理已清除', 'success')
      await loadAll()
    } catch (e: any) {
      toast(e.response?.data?.detail || '清除失败', 'error')
    } finally {
      purgeRunning.value = false
    }
  }

  async function resetHealth(id: number) {
    try {
      await proxyPoolApi.reset(id)
      toast('健康状态已重置', 'success')
      await loadAll()
    } catch (e: any) {
      toast(e.response?.data?.detail || '重置失败', 'error')
    }
  }

  // ── 选择逻辑 ──

  function toggleSelect(id: number) {
    if (selectedIds.value.has(id)) selectedIds.value.delete(id)
    else selectedIds.value.add(id)
  }

  function toggleSelectAll() {
    if (selectedIds.value.size === proxies.value.length) {
      selectedIds.value.clear()
    } else {
      selectedIds.value = new Set(proxies.value.map(p => p.id))
    }
  }

  // ── 密码显示 ──

  function togglePasswordVisible(id: number) {
    if (showPasswordIds.value.has(id)) showPasswordIds.value.delete(id)
    else showPasswordIds.value.add(id)
  }

  function isPasswordVisible(id: number) {
    return showPasswordIds.value.has(id)
  }

  // ── 分页 ──

  const totalPages = computed(() => Math.max(1, Math.ceil(totalCount.value / pageSize.value)))

  function goToPage(p: number) {
    if (p < 1 || p > totalPages.value) return
    currentPage.value = p
    loadProxies()
  }

  // ── 格式化工具 ──

  function formatLatency(ms: number): string {
    if (!ms || ms <= 0) return '-'
    if (ms < 1000) return `${ms}ms`
    return `${(ms / 1000).toFixed(1)}s`
  }

  function formatLastChecked(dateStr?: string): string {
    if (!dateStr) return '未检查'
    const d = new Date(dateStr)
    const diff = Date.now() - d.getTime()
    const sec = Math.floor(diff / 1000)
    if (sec < 60) return '刚刚'
    const min = Math.floor(sec / 60)
    if (min < 60) return `${min}分钟前`
    const hr = Math.floor(min / 60)
    if (hr < 24) return `${hr}小时前`
    const day = Math.floor(hr / 24)
    return `${day}天前`
  }

  return {
    // 状态
    proxies, proxyStats, loading, statsLoading,
    healthCheckRunning, purgeRunning, filterMode,
    selectedIds,
    addForm, addDialogOpen, addLoading,
    importDialogOpen, importText, importProtocol, importLoading,
    showPasswordIds,
    // URL 抓取
    fetchUrlDialogOpen, fetchUrlLoading, fetchUrlForm,
    // 分页
    currentPage, pageSize, totalCount, totalPages,
    // 方法
    loadProxies, loadStats, loadAll,
    addProxy, resetAddForm, deleteProxy, batchDelete,
    importProxies, fetchFromURL,
    triggerHealthCheck, purgeUnhealthy, resetHealth,
    toggleSelect, toggleSelectAll,
    togglePasswordVisible, isPasswordVisible,
    formatLatency, formatLastChecked,
    goToPage,
    // 平台代理策略
    platformProxyConfigs, proxyConfigLoading,
    PLATFORMS, PLATFORM_LABELS,
    loadProxyConfig, saveProxyConfig, saveAllProxyConfigs,
  }
}
