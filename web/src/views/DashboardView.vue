<template>
  <!-- 首屏加载遮罩 -->
  <Transition name="loading-fade">
    <div v-if="pageLoading" class="fixed inset-0 z-[100] flex flex-col items-center justify-center"
      style="background:rgba(8,12,22,0.1);-webkit-backdrop-filter:blur(5px);backdrop-filter:blur(5px)">
      <img src="/loading.webp" alt="loading" class="w-64 h-64">
    </div>
  </Transition>

  <!-- 真实内容 -->
  <div class="text-t-primary flex flex-col h-screen px-4 py-2 gap-1.5 mobile-stack" @click="showNotifPanel = false">

    <!-- 顶栏 -->
    <TopNavBar />

    <!-- 主区域：三栏 -->
    <div class="flex-1 flex min-h-0 gap-2 mobile-stack mobile-main-area">
      <LeftSidebar class="desktop-only" />
      <ContentArea />
      <LogPanel />
    </div>

    <!-- 手机：浮动按钮 + 底部抽屉 -->
    <button class="mobile-only fixed bottom-4 right-4 z-30 w-12 h-12 rounded-full bg-gradient-to-r from-blue-600 to-cyan-500 text-white shadow-lg shadow-blue-500/30 flex items-center justify-center text-lg font-bold"
      @click="showMobileDrawer = true">
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg>
    </button>
    <MobileDrawer v-model:show="showMobileDrawer">
      <LeftSidebar :mobile="true" />
    </MobileDrawer>

    <!-- 管理后台弹窗 -->
    <AdminModal />

    <!-- Toast 通知 -->
    <ToastOverlay />

  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useDashboard } from '../composables/useDashboard'
import { useTaskEngine } from '../composables/useTaskEngine'
import { useAdmin } from '../composables/useAdmin'
import { useTooltip } from '../composables/useTooltip'

import TopNavBar from '../components/dashboard/TopNavBar.vue'
import LeftSidebar from '../components/dashboard/LeftSidebar.vue'
import ContentArea from '../components/dashboard/ContentArea.vue'
import LogPanel from '../components/dashboard/LogPanel.vue'
import AdminModal from '../components/dashboard/AdminModal.vue'
import ToastOverlay from '../components/dashboard/ToastOverlay.vue'
import MobileDrawer from '../components/dashboard/MobileDrawer.vue'

const { initLoad, loadAllArchivedCounts, startGlobalStatsPolling, stopGlobalStatsPolling, startCompletionPolling, stopCompletionPolling } = useDashboard()
const auth = useAuthStore()
const taskEngine = useTaskEngine()
const admin = useAdmin()
const tooltip = useTooltip()

const { showNotifPanel } = admin
const showMobileDrawer = ref(false)
const pageLoading = ref(true)

/**
 * visibility 保护：tab 切到后台时暂停所有轮询，切回前台时恢复。
 * 避免用户不看页面时仍持续消耗带宽和服务器资源。
 */
function handleVisibilityChange() {
  if (document.hidden) {
    // tab 不可见 — 暂停全部轮询
    stopGlobalStatsPolling()
    stopCompletionPolling()
    admin.stopNotifPolling()
    admin.stopRealtimePolling()
  } else {
    // tab 重新可见 — 恢复常驻轮询（realtime 由 adminTab watcher 自行管理，不在此恢复）
    startGlobalStatsPolling()
    startCompletionPolling()
    admin.startNotifPolling()
  }
}

onMounted(async () => {
  tooltip.setup()
  if ('Notification' in window && Notification.permission === 'default') {
    Notification.requestPermission()
  }
  try {
    const data = await initLoad()
    taskEngine.resumeFromInit(data.current_task)
    taskEngine.setupWatchers()
    admin.setupAdminWatchers()
    admin.startNotifPolling()
    loadAllArchivedCounts()
    startGlobalStatsPolling()
    startCompletionPolling()
  } catch {
    auth.logout()
    window.location.href = '/'
    return
  }
  pageLoading.value = false
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onUnmounted(() => {
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  tooltip.destroy()
  taskEngine.cleanup()
  admin.cleanup()
  stopGlobalStatsPolling()
  stopCompletionPolling()
})
</script>
