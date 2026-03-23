/**
 * 卡池管理 — 管理后台数据层
 * 提供卡池的增删改查、验证、批量导入、清除无效卡等功能
 */
import { ref, reactive, computed } from 'vue'
import { cardPoolApi } from '../api/client'
import { useDashboard } from './useDashboard'

// ── 类型定义 ──

export interface CardEntry {
  id: number
  name: string
  card_number: string
  exp_month: number
  exp_year: number
  cvc: string
  billing_name: string
  billing_email: string
  billing_country: string
  billing_city: string
  billing_line1: string
  billing_zip: string
  provider: string
  source: string
  is_valid: boolean
  use_count: number
  fail_count: number
  last_used_at?: string
  last_valid_at?: string
}

export interface CardPoolStats {
  total: number
  valid: number
  invalid: number
}

// ── 模块级单例状态 ──

const cards = ref<CardEntry[]>([])
const cardStats = reactive<CardPoolStats>({ total: 0, valid: 0, invalid: 0 })
const loading = ref(false)
const statsLoading = ref(false)
const validateRunning = ref(false)
const purgeRunning = ref(false)
const filterMode = ref<'' | 'valid' | 'invalid'>('')
const selectedIds = ref<Set<number>>(new Set())

// 添加表单
const addForm = reactive({
  name: '', card_number: '', exp_month: 1, exp_year: 2026, cvc: '',
  billing_name: '', billing_email: '', billing_country: 'US',
  billing_city: '', billing_line1: '', billing_zip: '', provider: 'manual',
})
const addDialogOpen = ref(false)
const addLoading = ref(false)

// 批量导入
const importDialogOpen = ref(false)
const importText = ref('')
const importProvider = ref('manual')
const importLoading = ref(false)

// 分页
const currentPage = ref(1)
const pageSize = ref(50)
const totalCount = ref(0)

