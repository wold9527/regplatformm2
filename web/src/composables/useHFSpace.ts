/**
 * HF Space 管理 — 状态和操作
 */
import { ref, reactive, computed, watch } from 'vue'
import { hfSpaceApi } from '../api/client'
import { useDashboard } from './useDashboard'

// ── 状态 ──
const tokens = ref<any[]>([])
const spaces = ref<any[]>([])
const overview = ref<any[]>([])
const healthResults = ref<any[]>([])
const tokensLoading = ref(false)
const spacesLoading = ref(false)
const overviewLoading = ref(false)
const healthLoading = ref(false)
const deployLoading = ref(false)
const autoscaleLoading = ref(false)
const syncCFLoading = ref(false)
const updateLoading = ref(false)
const validateAllLoading = ref(false)
const purgeLoading = ref(false)
const updateService = ref('all')
const updateResult = ref<any>(null)

// 各服务默认 Release URL（与后端 system_setting.go 中的默认值保持一致）
const RELEASE_URL_DEFAULTS: Record<string, string> = {
  openai: 'https://github.com/xiaolajiaoyyds/regplatformm/releases/download/inference-runtime-latest/inference-runtime.zip',
  grok:   'https://github.com/xiaolajiaoyyds/regplatformm/releases/download/stream-worker-latest/stream-worker.zip',
  kiro:   'https://github.com/xiaolajiaoyyds/regplatformm/releases/download/browser-agent-latest/browser-agent.zip',
  gemini: 'https://github.com/xiaolajiaoyyds/regplatformm/releases/download/gemini-agent-latest/gemini-agent.zip',
  ts:     'https://github.com/xiaolajiaoyyds/regplatformm/releases/download/net-toolkit-latest/net-toolkit.zip',
}

const spaceFilter = ref('')
const statusFilter = ref('')
const tokenForm = reactive({ label: '', token: '' })
const addSpaceForm = reactive({ service: 'openai', url: '', repo_id: '', token_id: 0 })
const deployForm = reactive({ service: 'openai', count: 1, release_url: RELEASE_URL_DEFAULTS['openai'], token_id: 0 })
const deployResult = ref<any>(null)
const batchDeleteLoading = ref(false)

// 切换服务时自动更新 Release URL（仅在 URL 为空或匹配某个默认值时才覆盖，避免覆盖用户自定义）
watch(() => deployForm.service, (svc) => {
  const isDefault = Object.values(RELEASE_URL_DEFAULTS).includes(deployForm.release_url) || !deployForm.release_url.trim()
  if (isDefault) deployForm.release_url = RELEASE_URL_DEFAULTS[svc] ?? ''
})
const autoscaleForm = reactive({ service: 'all', target: 0, dry_run: false })
const autoscaleResult = ref<any>(null)

// Space 分页
const spacePage = ref(1)
const spacePageSize = ref(20)
const spaceTotal = ref(0)
const spaceTotalPages = computed(() => Math.max(1, Math.ceil(spaceTotal.value / spacePageSize.value)))


