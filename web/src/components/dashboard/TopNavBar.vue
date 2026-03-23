<template>
  <nav class="glass flex-none px-4 py-2 flex items-center justify-between z-50 rounded-xl mobile-compact-nav">
    <!-- 左：平台标题 + 我的任务状态 -->
    <div class="flex items-center gap-4 flex-none">
      <!-- 平台标题：DM Sans 字重加重，tracking-tight 凝练感 -->
      <div class="flex items-center gap-2">
        <div class="text-base font-black tracking-tight bg-gradient-to-r bg-clip-text text-transparent leading-none select-none"
          :class="platformTitleClass">{{ platformLabel }}</div>
        <!-- 运行状态小标签，紧贴平台名 -->
        <span v-if="isRunning || isQueued"
          class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold leading-none"
          :class="isRunning ? 'bg-green-500/15 text-ok border border-green-500/20' : 'bg-amber-500/15 text-warn border border-amber-500/20'">
          <span class="relative flex h-1.5 w-1.5">
            <span class="pulse-ring absolute inline-flex h-full w-full rounded-full opacity-75"
              :class="isRunning ? 'bg-green-400' : 'bg-amber-400'"></span>
            <span class="relative inline-flex rounded-full h-1.5 w-1.5"
              :class="isRunning ? 'bg-green-500' : 'bg-amber-500'"></span>
          </span>
          {{ isRunning ? '运行中' : '排队中' }}
        </span>
      </div>
      <!-- 个人历史统计：只桌面端显示 -->
      <div class="flex items-center gap-3 desktop-only">
        <div class="h-3.5 w-px bg-b-panel"></div>
        <div class="flex items-center gap-3 text-xs">
          <span class="tip tip-bottom text-t-faint" :data-tip="`${platformLabel} 历史注册成功总数`">
            成功 <span class="text-ok font-mono font-semibold ml-0.5 tabular-nums">{{ platformUserStats.success }}</span>
          </span>
          <span class="tip tip-bottom text-t-faint" :data-tip="`${platformLabel} 历史注册失败总数`">
            失败 <span class="text-err font-mono font-semibold ml-0.5 tabular-nums">{{ platformUserStats.fail }}</span>
          </span>
        </div>
      </div>
    </div>

    <!-- 中：全站统计 -->
    <div class="flex items-center gap-2 desktop-only">
      <!-- 分组 1：活跃状态 -->
      <div class="flex items-center gap-2.5 glass-light rounded-lg px-3 py-1.5">
        <div class="flex items-center gap-1.5 tip tip-bottom" data-tip="全站正在运行的任务数">
          <span class="relative flex h-1.5 w-1.5" v-if="globalStats.running_tasks > 0">
            <span class="pulse-ring absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-cyan-500"></span>
          </span>
          <span v-else class="h-1.5 w-1.5 rounded-full bg-b-panel"></span>
          <span class="text-[10px] text-t-faint">运行</span>
          <span class="text-[11px] font-bold font-mono tabular-nums" :class="globalStats.running_tasks > 0 ? 'text-accent' : 'text-t-faint'">{{ globalStats.running_tasks }}</span>
        </div>
        <div class="w-px h-2.5 bg-b-panel"></div>
        <div class="flex items-center gap-1.5 tip tip-bottom" data-tip="全站有任务运行中的用户数">
          <span class="text-[10px] text-t-faint">在线</span>
          <span class="text-[11px] font-bold font-mono tabular-nums text-info">{{ globalStats.active_users }}</span>
        </div>
        <template v-if="globalStats.queued_tasks > 0">
          <div class="w-px h-2.5 bg-b-panel"></div>
          <div class="flex items-center gap-1.5 tip tip-bottom" data-tip="全站排队等待的任务数">
            <span class="relative flex h-1.5 w-1.5">
              <span class="pulse-ring absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
              <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-amber-500"></span>
            </span>
            <span class="text-[10px] text-t-faint">排队</span>
            <span class="text-[11px] font-bold font-mono tabular-nums text-warn">{{ globalStats.queued_tasks }}</span>
          </div>
        </template>
      </div>

      <!-- 分组 2：今日数据 -->
      <div class="flex items-center gap-2.5 glass-light rounded-lg px-3 py-1.5">
        <div class="flex items-center gap-1.5 tip tip-bottom" data-tip="今日全站注册成功总数（凌晨6点重置）">
          <span class="text-[10px] text-t-faint">今日</span>
          <span class="text-[11px] font-bold font-mono tabular-nums text-warn">{{ globalStats.today_total }}</span>
        </div>
        <div class="w-px h-2.5 bg-b-panel"></div>
        <!-- 各平台今日数量：彩色小点+数字紧凑排列 -->
        <div class="flex items-center gap-2">
          <span class="flex items-center gap-1 tip tip-bottom" data-tip="今日 Grok 注册成功数">
            <span class="w-1.5 h-1.5 rounded-full bg-blue-500 flex-none"></span>
            <span class="text-[10px] font-mono tabular-nums text-info">{{ globalStats.today_platforms?.grok || 0 }}</span>
          </span>
          <span class="flex items-center gap-1 tip tip-bottom" data-tip="今日 OpenAI 注册成功数">
            <span class="w-1.5 h-1.5 rounded-full bg-green-500 flex-none"></span>
            <span class="text-[10px] font-mono tabular-nums text-ok">{{ globalStats.today_platforms?.openai || 0 }}</span>
          </span>
          <span class="flex items-center gap-1 tip tip-bottom" data-tip="今日 Kiro 注册成功数">
            <span class="w-1.5 h-1.5 rounded-full bg-amber-500 flex-none"></span>
            <span class="text-[10px] font-mono tabular-nums text-warn">{{ globalStats.today_platforms?.kiro || 0 }}</span>
          </span>
          <span class="flex items-center gap-1 tip tip-bottom" data-tip="今日 Gemini 注册成功数">
            <span class="w-1.5 h-1.5 rounded-full bg-purple-500 flex-none"></span>
            <span class="text-[10px] font-mono tabular-nums text-purple-400">{{ globalStats.today_platforms?.gemini || 0 }}</span>
          </span>
        </div>
        <div class="w-px h-2.5 bg-b-panel"></div>
        <div class="flex items-center gap-1.5 tip tip-bottom" data-tip="今日全站注册成功率">
          <span class="text-[10px] text-t-faint">成功率</span>
          <span class="text-[11px] font-bold font-mono tabular-nums"
            :class="globalStats.success_rate >= 80 ? 'text-ok' : globalStats.success_rate >= 50 ? 'text-warn' : 'text-err'">
            {{ (globalStats.success_rate || 0).toFixed(0) }}%
          </span>
        </div>
        <div class="w-px h-2.5 bg-b-panel"></div>
        <div class="flex items-center gap-1.5 tip tip-bottom" data-tip="平台累计注册成功总数">
          <span class="text-[10px] text-t-faint">总计</span>
          <span class="text-[11px] font-bold font-mono tabular-nums text-t-secondary">{{ globalStats.total_results }}</span>
        </div>
      </div>
    </div>

    <!-- 右：通知 + 管理 + 退出 -->
    <div class="flex items-center gap-2.5 flex-none">
      <!-- 通知铃铛 -->
      <div class="relative" @click.stop>
        <button @click="showNotifPanel = !showNotifPanel" data-notif-toggle
          class="relative text-t-secondary hover:text-white transition p-1.5 rounded-lg hover:bg-s-hover">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/></svg>
          <span v-if="unreadCount > 0"
            class="absolute -top-0.5 -right-0.5 flex items-center justify-center min-w-[14px] h-[14px] rounded-full bg-red-500 text-[8px] font-bold text-white px-0.5">
            {{ unreadCount > 99 ? '99+' : unreadCount }}
          </span>
        </button>
        <!-- 通知下拉面板 -->
        <Transition name="notif-panel">
          <div v-if="showNotifPanel"
            class="absolute right-0 top-full mt-2 w-80 max-h-[420px] rounded-xl shadow-2xl overflow-hidden z-50 flex flex-col backdrop-blur-xl mobile-notif-panel"
            style="background:var(--bg-admin);border:1px solid var(--border-glass);box-shadow:0 8px 32px rgba(0,0,0,0.5)">
            <!-- 头部 -->
            <div class="flex items-center justify-between px-4 py-2.5 border-b border-b-panel">
              <div class="flex items-center gap-2">
                <svg class="w-3.5 h-3.5 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/></svg>
                <span class="text-xs font-semibold text-t-primary">通知</span>
                <span v-if="unreadCount > 0" class="text-[10px] font-mono font-bold text-accent bg-cyan-900/30 px-1.5 py-0.5 rounded-full leading-none">{{ unreadCount }}</span>
              </div>
              <button v-if="unreadCount > 0" @click="markAllNotifsRead"
                class="text-[10px] text-accent hover:text-cyan-300 hover:bg-cyan-900/20 px-2 py-1 rounded-md transition font-medium">全部已读</button>
            </div>
            <!-- 列表 -->
            <div class="flex-1 overflow-y-auto scroll-thin">
              <div v-if="userNotifications.length === 0" class="flex flex-col items-center justify-center py-12 gap-2">
                <svg class="w-8 h-8 text-t-faint" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"/></svg>
                <span class="text-[10px] text-t-faint">暂无通知</span>
              </div>
              <div v-for="(n, idx) in userNotifications" :key="n.id"
                class="px-4 py-2.5 transition cursor-pointer group"
                :class="[
                  !n.is_read ? 'bg-cyan-500/[0.04] hover:bg-cyan-500/[0.08]' : 'hover:bg-s-hover',
                  idx < userNotifications.length - 1 ? 'border-b border-b-panel' : ''
                ]"
                @click="!n.is_read && markNotifRead(n.id)">
                <div class="flex items-start gap-2.5">
                  <!-- 未读指示器 -->
                  <div class="flex-none mt-1.5 w-2 flex justify-center">
                    <span v-if="!n.is_read" class="w-1.5 h-1.5 rounded-full bg-cyan-400 shadow-sm shadow-cyan-400/50"></span>
                  </div>
                  <!-- 内容 -->
                  <div class="flex-1 min-w-0 overflow-hidden">
                    <div class="flex items-center gap-1.5 min-w-0">
                      <span class="text-[11px] font-semibold truncate" :class="!n.is_read ? 'text-t-primary' : 'text-t-secondary'">{{ n.title }}</span>
                      <span v-if="n.user_id === 0" class="text-[8px] px-1.5 py-0.5 rounded-full bg-cyan-900/30 text-cyan-400/80 flex-none font-medium leading-none">广播</span>
                    </div>
                    <div class="text-[10px] mt-1 leading-relaxed break-all" :class="!n.is_read ? 'text-t-secondary' : 'text-t-faint'">{{ n.content }}</div>
                    <div class="text-[9px] text-t-faint mt-1 font-mono">{{ formatNotifTime(n.created_at) }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </Transition>
      </div>
      <button v-if="user.is_admin" @click="openAdmin"
        class="text-xs px-3 py-1.5 rounded-lg transition font-medium"
        :class="showAdmin ? 'bg-blue-600 text-white' : 'glass-light text-t-secondary hover:text-white'">
        管理后台
      </button>
      <button @click="logout"
        class="text-xs px-2.5 py-1.5 rounded-lg text-t-muted hover:text-red-400 hover:bg-red-900/20 transition font-medium">
        退出
      </button>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useDashboard } from '../../composables/useDashboard'
import { useAdmin } from '../../composables/useAdmin'

const { user, isRunning, isQueued, platformLabel, platformTitleClass, platformUserStats, globalStats, logout } = useDashboard()
const { showAdmin, openAdmin, showNotifPanel, unreadCount, userNotifications, markNotifRead, markAllNotifsRead, formatNotifTime } = useAdmin()

// 点击外部关闭通知面板
function handleOutsideClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (showNotifPanel.value && !target.closest('.mobile-notif-panel') && !target.closest('[data-notif-toggle]')) {
    showNotifPanel.value = false
  }
}
onMounted(() => document.addEventListener('click', handleOutsideClick))
onUnmounted(() => document.removeEventListener('click', handleOutsideClick))
</script>