export function useCardPool() {
  const { toast } = useDashboard()

  // ── 加载 ──

  async function loadCards() {
    loading.value = true
    try {
      const { data } = await cardPoolApi.list({
        page: currentPage.value, page_size: pageSize.value,
        filter: filterMode.value || undefined,
      })
      cards.value = data.items || []
      totalCount.value = data.total || 0
    } catch (e: any) {
      toast(e.response?.data?.detail || '加载卡池失败', 'error')
    } finally { loading.value = false }
  }

  async function loadStats() {
    statsLoading.value = true
    try {
      const { data } = await cardPoolApi.stats()
      Object.assign(cardStats, data)
    } catch (e: any) {
      console.warn('加载卡池统计失败:', e.response?.data?.detail || e.message)
    } finally { statsLoading.value = false }
  }

  async function loadAll() { await Promise.all([loadCards(), loadStats()]) }

  // ── 操作 ──

  async function addCard() {
    if (!addForm.card_number.trim() || !addForm.cvc.trim()) return
    addLoading.value = true
    try {
      await cardPoolApi.add({
        name: addForm.name.trim() || undefined,
        card_number: addForm.card_number.trim(),
        exp_month: Number(addForm.exp_month), exp_year: Number(addForm.exp_year),
        cvc: addForm.cvc.trim(),
        billing_name: addForm.billing_name.trim() || undefined,
        billing_email: addForm.billing_email.trim() || undefined,
        billing_country: addForm.billing_country.trim() || undefined,
        billing_city: addForm.billing_city.trim() || undefined,
        billing_line1: addForm.billing_line1.trim() || undefined,
        billing_zip: addForm.billing_zip.trim() || undefined,
        provider: addForm.provider || undefined,
      })
      toast('卡片已添加', 'success')
      addDialogOpen.value = false; resetAddForm(); await loadAll()
    } catch (e: any) { toast(e.response?.data?.detail || '添加失败', 'error') }
    finally { addLoading.value = false }
  }

  function resetAddForm() {
    Object.assign(addForm, {
      name: '', card_number: '', exp_month: 1, exp_year: 2026, cvc: '',
      billing_name: '', billing_email: '', billing_country: 'US',
      billing_city: '', billing_line1: '', billing_zip: '', provider: 'manual',
    })
  }

  async function deleteCard(id: number) {
    if (!confirm('确定要删除此卡片吗？')) return
    try {
      await cardPoolApi.delete(id); toast('已删除', 'success')
      selectedIds.value.delete(id); await loadAll()
    } catch (e: any) { toast(e.response?.data?.detail || '删除失败', 'error') }
  }

  async function batchDelete() {
    const ids = [...selectedIds.value]
    if (!ids.length || !confirm(`确定要删除选中的 ${ids.length} 张卡吗？`)) return
    try {
      await cardPoolApi.batchDelete(ids)
      toast(`已删除 ${ids.length} 张卡`, 'success'); selectedIds.value.clear(); await loadAll()
    } catch (e: any) { toast(e.response?.data?.detail || '批量删除失败', 'error') }
  }

  async function importCards() {
    const text = importText.value.trim()
    if (!text) return
    importLoading.value = true
    try {
      const { data } = await cardPoolApi.import(text, importProvider.value)
      toast(data.message || `已导入 ${data.imported || 0} 张卡`, 'success')
      importDialogOpen.value = false; importText.value = ''; await loadAll()
    } catch (e: any) { toast(e.response?.data?.detail || '导入失败', 'error') }
    finally { importLoading.value = false }
  }

  async function triggerValidate() {
    if (validateRunning.value) return
    validateRunning.value = true
    try {
      const ids = selectedIds.value.size > 0 ? [...selectedIds.value] : cards.value.map(c => c.id)
      const { data } = await cardPoolApi.validate(ids)
      toast(data.message || '验证已完成', 'success')
      setTimeout(() => loadAll(), 3000)
    } catch (e: any) { toast(e.response?.data?.detail || '触发验证失败', 'error') }
    finally { setTimeout(() => { validateRunning.value = false }, 3000) }
  }

  async function purgeInvalid() {
    if (!confirm('确定要清除所有无效卡吗？')) return
    purgeRunning.value = true
    try {
      const { data } = await cardPoolApi.purge()
      toast(data.message || '无效卡已清除', 'success'); await loadAll()
    } catch (e: any) { toast(e.response?.data?.detail || '清除失败', 'error') }
    finally { purgeRunning.value = false }
  }

  // ── 选择 / 分页 / 工具 ──

  function toggleSelect(id: number) {
    if (selectedIds.value.has(id)) selectedIds.value.delete(id)
    else selectedIds.value.add(id)
  }
  function toggleSelectAll() {
    if (selectedIds.value.size === cards.value.length) selectedIds.value.clear()
    else selectedIds.value = new Set(cards.value.map(c => c.id))
  }

  const totalPages = computed(() => Math.max(1, Math.ceil(totalCount.value / pageSize.value)))
  function goToPage(p: number) {
    if (p < 1 || p > totalPages.value) return
    currentPage.value = p; loadCards()
  }

  function maskCardNumber(num: string): string {
    if (!num || num.length < 4) return num || ''
    return '**** **** **** ' + num.slice(-4)
  }

  function formatTime(dateStr?: string): string {
    if (!dateStr) return '-'
    const d = new Date(dateStr)
    const diff = Date.now() - d.getTime()
    const min = Math.floor(diff / 60000)
    if (min < 1) return '刚刚'
    if (min < 60) return `${min}分钟前`
    const hr = Math.floor(min / 60)
    if (hr < 24) return `${hr}小时前`
    return `${Math.floor(hr / 24)}天前`
  }

  return {
    cards, cardStats, loading, statsLoading,
    validateRunning, purgeRunning, filterMode, selectedIds,
    addForm, addDialogOpen, addLoading,
    importDialogOpen, importText, importProvider, importLoading,
    currentPage, pageSize, totalCount, totalPages,
    loadCards, loadStats, loadAll,
    addCard, resetAddForm, deleteCard, batchDelete, importCards,
    triggerValidate, purgeInvalid,
    toggleSelect, toggleSelectAll, goToPage,
    maskCardNumber, formatTime,
  }
}
