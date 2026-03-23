<template>
  <Teleport to="body">
    <div v-if="showAdmin && user.is_admin"
      class="fixed inset-0 z-50 modal-mask flex items-center justify-center p-6 mobile-admin-mask"
      @click.self="showAdmin = false">
      <div class="admin-panel rounded-2xl w-full max-w-5xl max-h-[85vh] flex flex-col animate-in shadow-2xl mobile-admin-panel" @click.stop>
        <div class="flex-none flex items-center justify-between px-6 py-4 border-b border-b-panel mobile-admin-header">
          <div class="flex items-center gap-4 min-w-0 flex-1">
            <div class="text-base font-bold text-t-primary flex-none">管理后台</div>
            <div class="flex gap-1 overflow-x-auto scroll-thin flex-1 min-w-0 pb-0.5">
              <button @click="adminTab = 'overview'"
                class="text-[11px] px-3 py-1 rounded-md transition font-medium"
                :class="adminTab === 'overview' ? 'bg-blue-600/80 text-white' : 'text-t-muted hover:text-t-primary'">概览</button>
              <button @click="adminTab = 'users'; loadAdminUsers()"
                class="text-[11px] px-3 py-1 rounded-md transition font-medium"
                :class="adminTab === 'users' ? 'bg-blue-600/80 text-white' : 'text-t-muted hover:text-t-primary'">用户</button>
              <button @click="adminTab = 'settings'; loadAdminSettings()"
                class="text-[11px] px-3 py-1 rounded-md transition font-medium"
                :class="adminTab === 'settings' ? 'bg-blue-600/80 text-white' : 'text-t-muted hover:text-t-primary'">系统设置</button>
              <button @click="adminTab = 'ops'; loadOpsTab()"
                class="text-[11px] px-3 py-1 rounded-md transition font-medium"
                :class="adminTab === 'ops' ? 'bg-amber-600/80 text-white' : 'text-t-muted hover:text-t-primary'">运营</button>
              <button @click="adminTab = 'notify'; loadNotifyTab()"
                class="text-[11px] px-3 py-1 rounded-md transition font-medium"
                :class="adminTab === 'notify' ? 'bg-teal-600/80 text-white' : 'text-t-muted hover:text-t-primary'">通知公告</button>
              <button @click="adminTab = 'cleanup'; loadDataStats()"
                class="text-[11px] px-3 py-1 rounded-md transition font-medium"
                :class="adminTab === 'cleanup' ? 'bg-red-600/80 text-white' : 'text-t-muted hover:text-t-primary'">数据清理</button>
              <button @click="adminTab = 'realtime'; startRealtimePolling()"
                class="text-[11px] px-3 py-1 rounded-md transition font-medium"
                :class="adminTab === 'realtime' ? 'bg-cyan-600/80 text-white' : 'text-t-muted hover:text-t-primary'">实时状态</button>
              <button @click="adminTab = 'hfspace'; loadHFSpace()"
                class="text-[11px] px-3 py-1 rounded-md transition font-medium"
                :class="adminTab === 'hfspace' ? 'bg-purple-500/80 text-white' : 'text-t-muted hover:text-t-primary'">HF 空间</button>
              <button @click="adminTab = 'proxypool'; loadProxyPoolTab()"
                class="text-[11px] px-3 py-1 rounded-md transition font-medium"
                :class="adminTab === 'proxypool' ? 'bg-teal-600/80 text-white' : 'text-t-muted hover:text-t-primary'">代理池</button>
              <button @click="adminTab = 'cardpool'; loadCardPoolTab()"
                class="text-[11px] px-3 py-1 rounded-md transition font-medium"
                :class="adminTab === 'cardpool' ? 'bg-orange-600/80 text-white' : 'text-t-muted hover:text-t-primary'">卡池</button>
            </div>
          </div>
          <button @click="showAdmin = false" class="text-t-muted hover:text-white transition p-1">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
          </button>
        </div>
        <div class="flex-1 overflow-y-auto scroll-thin p-6 space-y-6 mobile-admin-body">

          <!-- 概览 Tab -->
          <div v-show="adminTab === 'overview'" class="space-y-4">

            <!-- 第一行：6 核心指标卡片 -->
            <div class="grid grid-cols-6 gap-2.5 mobile-grid-2">
              <!-- 总用户 -->
              <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
                <div class="flex items-center justify-between">
                  <span class="text-[10px] text-t-muted font-medium tracking-wide">总用户</span>
                  <span class="w-5 h-5 rounded-md bg-blue-500/15 flex items-center justify-center">
                    <svg class="w-3 h-3 text-info" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
                  </span>
                </div>
                <div class="text-2xl font-bold tabular-nums leading-none">{{ adminStats.total_users || 0 }}</div>
                <div class="text-[10px] text-t-faint">
                  活跃 <span class="text-info font-medium">{{ adminStats.active_7d || 0 }}</span> / <span class="text-t-secondary">{{ adminStats.active_30d || 0 }}</span>
                  <span class="text-t-faint"> 7/30日</span>
                </div>
              </div>

              <!-- 总任务 -->
              <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
                <div class="flex items-center justify-between">
                  <span class="text-[10px] text-t-muted font-medium tracking-wide">总任务</span>
                  <span class="w-5 h-5 rounded-md bg-cyan-500/15 flex items-center justify-center">
                    <svg class="w-3 h-3 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg>
                  </span>
                </div>
                <div class="text-2xl font-bold tabular-nums leading-none">{{ adminStats.total_tasks || 0 }}</div>
                <div class="text-[10px] text-t-faint">今日 <span class="text-accent font-medium">{{ adminStats.today_tasks || 0 }}</span> 个</div>
              </div>

              <!-- 成功注册 -->
              <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
                <div class="flex items-center justify-between">
                  <span class="text-[10px] text-t-muted font-medium tracking-wide">成功注册</span>
                  <span class="w-5 h-5 rounded-md bg-green-500/15 flex items-center justify-center">
                    <svg class="w-3 h-3 text-ok" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>
                  </span>
                </div>
                <div class="text-2xl font-bold tabular-nums text-ok leading-none">{{ adminStats.total_success || 0 }}</div>
                <div class="text-[10px] text-t-faint">今日 <span class="text-ok font-medium">{{ adminStats.today_results || 0 }}</span> 个</div>
              </div>

              <!-- 成功率 -->
              <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
                <div class="flex items-center justify-between">
                  <span class="text-[10px] text-t-muted font-medium tracking-wide">成功率</span>
                  <span class="w-5 h-5 rounded-md flex items-center justify-center"
                    :class="(adminStats.success_rate||0)>=80?'bg-green-500/15':(adminStats.success_rate||0)>=50?'bg-amber-500/15':'bg-red-500/15'">
                    <svg class="w-3 h-3" :class="(adminStats.success_rate||0)>=80?'text-ok':(adminStats.success_rate||0)>=50?'text-warn':'text-err'"
                      viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
                  </span>
                </div>
                <div class="text-2xl font-bold tabular-nums leading-none"
                  :class="(adminStats.success_rate||0)>=80?'text-ok':(adminStats.success_rate||0)>=50?'text-warn':'text-err'">
                  {{ (adminStats.success_rate || 0).toFixed(1) }}%
                </div>
                <div class="text-[10px] text-t-faint">失败 <span class="text-err font-medium">{{ adminStats.total_fail || 0 }}</span> 次</div>
              </div>

              <!-- 运行中 -->
              <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
                <div class="flex items-center justify-between">
                  <span class="text-[10px] text-t-muted font-medium tracking-wide">运行中</span>
                  <span class="w-5 h-5 rounded-md flex items-center justify-center"
                    :class="(adminStats.running_tasks||0)>0 ? 'bg-cyan-500/15' : 'bg-s-hover'">
                    <span v-if="(adminStats.running_tasks||0)>0" class="w-2 h-2 rounded-full bg-accent overview-pulse"></span>
                    <span v-else class="w-2 h-2 rounded-full bg-t-faint/40"></span>
                  </span>
                </div>
                <div class="text-2xl font-bold tabular-nums leading-none"
                  :class="(adminStats.running_tasks||0)>0 ? 'text-accent' : 'text-t-faint'">
                  {{ adminStats.running_tasks || 0 }}
                </div>
                <div class="text-[10px] text-t-faint">排队 <span class="text-warn font-medium">{{ adminStats.queued_tasks || 0 }}</span> 个</div>
              </div>

              <!-- 系统积分 -->
              <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
                <div class="flex items-center justify-between">
                  <span class="text-[10px] text-t-muted font-medium tracking-wide">积分余量</span>
                  <span class="w-5 h-5 rounded-md bg-emerald-500/15 flex items-center justify-center">
                    <svg class="w-3 h-3 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
                  </span>
                </div>
                <div class="text-2xl font-bold tabular-nums text-emerald-400 leading-none">{{ adminStats.total_credits_in_system || 0 }}</div>
                <div class="text-[10px] text-t-faint">今日购入 <span class="text-emerald-400 font-medium">{{ adminStats.today_purchase_credits || 0 }}</span></div>
              </div>
            </div>

            <!-- 第二行：平台成功率对比 + 7日新增趋势 -->
            <div class="grid grid-cols-5 gap-2.5 mobile-grid-1">

              <!-- 平台成功率对比（占 3 列） -->
              <div class="col-span-3 glass-light rounded-xl p-3">
                <div class="text-[10px] text-t-muted uppercase tracking-wider mb-3 flex items-center gap-1.5">
                  <span class="w-1.5 h-1.5 rounded-full bg-blue-500"></span>
                  平台分析
                </div>
                <div class="space-y-3">
                  <div v-for="p in overviewPlatforms" :key="p.key" class="space-y-1">
                    <!-- 平台头部：名称 + 今日注册/总注册 + 成功率 -->
                    <div class="flex items-center justify-between">
                      <div class="flex items-center gap-2">
                        <span class="w-1.5 h-1.5 rounded-full flex-none" :class="p.dot"></span>
                        <span class="text-[11px] font-semibold" :class="p.text">{{ p.label }}</span>
                        <span class="text-[10px] text-t-faint">总 {{ (adminStats.platforms || {})[p.key] || 0 }}</span>
                        <span v-if="(adminStats.today_platforms || {})[p.key]" class="text-[10px] px-1.5 py-0 rounded" :class="p.badge">今 +{{ (adminStats.today_platforms || {})[p.key] }}</span>
                      </div>
                      <div class="flex items-center gap-3 text-[10px]">
                        <!-- 平均耗时 -->
                        <span v-if="platformDuration(p.key)" class="text-t-faint">
                          均耗 <span class="text-t-secondary font-medium">{{ platformDuration(p.key) }}</span>
                        </span>
                        <!-- 今日成功率 -->
                        <div class="flex items-center gap-1">
                          <span class="text-t-faint">今日</span>
                          <span class="font-mono font-bold"
                            :class="todayPlatformRate(p.key)>=80?'text-ok':todayPlatformRate(p.key)>=50?'text-warn':todayPlatformRate(p.key)>0?'text-err':'text-t-faint'">
                            {{ todayPlatformRate(p.key) > 0 ? todayPlatformRate(p.key).toFixed(1)+'%' : '-' }}
                          </span>
                        </div>
                        <!-- 历史成功率 -->
                        <div class="flex items-center gap-1">
                          <span class="text-t-faint">历史</span>
                          <span class="font-mono font-bold"
                            :class="totalPlatformRate(p.key)>=80?'text-ok':totalPlatformRate(p.key)>=50?'text-warn':totalPlatformRate(p.key)>0?'text-err':'text-t-faint'">
                            {{ totalPlatformRate(p.key) > 0 ? totalPlatformRate(p.key).toFixed(1)+'%' : '-' }}
                          </span>
                        </div>
                      </div>
                    </div>
                    <!-- 双进度条：历史（底层浅色）+ 今日（叠加亮色） -->
                    <div class="relative h-1.5 rounded-full bg-s-hover overflow-hidden">
                      <!-- 历史成功率底层 -->
                      <div class="absolute inset-y-0 left-0 rounded-full opacity-30 transition-all duration-700" :class="p.bar"
                        :style="`width:${totalPlatformRate(p.key)}%`"></div>
                      <!-- 今日成功率叠加 -->
                      <div class="absolute inset-y-0 left-0 rounded-full transition-all duration-700" :class="p.bar"
                        :style="`width:${todayPlatformRate(p.key)}%`"></div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 7日新增用户趋势（占 2 列） -->
              <div class="col-span-2 glass-light rounded-xl p-3">
                <div class="text-[10px] text-t-muted uppercase tracking-wider mb-3 flex items-center justify-between">
                  <div class="flex items-center gap-1.5">
                    <span class="w-1.5 h-1.5 rounded-full bg-purple-500"></span>
                    7日新增用户
                  </div>
                  <span class="text-t-secondary font-mono font-bold">{{ newUsers7dTotal }}</span>
                </div>
                <!-- 迷你柱状图 -->
                <div v-if="adminStats.new_users_7d && adminStats.new_users_7d.length" class="flex items-end gap-1 h-16 mt-1">
                  <div v-for="(d, i) in adminStats.new_users_7d" :key="i"
                    class="flex-1 flex flex-col items-center justify-end gap-0.5 group">
                    <div class="w-full rounded-t-sm transition-all duration-500 overview-bar-hover"
                      :class="i === adminStats.new_users_7d.length - 1 ? 'bg-purple-500' : 'bg-purple-500/40'"
                      :style="`height:${newUsers7dMax > 0 ? Math.max(4, Math.round((d.count / newUsers7dMax) * 52)) : 4}px`"
                      :title="`${d.day}: ${d.count}`">
                    </div>
                    <span class="text-[8px] text-t-faint group-hover:text-t-muted transition" style="line-height:1">{{ d.day }}</span>
                  </div>
                </div>
                <div v-else class="h-16 flex items-center justify-center text-[10px] text-t-faint">暂无数据</div>
              </div>
            </div>

            <!-- 第三行：今日数据 + 积分流水 + 最近交易 -->
            <div class="grid grid-cols-3 gap-2.5 mobile-grid-1">

              <!-- 今日快照 -->
              <div class="glass-light rounded-xl p-3">
                <div class="text-[10px] text-t-muted uppercase tracking-wider mb-2.5 flex items-center gap-1.5">
                  <span class="w-1.5 h-1.5 rounded-full bg-cyan-500"></span>
                  今日快照
                  <span class="text-t-faint text-[9px] ml-auto">6:00 重置</span>
                </div>
                <div class="grid grid-cols-3 gap-2 mb-2.5">
                  <div class="bg-s-inset rounded-lg p-2 text-center">
                    <div class="text-[9px] text-t-faint mb-0.5">任务</div>
                    <div class="text-base font-bold text-info tabular-nums">{{ adminStats.today_tasks || 0 }}</div>
                  </div>
                  <div class="bg-s-inset rounded-lg p-2 text-center">
                    <div class="text-[9px] text-t-faint mb-0.5">注册</div>
                    <div class="text-base font-bold text-accent tabular-nums">{{ adminStats.today_results || 0 }}</div>
                  </div>
                  <div class="bg-s-inset rounded-lg p-2 text-center">
                    <div class="text-[9px] text-t-faint mb-0.5">购入</div>
                    <div class="text-base font-bold text-emerald-400 tabular-nums">{{ adminStats.today_purchase_count || 0 }}<span class="text-[8px] text-t-faint font-normal">笔</span></div>
                  </div>
                </div>
                <!-- 今日平台分布 -->
                <div class="space-y-1.5">
                  <template v-for="p in overviewPlatforms" :key="p.key">
                    <div v-if="(adminStats.today_platforms || {})[p.key] || (adminStats.platforms || {})[p.key]"
                      class="flex items-center gap-2">
                      <span class="w-1.5 h-1.5 rounded-full flex-none" :class="p.dot"></span>
                      <span class="text-[10px] font-medium flex-none w-12" :class="p.text">{{ p.label }}</span>
                      <div class="flex-1 h-1 rounded-full bg-s-hover overflow-hidden">
                        <div class="h-full rounded-full transition-all duration-500" :class="p.bar"
                          :style="`width:${adminStats.total_results ? Math.min(100, ((adminStats.platforms||{})[p.key]||0)/adminStats.total_results*100) : 0}%`"></div>
                      </div>
                      <span class="text-[10px] font-mono text-t-secondary flex-none">{{ (adminStats.platforms || {})[p.key] || 0 }}</span>
                      <span class="text-[10px] text-t-faint flex-none w-8 text-right">
                        {{ adminStats.total_results ? (((adminStats.platforms||{})[p.key]||0)/adminStats.total_results*100).toFixed(0)+'%' : '' }}
                      </span>
                    </div>
                  </template>
                </div>
              </div>

              <!-- 积分流水 -->
              <div class="glass-light rounded-xl p-3">
                <div class="text-[10px] text-t-muted uppercase tracking-wider mb-2.5 flex items-center gap-1.5">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
                  积分流水
                </div>
                <div class="space-y-2">
                  <div class="flex items-center justify-between py-1.5 border-b border-b-panel">
                    <div>
                      <div class="text-[10px] text-t-faint">购买收入</div>
                      <div class="text-base font-bold text-emerald-400 tabular-nums">{{ adminStats.purchase_credits || 0 }}</div>
                    </div>
                    <div class="text-right">
                      <div class="text-[10px] text-t-faint">{{ adminStats.purchase_count || 0 }} 笔</div>
                      <div class="text-[10px] text-emerald-400">今日 +{{ adminStats.today_purchase_credits || 0 }}</div>
                    </div>
                  </div>
                  <div class="flex items-center justify-between py-1.5 border-b border-b-panel">
                    <div>
                      <div class="text-[10px] text-t-faint">兑换码</div>
                      <div class="text-base font-bold text-warn tabular-nums">{{ adminStats.redeem_credits || 0 }}</div>
                    </div>
                    <div class="text-right">
                      <div class="text-[10px] text-t-faint">{{ adminStats.redeem_count || 0 }} 笔</div>
                    </div>
                  </div>
                  <div class="flex items-center justify-between py-1">
                    <div>
                      <div class="text-[10px] text-t-faint">总消费</div>
                      <div class="text-sm font-bold text-err tabular-nums">{{ adminStats.total_consumed || 0 }}</div>
                    </div>
                    <div class="text-right">
                      <div class="text-[10px] text-t-faint">退还</div>
                      <div class="text-sm font-bold text-accent tabular-nums">{{ adminStats.total_refunded || 0 }}</div>
                    </div>
                  </div>
                  <!-- 净流入计算 -->
                  <div class="mt-1 pt-1.5 border-t border-b-panel flex items-center justify-between">
                    <span class="text-[10px] text-t-faint">净流入积分</span>
                    <span class="text-[11px] font-bold tabular-nums"
                      :class="((adminStats.purchase_credits||0)+(adminStats.redeem_credits||0)-(adminStats.total_consumed||0)+(adminStats.total_refunded||0)) >= 0 ? 'text-ok' : 'text-err'">
                      {{ (adminStats.purchase_credits||0) + (adminStats.redeem_credits||0) - (adminStats.total_consumed||0) + (adminStats.total_refunded||0) }}
                    </span>
                  </div>
                </div>
              </div>

              <!-- 最近交易动态 -->
              <div class="glass-light rounded-xl p-3">
                <div class="text-[10px] text-t-muted uppercase tracking-wider mb-2 flex items-center gap-1.5">
                  <span class="w-1.5 h-1.5 rounded-full bg-blue-500"></span>
                  最近交易
                </div>
                <div class="space-y-1 max-h-44 overflow-y-auto scroll-thin">
                  <div v-if="!adminStats.recent_txs || adminStats.recent_txs.length === 0"
                    class="text-t-faint text-center py-6 text-[10px]">暂无交易</div>
                  <div v-for="tx in (adminStats.recent_txs || [])" :key="tx.id"
                    class="flex items-center gap-1.5 text-[10px] px-1.5 py-1 rounded hover:bg-s-hover transition">
                    <span class="flex-none w-1.5 h-1.5 rounded-full"
                      :class="tx.type === 'purchase' ? 'bg-emerald-500' : tx.type === 'redeem' ? 'bg-amber-500' : 'bg-blue-500'"></span>
                    <span class="text-t-secondary flex-1 min-w-0 truncate font-medium" :title="`#${tx.user_id} ${tx.name || tx.username}`">{{ tx.name || tx.username }}</span>
                    <span class="flex-none px-1 py-0 rounded text-[9px] font-semibold"
                      :class="tx.type === 'purchase' ? 'bg-emerald-500/15 text-emerald-400' : tx.type === 'redeem' ? 'bg-amber-500/15 text-amber-400' : 'bg-blue-500/15 text-blue-400'">
                      {{ tx.type === 'purchase' ? '买' : tx.type === 'redeem' ? '换' : '充' }}
                    </span>
                    <span class="text-ok font-mono font-bold flex-none">+{{ tx.amount }}</span>
                    <span class="text-t-faint flex-none ml-auto">{{ tx.created_at }}</span>
                  </div>
                </div>
              </div>
            </div>

          </div>

          <!-- 用户 Tab -->
          <AdminUsersTab />

          <!-- 系统设置 / 数据清理 / 实时状态 Tabs -->
          <AdminSystemTab />

          <!-- 运营 Tab（兑换码） -->
          <AdminOpsTab />

          <!-- 通知公告 Tab -->
          <AdminNotifyTab />

          <!-- HF 空间管理 Tab -->
          <AdminHFSpaceTab />

          <!-- 代理池管理 Tab -->
          <AdminProxyPoolTab />

          <!-- 卡池管理 Tab -->
          <AdminCardPoolTab />

        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useDashboard } from '../../composables/useDashboard'
