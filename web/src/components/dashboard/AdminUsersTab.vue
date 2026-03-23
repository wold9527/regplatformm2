<template>
  <div v-show="adminTab === 'users'" class="space-y-4">
    <div class="flex items-center justify-between">
      <span class="text-xs font-bold text-t-secondary uppercase tracking-wider">用户列表</span>
      <div class="flex items-center gap-2">
        <input v-model="adminUserSearch" type="text" placeholder="搜索用户名 / 邮箱..."
          class="bg-s-inset border border-b-panel rounded-lg px-3 py-1 text-xs text-white focus:border-blue-500 focus:outline-none w-48 placeholder:text-t-faint">
        <span class="text-[10px] text-t-faint">共 {{ userTotal }} 人</span>
        <button @click="loadAdminUsers" class="text-[10px] text-t-muted hover:text-white transition px-2 py-1 rounded hover:bg-s-hover">刷新</button>
      </div>
    </div>
    <table class="w-full text-sm">
      <thead class="text-[10px] text-t-muted border-b border-b-panel">
        <tr>
          <th class="px-3 py-2 text-left">用户</th>
          <th class="px-3 py-2 text-left">角色</th>
          <th class="px-3 py-2 text-right cursor-pointer select-none hover:text-t-primary transition" @click="toggleUserSort('credits')">
            内部积分
            <span v-if="userSortField === 'credits'" class="ml-0.5">{{ userSortOrder === 'desc' ? '↓' : '↑' }}</span>
          </th>
          <th class="px-3 py-2 text-right cursor-pointer select-none hover:text-t-primary transition" @click="toggleUserSort('newapi_quota')">
            余额
            <span v-if="userSortField === 'newapi_quota'" class="ml-0.5">{{ userSortOrder === 'desc' ? '↓' : '↑' }}</span>
          </th>
          <th class="px-3 py-2 text-center">免费试用</th>
          <th class="px-3 py-2 text-center">管理员</th>
          <th class="px-3 py-2 text-right cursor-pointer select-none hover:text-t-primary transition" @click="toggleUserSort('created_at')">
            注册时间
            <span v-if="userSortField === 'created_at'" class="ml-0.5">{{ userSortOrder === 'desc' ? '↓' : '↑' }}</span>
          </th>
          <th class="px-3 py-2 text-right">操作</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-b-panel">
        <template v-for="u in filteredAdminUsers" :key="u.id">
          <tr class="hover:bg-s-hover transition cursor-pointer" @click="toggleUserDetail(u.id)">
            <td class="px-3 py-2.5">
              <div class="flex items-center gap-2">
                <svg class="w-3 h-3 text-t-faint transition-transform flex-none" :class="expandedUserId === u.id ? 'rotate-90' : ''" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"/></svg>
                <div>
                  <div class="text-sm font-medium">{{ u.name || u.username }}</div>
                  <div class="text-[10px] text-t-faint">@{{ u.username }}
                    <span v-if="u.newapi_id" class="text-t-faint font-mono"> · API#{{ u.newapi_id }}</span>
                  </div>
                </div>
              </div>
            </td>
            <td class="px-3 py-2.5"><span class="badge-base" :class="vipBadgeClassFor(u.role)">{{ vipLabelFor(u.role) }}</span></td>
            <td class="px-3 py-2.5 text-right font-mono text-sm text-warn">{{ u.credits || 0 }}</td>
            <td class="px-3 py-2.5 text-right font-mono text-sm" :class="u.newapi_quota > 0 ? 'text-emerald-400' : 'text-t-faint'">
              {{ u.newapi_id ? ('$' + (u.newapi_quota / 500000).toFixed(2)) : '—' }}
            </td>
            <td class="px-3 py-2.5 text-center text-[10px]">
              <span v-if="!u.free_trial_used" class="text-t-faint">未领取</span>
              <span v-else-if="u.free_trial_remaining > 0" class="text-warn">剩{{ u.free_trial_remaining }}</span>
              <span v-else class="text-t-faint">已用完</span>
            </td>
            <td class="px-3 py-2.5 text-center text-ok text-sm">{{ u.is_admin ? '✓' : '' }}</td>
            <td class="px-3 py-2.5 text-right text-[10px] text-t-faint">{{ new Date(u.created_at).toLocaleDateString() }}</td>
            <td class="px-3 py-2.5 text-right space-x-2" @click.stop>
              <button @click="adminRecharge(u.id, u.username)" class="text-xs text-info hover:text-blue-300">充值</button>
              <button @click="toggleAdminRole(u.id)" class="text-xs text-t-muted hover:text-white">切换管理</button>
            </td>
          </tr>
          <!-- 用户详情展开行 -->
          <tr v-if="expandedUserId === u.id">
            <td colspan="8" class="p-0">
              <div class="bg-s-inset px-6 py-4 animate-in border-t border-b border-b-panel">
                <div v-if="userDetailLoading" class="text-t-muted text-center py-6 text-xs">加载中...</div>
                <div v-else-if="userDetailData" class="space-y-4">
                  <!-- 用户概要 -->
                  <div class="grid grid-cols-4 gap-3">
                    <div class="glass-light rounded-lg p-2.5">
                      <div class="text-[10px] text-t-muted">总任务</div>
                      <div class="text-lg font-bold">{{ userDetailData.task_summary?.total_tasks || 0 }}</div>
                    </div>
                    <div class="glass-light rounded-lg p-2.5">
                      <div class="text-[10px] text-t-muted">总成功</div>
                      <div class="text-lg font-bold text-ok">{{ userDetailData.task_summary?.total_success || 0 }}</div>
                    </div>
                    <div class="glass-light rounded-lg p-2.5">
                      <div class="text-[10px] text-t-muted">总失败</div>
                      <div class="text-lg font-bold text-err">{{ userDetailData.task_summary?.total_fail || 0 }}</div>
                    </div>
                    <div class="glass-light rounded-lg p-2.5">
                      <div class="text-[10px] text-t-muted">成功率</div>
                      <div class="text-lg font-bold" :class="userSuccessRate >= 80 ? 'text-ok' : userSuccessRate >= 50 ? 'text-warn' : 'text-err'">
                        {{ userSuccessRate.toFixed(1) }}%
                      </div>
                    </div>
                  </div>

                  <!-- 各平台账号 -->
                  <div class="grid grid-cols-4 gap-3">
                    <div v-for="p in [
                      { key: 'grok', label: 'Grok', dot: 'bg-blue-500', text: 'text-info' },
                      { key: 'openai', label: 'OpenAI', dot: 'bg-green-500', text: 'text-ok' },
                      { key: 'kiro', label: 'Kiro', dot: 'bg-amber-500', text: 'text-warn' },
                      { key: 'gemini', label: 'Gemini', dot: 'bg-purple-500', text: 'text-purple-400' },
                    ]" :key="p.key" class="glass-light rounded-lg p-2.5">
                      <div class="flex items-center gap-1.5 mb-2">
                        <span class="w-2 h-2 rounded-full" :class="p.dot"></span>
                        <span class="text-[10px] font-bold" :class="p.text">{{ p.label }}</span>
                        <span class="text-[10px] text-t-faint font-mono">{{ (userDetailData.platform_results?.[p.key] || []).length }}</span>
                      </div>
                      <div class="space-y-1 max-h-36 overflow-y-auto scroll-thin">
                        <div v-if="(userDetailData.platform_results?.[p.key] || []).length === 0"
                          class="text-[10px] text-t-faint text-center py-3">无账号</div>
                        <!-- 平台账号列表：默认前 5 条，展开后全部显示 -->
                        <div v-for="r in (expandedPlatforms.has(`${expandedUserId}-${p.key}`)
                          ? (userDetailData.platform_results?.[p.key] || [])
                          : (userDetailData.platform_results?.[p.key] || []).slice(0, 5))"
                          :key="r.id"
                          class="flex items-center gap-1.5 px-1.5 py-1 rounded hover:bg-s-hover transition cursor-pointer group"
                          @click="copyUserDetailAccount(r)">
                          <div class="flex-1 min-w-0">
                            <div class="text-[10px] text-t-primary truncate">{{ r.email }}</div>
                            <div class="text-[10px] font-mono text-t-faint truncate">{{ getUserDetailPassword(r) }}</div>
                          </div>
                          <span v-if="r.is_archived" class="text-[8px] px-1 py-0.5 rounded bg-s-panel text-t-muted flex-none">归档</span>
                          <span class="opacity-0 group-hover:opacity-100 text-[10px] flex-none" :class="p.text">复制</span>
                        </div>
                        <!-- 超过 5 条时显示展开/收起按钮 -->
                        <button
                          v-if="(userDetailData.platform_results?.[p.key] || []).length > 5"
                          class="w-full text-center text-[10px] text-t-muted hover:text-white py-1 transition"
                          @click.stop="togglePlatformExpand(`${expandedUserId}-${p.key}`)">
                          {{ expandedPlatforms.has(`${expandedUserId}-${p.key}`)
                            ? '收起'
                            : `显示全部 (${(userDetailData.platform_results?.[p.key] || []).length})` }}
                        </button>
                      </div>
                    </div>
                  </div>

                  <!-- 最近交易 -->
                  <div>
                    <div class="text-[10px] text-t-muted uppercase tracking-wider mb-1.5">最近交易</div>
                    <div class="space-y-0.5 max-h-28 overflow-y-auto scroll-thin">
                      <div v-if="(userDetailData.recent_txs || []).length === 0"
                        class="text-[10px] text-t-faint text-center py-2">无交易记录</div>
                      <div v-for="tx in (userDetailData.recent_txs || [])" :key="tx.id"
                        class="flex items-center gap-2 text-[10px] px-1.5 py-0.5 rounded hover:bg-s-hover">
                        <span class="font-mono font-bold flex-none" :class="tx.amount > 0 ? 'text-ok' : 'text-err'">
                          {{ tx.amount > 0 ? '+' : '' }}{{ tx.amount }}
                        </span>
                        <span class="flex-none px-1 py-0.5 rounded text-[8px]"
                          :class="tx.type === 'purchase' ? 'bg-emerald-900/30 text-emerald-400' : tx.type === 'redeem' ? 'bg-amber-900/30 text-amber-400' : tx.type === 'consume' ? 'bg-red-900/30 text-red-400' : tx.type === 'refund' ? 'bg-cyan-900/30 text-cyan-400' : 'bg-blue-900/30 text-blue-400'">
                          {{ ({ purchase:'购买', redeem:'兑换', consume:'消费', refund:'退还', recharge:'充值', free_trial:'免费' } as any)[tx.type] || tx.type }}
                        </span>
                        <span class="text-t-muted flex-1 truncate">{{ tx.description }}</span>
                        <span class="text-t-faint flex-none">{{ tx.created_at }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </td>
          </tr>
        </template>
      </tbody>
    </table>
    <div v-if="adminUsers.length === 0" class="text-center text-t-faint text-xs py-6">暂无用户</div>

    <!-- 分页 -->
    <div v-if="userTotalPages > 1" class="flex items-center justify-between pt-3 border-t border-b-panel">
      <span class="text-[10px] text-t-faint">第 {{ userPage }} / {{ userTotalPages }} 页</span>
      <div class="flex items-center gap-1">
        <button @click="userGoPage(userPage - 1)" :disabled="userPage <= 1"
          class="px-2 py-1 text-[10px] rounded hover:bg-s-hover disabled:opacity-30 text-t-secondary transition">‹ 上一页</button>
        <template v-for="p in paginationRange" :key="p">
          <span v-if="p === '...'" class="px-1 text-[10px] text-t-faint">…</span>
          <button v-else @click="userGoPage(p as number)"
            class="min-w-[24px] h-6 text-[10px] rounded transition"
            :class="p === userPage ? 'bg-blue-600 text-white' : 'text-t-secondary hover:bg-s-hover'">{{ p }}</button>
        </template>
        <button @click="userGoPage(userPage + 1)" :disabled="userPage >= userTotalPages"
          class="px-2 py-1 text-[10px] rounded hover:bg-s-hover disabled:opacity-30 text-t-secondary transition">下一页 ›</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useDashboard } from '../../composables/useDashboard'
import { useAdmin } from '../../composables/useAdmin'

const { vipLabelFor, vipBadgeClassFor } = useDashboard()
const {
  adminTab, adminUsers, adminUserSearch, filteredAdminUsers,
  userSortField, userSortOrder, toggleUserSort,
  expandedUserId, userDetailLoading, userDetailData, userSuccessRate,
  userPage, userTotal, userTotalPages, paginationRange, userGoPage,
  loadAdminUsers, toggleUserDetail, adminRecharge, toggleAdminRole,
  getUserDetailPassword, copyUserDetailAccount,
} = useAdmin()

// 记录哪些平台已展开全部（key 格式：`${userId}-${platformKey}`）
const expandedPlatforms = ref(new Set<string>())

// 切换用户时重置展开状态
watch(expandedUserId, () => {
  expandedPlatforms.value = new Set()
})

// 切换某平台的展开/收起
function togglePlatformExpand(key: string) {
  const s = expandedPlatforms.value
  if (s.has(key)) {
    s.delete(key)
  } else {
    s.add(key)
  }
  // 触发 Vue 响应式更新（Set 内部变更不会自动触发）
  expandedPlatforms.value = new Set(s)
}
</script>
