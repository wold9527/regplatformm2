<template>
  <div v-show="adminTab === 'ops'" class="space-y-6">

    <!-- ═══ 兑换码区域 ═══ -->
    <div class="glass-light rounded-xl p-4 space-y-3">
      <div class="text-xs font-bold text-t-secondary uppercase tracking-wider">生成兑换码</div>
      <div class="grid grid-cols-3 gap-3 mobile-grid-1">
        <div>
          <label class="text-[10px] text-t-muted mb-1 block">数量</label>
          <input v-model.number="codeForm.count" type="number" min="1" max="100"
            class="w-full bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-blue-500 focus:outline-none">
        </div>
        <div>
          <label class="text-[10px] text-t-muted mb-1 block">每张次数</label>
          <input v-model.number="codeForm.credits_per_code" type="number" min="1" max="1000"
            class="w-full bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-blue-500 focus:outline-none">
        </div>
        <div>
          <label class="text-[10px] text-t-muted mb-1 block">批次名称</label>
          <input v-model="codeForm.batch_name" type="text" placeholder="可选"
            class="w-full bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-blue-500 focus:outline-none">
        </div>
      </div>
      <button @click="generateCodes"
        class="w-full py-2 rounded-lg text-xs font-bold bg-gradient-to-r from-amber-600 to-orange-600 hover:from-amber-500 hover:to-orange-500 text-white transition">
        生成兑换码
      </button>
    </div>

    <!-- 刚生成的兑换码 -->
    <div v-if="generatedCodes.length > 0" class="glass-light rounded-xl p-4 space-y-3">
      <div class="flex items-center justify-between">
        <div class="text-xs font-bold text-t-secondary uppercase tracking-wider">刚生成的兑换码</div>
        <button @click="copyAllCodes" class="text-[10px] text-info hover:text-blue-300 px-2 py-1 rounded hover:bg-s-hover transition">复制全部</button>
      </div>
      <div class="grid grid-cols-2 gap-1.5 max-h-40 overflow-y-auto scroll-thin">
        <div v-for="c in generatedCodes" :key="c"
          class="font-mono text-[11px] text-warn bg-s-inset rounded px-2 py-1 cursor-pointer hover:bg-s-hover transition"
          @click="copyText(c); toast('已复制', 'success')">{{ c }}</div>
      </div>
    </div>

    <!-- 兑换码记录 -->
    <div class="glass-light rounded-xl overflow-hidden">
      <div class="px-4 py-2.5 flex items-center justify-between border-b border-b-panel">
        <div class="flex items-center gap-2">
          <span class="text-xs font-bold text-t-secondary uppercase tracking-wider">兑换码记录</span>
          <span class="text-[10px] font-mono text-t-faint">{{ codeTotal }}</span>
        </div>
        <button @click="loadAdminCodes" class="text-[10px] text-t-muted hover:text-white transition px-2 py-1 rounded hover:bg-s-hover">刷新</button>
      </div>
      <table class="w-full text-sm" v-if="adminCodes.length > 0">
        <thead class="text-[10px] text-t-muted border-b border-b-panel">
          <tr>
            <th class="px-2 py-2 text-left">兑换码</th>
            <th class="px-2 py-2 text-center">次数</th>
            <th class="px-2 py-2 text-center">状态</th>
            <th class="px-2 py-2 text-left">批次</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-b-panel">
          <tr v-for="c in adminCodes" :key="c.id" class="hover:bg-s-hover transition">
            <td class="px-2 py-2 font-mono text-[11px] text-warn cursor-pointer hover:text-amber-200"
              @click="copyText(c.code); toast('已复制', 'success')">{{ c.code }}</td>
            <td class="px-2 py-2 text-center text-xs">{{ c.credits }}</td>
            <td class="px-2 py-2 text-center">
              <span class="text-[10px] px-1.5 py-0.5 rounded"
                :class="c.is_used ? 'bg-s-panel text-t-secondary' : 'bg-ok-dim text-ok'">
                {{ c.is_used ? '已使用' : '可用' }}
              </span>
            </td>
            <td class="px-2 py-2 text-[10px] text-t-muted">{{ c.batch_name }}</td>
          </tr>
        </tbody>
      </table>
      <div v-else class="text-center text-t-faint text-xs py-6">暂无兑换码</div>
      <!-- 分页 -->
      <div v-if="codeTotalPages > 1" class="flex items-center justify-center gap-1 px-4 py-2.5 border-t border-b-panel">
        <button @click="codeGoPage(codePage - 1)" :disabled="codePage <= 1"
          class="text-[10px] px-2 py-1 rounded transition disabled:opacity-30"
          :class="codePage > 1 ? 'text-t-muted hover:text-white hover:bg-s-hover' : ''">上一页</button>
        <span class="text-[10px] text-t-faint font-mono px-2">{{ codePage }} / {{ codeTotalPages }}</span>
        <button @click="codeGoPage(codePage + 1)" :disabled="codePage >= codeTotalPages"
          class="text-[10px] px-2 py-1 rounded transition disabled:opacity-30"
          :class="codePage < codeTotalPages ? 'text-t-muted hover:text-white hover:bg-s-hover' : ''">下一页</button>
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import { useDashboard } from '../../composables/useDashboard'
import { useAdmin } from '../../composables/useAdmin'

const { toast, copyText } = useDashboard()
const {
  adminTab,
  codeForm, generatedCodes, adminCodes,
  codePage, codeTotal, codeTotalPages,
  generateCodes, loadAdminCodes, copyAllCodes, codeGoPage,
} = useAdmin()
</script>