import { useAdmin } from '../../composables/useAdmin'
import AdminUsersTab from './AdminUsersTab.vue'
import AdminSystemTab from './AdminSystemTab.vue'
import AdminOpsTab from './AdminOpsTab.vue'
import AdminNotifyTab from './AdminNotifyTab.vue'
import AdminHFSpaceTab from './AdminHFSpaceTab.vue'
import AdminProxyPoolTab from './AdminProxyPoolTab.vue'
import AdminCardPoolTab from './AdminCardPoolTab.vue'

const { user } = useDashboard()
const {
  showAdmin, adminTab, adminStats,
  loadAdminUsers, loadAdminSettings,
  loadAdminCodes, loadAdminNotifications, loadAdminAnnouncements,
  loadDataStats, startRealtimePolling,
} = useAdmin()

/** 运营 tab 加载：兑换码 */
function loadOpsTab() {
  loadAdminCodes()
}

/** 通知公告 tab 加载 */
function loadNotifyTab() {
  loadAdminNotifications()
  loadAdminAnnouncements()
}

// HF Space
import { useHFSpace } from '../../composables/useHFSpace'
const { loadAll: loadHFSpace } = useHFSpace()

// 代理池
import { useProxyPool } from '../../composables/useProxyPool'
const { loadAll: loadProxyPoolTab } = useProxyPool()