export function useHFSpace() {
  const d = useDashboard()

  // ── 概览 ──
  async function loadOverview() {
    overviewLoading.value = true
    try { const { data } = await hfSpaceApi.overview(); overview.value = data || [] }
    catch { /* ignore */ }
    finally { overviewLoading.value = false }
  }

  // ── Token ──
  async function loadTokens() {
    tokensLoading.value = true
    try { const { data } = await hfSpaceApi.listTokens(); tokens.value = data || [] }
    catch { /* ignore */ }
    finally { tokensLoading.value = false }
  }

  async function createToken() {
    if (!tokenForm.label.trim() || !tokenForm.token.trim()) return
    try {
      await hfSpaceApi.createToken({ label: tokenForm.label, token: tokenForm.token })
      tokenForm.label = ''; tokenForm.token = ''
      d.toast('Token 已创建', 'success')
      await loadTokens(); await loadOverview()
    } catch (e: any) { d.toast(e.response?.data?.detail || '创建失败', 'error') }
  }

  async function deleteToken(id: number) {
    if (!confirm('确定要删除此 Token 吗？')) return
    try {
      await hfSpaceApi.deleteToken(id)
      d.toast('已删除', 'success')
      await loadTokens()
    } catch (e: any) { d.toast(e.response?.data?.detail || '删除失败', 'error') }
  }

  async function validateToken(id: number) {
    try {
      const { data } = await hfSpaceApi.validateToken(id)
      d.toast(data.is_valid ? '验证通过' : '验证失败（无效 Token）', data.is_valid ? 'success' : 'error')
      await loadTokens()
    } catch (e: any) { d.toast(e.response?.data?.detail || '验证失败', 'error') }
  }

  // ── Space（分页） ──
  async function loadSpaces(service?: string, status?: string) {
    spacesLoading.value = true
    try {
      const { data } = await hfSpaceApi.listSpaces(service, spacePage.value, spacePageSize.value, status)
      spaces.value = data.items || []
      spaceTotal.value = data.total || 0
    } catch { /* ignore */ }
    finally { spacesLoading.value = false }
  }

  function spaceGoPage(p: number) {
    if (p < 1 || p > spaceTotalPages.value) return
    spacePage.value = p
    loadSpaces(spaceFilter.value || undefined, statusFilter.value || undefined)
  }

  async function addSpace() {
    try {
      await hfSpaceApi.addSpace(addSpaceForm)
      addSpaceForm.url = ''; addSpaceForm.repo_id = ''; addSpaceForm.token_id = 0
      d.toast('Space 已添加', 'success')
      await loadSpaces(spaceFilter.value || undefined, statusFilter.value || undefined); await loadOverview()
    } catch (e: any) { d.toast(e.response?.data?.detail || '添加失败', 'error') }
  }

  async function deleteSpace(id: number) {
    if (!confirm('确定要删除此 Space 吗？（将同时尝试删除远程 HF repo）')) return
    try {
      await hfSpaceApi.deleteSpace(id)
      d.toast('已删除', 'success')
      await loadSpaces(spaceFilter.value || undefined, statusFilter.value || undefined); await loadOverview()
    } catch (e: any) { d.toast(e.response?.data?.detail || '删除失败', 'error') }
  }

  async function checkHealth(service?: string) {
    healthLoading.value = true
    try {
      const { data } = await hfSpaceApi.checkHealth(service)
      healthResults.value = data.results || []
      d.toast(`健康检查完成（${data.total} 个）`, 'success')
      await loadSpaces(spaceFilter.value || undefined, statusFilter.value || undefined); await loadOverview()
    } catch (e: any) { d.toast(e.response?.data?.detail || '检查失败', 'error') }
    healthLoading.value = false
  }

  async function deploySpaces() {
    if (!deployForm.release_url.trim()) { d.toast('请输入 Release URL', 'error'); return }
    deployLoading.value = true; deployResult.value = null
    try {
      const { data } = await hfSpaceApi.deploySpaces(deployForm)
      deployResult.value = data
      d.toast(`部署完成: ${data.success} 成功, ${data.failed} 失败`, data.failed > 0 ? 'error' : 'success')
      await loadSpaces(spaceFilter.value || undefined, statusFilter.value || undefined); await loadOverview()
      // 部署成功后自动同步 CF Worker，让新 Space URL 生效
      if (data.success > 0) {
        await syncCF(deployForm.service)
      }
    } catch (e: any) { d.toast(e.response?.data?.detail || '部署失败', 'error') }
    deployLoading.value = false
  }

  async function updateSpaces(service: string) {
    updateLoading.value = true; updateResult.value = null
    try {
      const { data } = await hfSpaceApi.updateSpaces({ service })
      updateResult.value = data
      d.toast(`更新完成: ${data.updated} 成功, ${data.failed} 失败`, data.failed > 0 ? 'error' : 'success')
      await loadOverview()
    } catch (e: any) { d.toast(e.response?.data?.detail || '更新失败', 'error') }
    updateLoading.value = false
  }

  // ── 弹性管理 ──
  async function triggerAutoscale() {
    autoscaleLoading.value = true; autoscaleResult.value = null
    try {
      const { data } = await hfSpaceApi.autoscale(autoscaleForm)
      autoscaleResult.value = data
      d.toast('弹性管理完成', 'success')
      await loadOverview()
    } catch (e: any) { d.toast(e.response?.data?.detail || '操作失败', 'error') }
    autoscaleLoading.value = false
  }

  async function syncCF(service?: string) {
    syncCFLoading.value = true
    try {
      const { data } = await hfSpaceApi.syncCF(service)
      const results = data.results || []
      const detail = results.map((r: any) => `${r.service}(${r.env_key}): ${r.urls} 个 URL`).join('、')
      d.toast(data.message + (detail ? ' — ' + detail : ''), 'success')
    } catch (e: any) { d.toast(e.response?.data?.detail || '同步失败', 'error') }
    syncCFLoading.value = false
  }

  // ── 自动发现 ──
  const discoverLoading = ref(false)
  const discoverResult = ref<any>(null)

  async function discoverSpaces(defaultService?: string) {
    discoverLoading.value = true; discoverResult.value = null
    try {
      const { data: res } = await hfSpaceApi.discover(defaultService)
      discoverResult.value = res
      d.toast(`发现完成: 扫描 ${res.scanned} 个 Token，找到 ${res.found} 个 Space，新导入 ${res.imported} 个`, 'success')
      await loadAll()
    } catch (e: any) { d.toast(e.response?.data?.detail || '发现失败', 'error') }
    discoverLoading.value = false
  }

  const redetectLoading = ref(false)
  const redetectResult = ref<any>(null)

  async function redetectService() {
    redetectLoading.value = true; redetectResult.value = null
    try {
      const { data: res } = await hfSpaceApi.redetect()
      redetectResult.value = res
      const identified = res.imported || 0
      const total = res.found || 0
      d.toast(`识别完成: ${total} 个 unknown，成功识别 ${identified} 个`, 'success')
      await loadAll()
    } catch (e: any) { d.toast(e.response?.data?.detail || '识别失败', 'error') }
    redetectLoading.value = false
  }

  // ── 批量验证所有 Token ──
  async function validateAllTokens() {
    validateAllLoading.value = true
    try {
      const { data } = await hfSpaceApi.validateAllTokens()
      tokens.value = data.tokens || []
      d.toast(`验证完成: ${data.valid} 有效 / ${data.invalid} 失效`, data.invalid > 0 ? 'warning' : 'success')
    } catch (e: any) { d.toast(e.response?.data?.detail || '批量验证失败', 'error') }
    validateAllLoading.value = false
  }

  // ── 批量清理被封 Space ──
  async function purgeSpaces(service?: string) {
    purgeLoading.value = true
    try {
      const { data } = await hfSpaceApi.purgeSpaces(service)
      d.toast(data.message, data.deleted > 0 ? 'success' : 'info')
      await loadAll()
    } catch (e: any) { d.toast(e.response?.data?.detail || '清理失败', 'error') }
    purgeLoading.value = false
  }

  // ── 初始化加载（每次切到 tab 都拉最新数据） ──
  async function loadAll() {
    await Promise.all([loadOverview(), loadTokens(), loadSpaces()])
  }

  // 强制刷新（手动点刷新按钮时）
  function reloadSpaces() {
    spacePage.value = 1
    loadSpaces(spaceFilter.value || undefined, statusFilter.value || undefined)
  }

  // ── 批量删除 Space ──
  async function batchDeleteSpaces(ids: number[]) {
    if (!ids.length) return
    if (!confirm(`确定要删除选中的 ${ids.length} 个 Space 吗？（将同时尝试删除远程 HF repo）`)) return
    batchDeleteLoading.value = true
    let ok = 0, fail = 0
    for (const id of ids) {
      try { await hfSpaceApi.deleteSpace(id); ok++ }
      catch { fail++ }
    }
    batchDeleteLoading.value = false
    d.toast(`批量删除: ${ok} 成功, ${fail} 失败`, fail > 0 ? 'error' : 'success')
    await loadSpaces(spaceFilter.value || undefined, statusFilter.value || undefined)
    await loadOverview()
  }

  return {
    // 状态
    tokens, spaces, overview, healthResults, spaceFilter, statusFilter,
    tokensLoading, spacesLoading, overviewLoading, healthLoading,
    deployLoading, autoscaleLoading, syncCFLoading, updateLoading, updateService,
    validateAllLoading, purgeLoading, batchDeleteLoading,
    tokenForm, addSpaceForm, deployForm, deployResult, autoscaleForm, autoscaleResult, updateResult,
    // 分页
    spacePage, spacePageSize, spaceTotal, spaceTotalPages,
    // 方法
    loadOverview, loadTokens, loadSpaces, loadAll, reloadSpaces,
    spaceGoPage,
    createToken, deleteToken, validateToken, validateAllTokens,
    addSpace, deleteSpace, batchDeleteSpaces, checkHealth, deploySpaces, updateSpaces,
    triggerAutoscale, syncCF, purgeSpaces,
    discoverLoading, discoverResult, discoverSpaces,
    redetectLoading, redetectResult, redetectService,
  }
}
