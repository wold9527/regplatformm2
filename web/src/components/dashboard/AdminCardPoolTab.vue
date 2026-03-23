<template>
  <div v-show="adminTab === 'cardpool'" class="space-y-4">

    <!-- 统计卡片 -->
    <div class="grid grid-cols-3 gap-3 mobile-grid-2">
      <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
        <div class="flex items-center justify-between">
          <span class="text-[10px] text-t-muted font-medium tracking-wide">总卡量</span>
          <span class="w-5 h-5 rounded-md bg-blue-500/15 flex items-center justify-center">
            <svg class="w-3 h-3 text-info" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20"/>
            </svg>
          </span>
        </div>
        <div class="text-2xl font-bold tabular-nums leading-none">{{ cardStats.total }}</div>
        <div class="text-[10px] text-t-faint">卡池总量</div>
      </div>
      <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
        <div class="flex items-center justify-between">
          <span class="text-[10px] text-t-muted font-medium tracking-wide">有效</span>
          <span class="w-5 h-5 rounded-md bg-green-500/15 flex items-center justify-center">
            <svg class="w-3 h-3 text-ok" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="20 6 9 17 4 12"/>
            </svg>
          </span>
        </div>
        <div class="text-2xl font-bold tabular-nums text-ok leading-none">{{ cardStats.valid }}</div>
        <div class="text-[10px] text-t-faint">
          占比 <span class="text-ok font-medium">{{ cardStats.total > 0 ? ((cardStats.valid / cardStats.total) * 100).toFixed(0) : 0 }}%</span>
        </div>
      </div>
      <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
        <div class="flex items-center justify-between">
          <span class="text-[10px] text-t-muted font-medium tracking-wide">无效</span>
          <span class="w-5 h-5 rounded-md bg-red-500/15 flex items-center justify-center">
            <svg class="w-3 h-3 text-err" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </span>
        </div>
        <div class="text-2xl font-bold tabular-nums text-err leading-none">{{ cardStats.invalid }}</div>
        <div class="text-[10px] text-t-faint">可清除</div>
      </div>
    </div>

    <!-- 操作栏 -->
    <div class="flex items-center gap-2 flex-wrap">
      <!-- 过滤器 -->
      <div class="flex bg-s-inset rounded-lg p-0.5 gap-0.5">
        <button @click="currentPage = 1; filterMode = ''; loadCards()"
          class="text-[11px] px-3 py-1 rounded-md transition font-medium"
          :class="filterMode === '' ? 'bg-blue-600/80 text-white shadow' : 'text-t-muted hover:text-t-primary'">
          全部
        </button>
        <button @click="currentPage = 1; filterMode = 'valid'; loadCards()"
          class="text-[11px] px-3 py-1 rounded-md transition font-medium"
          :class="filterMode === 'valid' ? 'bg-green-600/80 text-white shadow' : 'text-t-muted hover:text-t-primary'">
          有效
        </button>
        <button @click="currentPage = 1; filterMode = 'invalid'; loadCards()"
          class="text-[11px] px-3 py-1 rounded-md transition font-medium"
          :class="filterMode === 'invalid' ? 'bg-red-600/80 text-white shadow' : 'text-t-muted hover:text-t-primary'">
          无效
        </button>
      </div>
      <div class="flex-1"></div>
      <!-- 批量删除 -->
      <button v-if="selectedIds.size > 0" @click="batchDelete"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-red-600/80 hover:bg-red-500/80 text-white transition flex items-center gap-1.5">
        删除选中 ({{ selectedIds.size }})
      </button>
      <!-- 添加卡片 -->
      <button @click="addDialogOpen = true"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-blue-600/80 hover:bg-blue-500/80 text-white transition flex items-center gap-1.5">
        添加卡片
      </button>
      <!-- 批量导入 -->
      <button @click="importDialogOpen = true"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-amber-600/80 hover:bg-amber-500/80 text-white transition flex items-center gap-1.5">
        批量导入
      </button>
      <!-- 验证 -->
      <button @click="triggerValidate" :disabled="validateRunning"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-cyan-600/80 hover:bg-cyan-500/80 disabled:bg-gray-700 disabled:text-gray-500 text-white transition flex items-center gap-1.5">
        {{ validateRunning ? '验证中...' : selectedIds.size > 0 ? `验证选中 (${selectedIds.size})` : '验证本页' }}
      </button>
      <!-- 清除无效 -->
      <button @click="purgeInvalid" :disabled="purgeRunning || cardStats.invalid === 0"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-red-600/60 hover:bg-red-500/80 disabled:bg-gray-700 disabled:text-gray-500 text-white transition flex items-center gap-1.5">
        {{ purgeRunning ? '清除中...' : `清除无效 (${cardStats.invalid})` }}
      </button>
      <!-- 刷新 -->
      <button @click="loadAll" :disabled="loading"
        class="px-2 py-1.5 rounded-lg text-xs text-t-muted hover:text-t-secondary bg-s-inset hover:bg-s-hover transition flex items-center gap-1">
        <svg class="w-3.5 h-3.5" :class="loading ? 'animate-spin' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
        </svg>
        刷新
      </button>
    </div>

    <!-- 卡片列表表格 -->
    <div class="glass-light rounded-xl overflow-hidden">
      <!-- 表头 -->
      <div class="grid gap-2 px-3 py-2 border-b border-b-panel text-[10px] text-t-faint uppercase tracking-wider font-semibold"
        style="grid-template-columns: 1.5rem 2fr 2.5fr 3rem 4rem 4.5rem 3rem 5rem 4rem">
        <div class="flex items-center justify-center">
          <button @click="toggleSelectAll"
            class="w-3.5 h-3.5 rounded border flex items-center justify-center transition"
            :class="selectedIds.size > 0 && selectedIds.size === cards.length
              ? 'bg-blue-600 border-blue-500'
              : selectedIds.size > 0 ? 'bg-blue-600/50 border-blue-500' : 'border-b-panel hover:border-blue-500/60'">
            <svg v-if="selectedIds.size > 0" class="w-2.5 h-2.5 text-white" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
            </svg>
          </button>
        </div>
        <div>名称</div>
        <div>卡号</div>
        <div>有效期</div>
        <div>来源</div>
        <div>状态</div>
        <div>使用</div>
        <div>最近使用</div>
        <div>操作</div>
      </div>

      <!-- 加载中 -->
      <div v-if="loading" class="py-12 text-center">
        <div class="flex flex-col items-center gap-2">
          <svg class="w-5 h-5 text-t-faint animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
          </svg>
          <span class="text-[11px] text-t-faint">加载中...</span>
        </div>
      </div>

      <!-- 空状态 -->
      <div v-else-if="cards.length === 0" class="py-12 text-center">
        <div class="flex flex-col items-center gap-3">
          <div class="w-10 h-10 rounded-full bg-s-hover flex items-center justify-center">
            <svg class="w-5 h-5 text-t-faint" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20"/>
            </svg>
          </div>
          <span class="text-[11px] text-t-faint">暂无卡片，请添加或导入</span>
        </div>
      </div>

      <!-- 卡片行 -->
      <div v-for="card in cards" :key="card.id"
        class="grid gap-2 px-3 py-2 border-b border-b-panel last:border-b-0 hover:bg-s-hover transition-colors duration-100 items-center group"
        style="grid-template-columns: 1.5rem 2fr 2.5fr 3rem 4rem 4.5rem 3rem 5rem 4rem"
        :class="selectedIds.has(card.id) ? 'bg-blue-900/10' : ''">
        <div class="flex items-center justify-center">
          <button @click="toggleSelect(card.id)"
            class="w-3.5 h-3.5 rounded border flex items-center justify-center transition"
            :class="selectedIds.has(card.id) ? 'bg-blue-600 border-blue-500' : 'border-b-panel hover:border-blue-500/60'">
            <svg v-if="selectedIds.has(card.id)" class="w-2.5 h-2.5 text-white" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
            </svg>
          </button>
        </div>
        <div class="text-[11px] text-t-secondary truncate">{{ card.name || '-' }}</div>
        <div class="text-[11px] font-mono text-t-primary truncate">{{ maskCardNumber(card.card_number) }}</div>
        <div class="text-[10px] text-t-faint font-mono">{{ String(card.exp_month).padStart(2,'0') }}/{{ String(card.exp_year).slice(-2) }}</div>
        <div class="text-[10px] text-t-faint truncate">{{ card.provider || '-' }}</div>
        <div>
          <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full flex items-center gap-1 w-fit"
            :class="card.is_valid ? 'bg-green-500/15 text-ok' : 'bg-red-500/15 text-err'">
            <span class="w-1 h-1 rounded-full flex-none" :class="card.is_valid ? 'bg-ok' : 'bg-err'"></span>
            {{ card.is_valid ? '有效' : '无效' }}
          </span>
          <div v-if="card.fail_count > 0" class="text-[9px] text-t-faint mt-0.5">失败 {{ card.fail_count }} 次</div>
        </div>
        <div class="text-[10px] text-t-faint tabular-nums">{{ card.use_count }}</div>
        <div class="text-[10px] text-t-faint">{{ formatTime(card.last_used_at) }}</div>
        <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 max-sm:opacity-100 transition-opacity">
          <button @click="deleteCard(card.id)"
            class="p-1 rounded hover:bg-red-900/30 text-t-muted hover:text-err transition tip tip-left"
            data-tip="删除此卡片">
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
            </svg>
          </button>
        </div>
      </div>

      <!-- 分页栏 -->
      <div v-if="totalCount > 0" class="px-3 py-2 border-t border-b-panel flex items-center justify-between">
        <span class="text-[10px] text-t-faint tabular-nums">共 {{ totalCount }} 条</span>
        <div class="flex items-center gap-2">
          <button @click="goToPage(currentPage - 1)" :disabled="currentPage <= 1"
            class="px-2.5 py-1 rounded-md text-[11px] font-medium transition"
            :class="currentPage > 1 ? 'text-t-secondary hover:bg-s-hover' : 'text-t-faint opacity-40'">
            &larr; 上一页
          </button>
          <span class="text-[11px] text-t-secondary tabular-nums">{{ currentPage }} / {{ totalPages }}</span>
          <button @click="goToPage(currentPage + 1)" :disabled="currentPage >= totalPages"
            class="px-2.5 py-1 rounded-md text-[11px] font-medium transition"
            :class="currentPage < totalPages ? 'text-t-secondary hover:bg-s-hover' : 'text-t-faint opacity-40'">
            下一页 &rarr;
          </button>
        </div>
      </div>
    </div>

    <!-- 添加卡片弹窗 -->
    <Teleport to="body">
      <Transition name="queue-modal">
        <div v-if="addDialogOpen"
          class="fixed inset-0 z-[9999] flex items-center justify-center"
          role="dialog" aria-modal="true" aria-label="添加卡片" tabindex="-1"
          @click.self="addDialogOpen = false" @keydown.escape="addDialogOpen = false">
          <div class="absolute inset-0 bg-black/60 backdrop-blur-sm"></div>
          <div class="relative w-[26rem] rounded-2xl overflow-hidden shadow-2xl"
            style="background:var(--bg-admin);border:1px solid var(--border-glass)">
            <div class="px-5 py-4 border-b border-b-panel flex items-center justify-between">
              <span class="text-sm font-bold text-t-primary">添加卡片</span>
              <button @click="addDialogOpen = false" class="text-t-muted hover:text-white transition p-1">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="p-5 space-y-3">
              <!-- 卡号 / CVC -->
              <div class="grid grid-cols-3 gap-2">
                <div class="col-span-2 space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">卡号 *</label>
                  <input v-model="addForm.card_number" type="text" placeholder="4242424242424242"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs font-mono outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">CVC *</label>
                  <input v-model="addForm.cvc" type="text" placeholder="123"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs font-mono outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
              </div>
              <!-- 有效期 / 名称 -->
              <div class="grid grid-cols-4 gap-2">
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">月</label>
                  <input v-model.number="addForm.exp_month" type="number" min="1" max="12" placeholder="01"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition focus:border-blue-500/60">
                </div>
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">年</label>
                  <input v-model.number="addForm.exp_year" type="number" placeholder="2026"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition focus:border-blue-500/60">
                </div>
                <div class="col-span-2 space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">别名</label>
                  <input v-model="addForm.name" type="text" placeholder="可选"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
              </div>
              <!-- 账单信息 -->
              <div class="grid grid-cols-2 gap-2">
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">持卡人</label>
                  <input v-model="addForm.billing_name" type="text" placeholder="John Doe"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">邮箱</label>
                  <input v-model="addForm.billing_email" type="email" placeholder="john@example.com"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
              </div>
              <div class="grid grid-cols-3 gap-2">
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">国家</label>
                  <input v-model="addForm.billing_country" type="text" placeholder="US"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">城市</label>
                  <input v-model="addForm.billing_city" type="text" placeholder="New York"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">邮编</label>
                  <input v-model="addForm.billing_zip" type="text" placeholder="10001"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
              </div>
              <div class="flex gap-2 pt-1">
                <button @click="addDialogOpen = false"
                  class="flex-1 py-2 rounded-xl text-xs font-medium text-t-secondary glass-light hover:bg-s-hover transition">
                  取消
                </button>
                <button @click="addCard" :disabled="!addForm.card_number || !addForm.cvc || addLoading"
                  class="flex-1 py-2 rounded-xl text-xs font-bold text-white transition disabled:bg-gray-700 disabled:text-gray-500"
                  style="background: linear-gradient(135deg, #2563eb, #1d4ed8);">
                  {{ addLoading ? '添加中...' : '添加卡片' }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- 批量导入弹窗 -->
    <Teleport to="body">
      <Transition name="queue-modal">
        <div v-if="importDialogOpen"
          class="fixed inset-0 z-[9999] flex items-center justify-center"
          role="dialog" aria-modal="true" aria-label="批量导入卡片" tabindex="-1"
          @click.self="importDialogOpen = false" @keydown.escape="importDialogOpen = false">
          <div class="absolute inset-0 bg-black/60 backdrop-blur-sm"></div>
          <div class="relative w-[30rem] rounded-2xl overflow-hidden shadow-2xl"
            style="background:var(--bg-admin);border:1px solid var(--border-glass)">
            <div class="px-5 py-4 border-b border-b-panel flex items-center justify-between">
              <span class="text-sm font-bold text-t-primary">批量导入卡片</span>
              <button @click="importDialogOpen = false" class="text-t-muted hover:text-white transition p-1">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="p-5 space-y-3">
              <!-- 来源 -->
              <div class="space-y-1">
                <label class="text-[10px] text-t-faint uppercase tracking-wider">来源/提供商</label>
                <input v-model="importProvider" type="text" placeholder="manual"
                  class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-amber-500/60">
              </div>
              <!-- 多行文本 -->
              <div class="space-y-1">
                <label class="text-[10px] text-t-faint uppercase tracking-wider">卡片列表（每行一张，管道分隔）</label>
                <div class="text-[10px] text-t-faint mb-1.5 leading-relaxed">
                  格式：<code class="text-accent bg-s-inset px-1 rounded">card_number|exp_month|exp_year|cvc</code>
                  或带账单 <code class="text-accent bg-s-inset px-1 rounded">card_number|exp_month|exp_year|cvc|name|email|country|city|line1|zip</code>
                </div>
                <textarea v-model="importText" rows="10"
                  placeholder="4242424242424242|12|2026|123&#10;5555555555554444|06|2027|456|John|john@test.com|US"
                  class="w-full bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-xs font-mono outline-none transition placeholder:text-t-faint focus:border-amber-500/60 resize-none scroll-thin"></textarea>
                <div class="text-[10px] text-t-faint">
                  共 {{ importText.split('\n').filter(l => l.trim()).length }} 行
                </div>
              </div>
              <div class="flex gap-2 pt-1">
                <button @click="importDialogOpen = false"
                  class="flex-1 py-2 rounded-xl text-xs font-medium text-t-secondary glass-light hover:bg-s-hover transition">
                  取消
                </button>
                <button @click="importCards" :disabled="!importText.trim() || importLoading"
                  class="flex-1 py-2 rounded-xl text-xs font-bold text-white transition disabled:bg-gray-700 disabled:text-gray-500"
                  style="background: linear-gradient(135deg, #d97706, #b45309);">
                  {{ importLoading ? '导入中...' : '开始导入' }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useAdmin } from '../../composables/useAdmin'
import { useCardPool } from '../../composables/useCardPool'

const { adminTab } = useAdmin()

const {
  cards, cardStats, loading,
  validateRunning, purgeRunning, filterMode, selectedIds,
  addForm, addDialogOpen, addLoading,
  importDialogOpen, importText, importProvider, importLoading,
  currentPage, totalCount, totalPages,
  loadCards, loadAll,
  addCard, deleteCard, batchDelete, importCards,
  triggerValidate, purgeInvalid,
  toggleSelect, toggleSelectAll, goToPage,
  maskCardNumber, formatTime,
} = useCardPool()

onMounted(() => { loadAll() })
</script>