// 卡池
import { useCardPool } from '../../composables/useCardPool'
const { loadAll: loadCardPoolTab } = useCardPool()

// ── 概览 Tab 辅助数据 ──

/** 平台配置（颜色/标签/样式） */
const overviewPlatforms = [
  { key: 'grok',   label: 'Grok',   dot: 'bg-blue-500',  bar: 'bg-blue-500',  text: 'text-info', badge: 'bg-blue-500/15 text-info' },
  { key: 'openai', label: 'OpenAI', dot: 'bg-green-500', bar: 'bg-green-500', text: 'text-ok',   badge: 'bg-green-500/15 text-ok' },
  { key: 'openai_team', label: 'Team', dot: 'bg-emerald-500', bar: 'bg-emerald-500', text: 'text-emerald-400', badge: 'bg-emerald-500/15 text-emerald-400' },
  { key: 'kiro',   label: 'Kiro',   dot: 'bg-amber-500', bar: 'bg-amber-500', text: 'text-warn', badge: 'bg-amber-500/15 text-warn' },
  { key: 'gemini', label: 'Gemini', dot: 'bg-purple-500', bar: 'bg-purple-500', text: 'text-purple-400', badge: 'bg-purple-500/15 text-purple-400' },
]

/** 获取某平台的今日成功率（%） */
function todayPlatformRate(key: string): number {
  const rates = adminStats.value?.today_platform_rates
  if (!rates || !rates[key]) return 0
  return rates[key].success_rate ?? 0
}

/** 获取某平台的历史成功率（%） */
function totalPlatformRate(key: string): number {
  const rates = adminStats.value?.platform_rates
  if (!rates || !rates[key]) return 0
  return rates[key].success_rate ?? 0
}

/** 获取某平台平均耗时的可读字符串 */
function platformDuration(key: string): string {
  const dur = adminStats.value?.platform_avg_duration
  if (!dur || !dur[key] || dur[key].avg_seconds <= 0) return ''
  const sec = Math.round(dur[key].avg_seconds)
  if (sec < 60) return `${sec}s`
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return s > 0 ? `${m}m${s}s` : `${m}m`
}

/** 7日新增用户趋势数据 */
const newUsers7dMax = computed<number>(() => {
  const data = adminStats.value?.new_users_7d
  if (!data || !data.length) return 1
  return Math.max(1, ...data.map((d: any) => d.count ?? 0))
})

/** 7日新增总量 */
const newUsers7dTotal = computed<number>(() => {
  const data = adminStats.value?.new_users_7d
  if (!data || !data.length) return 0
  return data.reduce((sum: number, d: any) => sum + (d.count ?? 0), 0)
})
</script>
