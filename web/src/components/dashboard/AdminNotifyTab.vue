<template>
  <div v-show="adminTab === 'notify'" class="space-y-6">

    <!-- ═══ 通知板块（可折叠） ═══ -->
    <div class="glass-light rounded-xl overflow-hidden">
      <button @click="notifCollapsed = !notifCollapsed"
        class="w-full px-4 py-3 flex items-center justify-between border-b border-b-panel hover:bg-s-hover transition">
        <div class="flex items-center gap-2">
          <svg class="w-3.5 h-3.5 text-t-muted transition-transform" :class="notifCollapsed ? '-rotate-90' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
          </svg>
          <span class="text-xs font-bold text-cyan-400 uppercase tracking-wider">通知管理</span>
          <span class="text-[10px] font-mono text-t-faint">{{ adminNotifications.length }}</span>
        </div>
        <span class="text-[10px] text-t-faint">发送给指定用户或全站广播</span>
      </button>
      <div v-show="!notifCollapsed" class="p-4 space-y-4">
        <div class="grid grid-cols-3 gap-3">
          <div>
            <label class="text-[10px] text-t-muted mb-1 block">用户 ID（0=广播）</label>
            <input v-model.number="notifyForm.user_id" type="number" min="0" placeholder="0"
              class="w-full bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-cyan-500 focus:outline-none">
          </div>
          <div>
            <label class="text-[10px] text-t-muted mb-1 block">标题</label>
            <input v-model="notifyForm.title" type="text" placeholder="通知标题"
              class="w-full bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-cyan-500 focus:outline-none">
          </div>
          <div>
            <label class="text-[10px] text-t-muted mb-1 block">内容</label>
            <input v-model="notifyForm.content" type="text" placeholder="通知内容"
              class="w-full bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-cyan-500 focus:outline-none">
          </div>
        </div>
        <button @click="handleSendNotification" :disabled="!notifyForm.title.trim() || !notifyForm.content.trim()"
          class="w-full py-2 rounded-lg text-xs font-bold bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 disabled:from-gray-700 disabled:to-gray-700 disabled:text-gray-500 text-white transition">
          发送通知
        </button>
        <!-- 通知记录 -->
        <div class="border-t border-b-panel pt-3">
          <div class="flex items-center justify-between mb-2">
            <span class="text-[10px] text-t-muted uppercase tracking-wider">已发送通知</span>
            <button @click="loadAdminNotifications" class="text-[10px] text-t-muted hover:text-white transition px-2 py-1 rounded hover:bg-s-hover">刷新</button>
          </div>
          <div v-if="adminNotifsLoading" class="text-t-muted text-center py-4 text-xs">加载中...</div>
          <div v-else-if="adminNotifications.length === 0" class="text-t-faint text-center py-4 text-[10px]">暂无通知记录</div>
          <div v-else class="max-h-[280px] overflow-y-auto scroll-thin divide-y divide-b-panel rounded-lg bg-s-inset/30">
            <div v-for="n in adminNotifications" :key="n.id" class="px-4 py-2.5 hover:bg-s-hover transition group">
              <div class="flex items-start gap-3">
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="text-[11px] font-bold text-t-primary truncate">{{ n.title }}</span>
                    <span v-if="n.user_id === 0" class="text-[8px] px-1.5 py-0.5 rounded-full bg-cyan-900/30 text-accent font-medium leading-none flex-none">广播</span>
                    <span v-else class="text-[8px] px-1.5 py-0.5 rounded-full bg-blue-900/30 text-info font-medium leading-none flex-none">用户#{{ n.user_id }}</span>
                  </div>
                  <div class="text-[10px] text-t-muted mt-0.5 line-clamp-1">{{ n.content }}</div>
                  <div class="text-[9px] text-t-faint mt-0.5 flex items-center gap-2">
                    <span>{{ n.created_at }}</span>
                    <span v-if="n.creator_name" class="text-t-muted">by {{ n.creator_name }}</span>
                  </div>
                </div>
                <button @click.stop="deleteAdminNotification(n.id)"
                  class="flex-none opacity-0 group-hover:opacity-100 text-t-muted hover:text-red-400 p-1 rounded hover:bg-red-900/20 transition">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ═══ 公告板块（可折叠） ═══ -->
    <div class="glass-light rounded-xl overflow-hidden">
      <button @click="annCollapsed = !annCollapsed"
        class="w-full px-4 py-3 flex items-center justify-between border-b border-b-panel hover:bg-s-hover transition">
        <div class="flex items-center gap-2">
          <svg class="w-3.5 h-3.5 text-t-muted transition-transform" :class="annCollapsed ? '-rotate-90' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
          </svg>
          <span class="text-xs font-bold text-amber-400 uppercase tracking-wider">公告管理</span>
          <span class="text-[10px] font-mono text-t-faint">{{ adminAnnouncements.length }}</span>
        </div>
        <span class="text-[10px] text-t-faint">全站公告，所有用户可见</span>
      </button>
      <div v-show="!annCollapsed" class="p-4 space-y-4">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="text-[10px] text-t-muted mb-1 block">标题</label>
            <input v-model="annForm.title" type="text" placeholder="公告标题"
              class="w-full bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-amber-500 focus:outline-none">
          </div>
          <div>
            <label class="text-[10px] text-t-muted mb-1 block">内容</label>
            <input v-model="annForm.content" type="text" placeholder="公告内容"
              class="w-full bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-amber-500 focus:outline-none">
          </div>
        </div>
        <button @click="handleCreateAnnouncement" :disabled="!annForm.title.trim() || !annForm.content.trim()"
          class="w-full py-2 rounded-lg text-xs font-bold bg-gradient-to-r from-amber-600 to-orange-600 hover:from-amber-500 hover:to-orange-500 disabled:from-gray-700 disabled:to-gray-700 disabled:text-gray-500 text-white transition">
          发布公告
        </button>
        <!-- 公告列表 -->
        <div class="border-t border-b-panel pt-3">
          <div class="flex items-center justify-between mb-2">
            <span class="text-[10px] text-t-muted uppercase tracking-wider">已发布公告</span>
            <button @click="loadAdminAnnouncements" class="text-[10px] text-t-muted hover:text-white transition px-2 py-1 rounded hover:bg-s-hover">刷新</button>
          </div>
          <div v-if="adminAnnouncements.length === 0" class="text-t-faint text-center py-4 text-[10px]">暂无公告</div>
          <div v-else class="max-h-[280px] overflow-y-auto scroll-thin divide-y divide-b-panel rounded-lg bg-s-inset/30">
            <div v-for="a in adminAnnouncements" :key="a.id" class="px-4 py-2.5 hover:bg-s-hover transition group">
              <div class="flex items-start gap-3">
                <div class="flex-1 min-w-0">
                  <div class="text-[11px] font-bold text-t-primary truncate">{{ a.title }}</div>
                  <div class="text-[10px] text-t-muted mt-0.5 line-clamp-2">{{ a.content }}</div>
                  <div class="text-[9px] text-t-faint mt-0.5">{{ formatTime(a.created_at) }}</div>
                </div>
                <button @click.stop="deleteAnnouncement(a.id)"
                  class="flex-none opacity-0 group-hover:opacity-100 text-t-muted hover:text-red-400 p-1 rounded hover:bg-red-900/20 transition">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useAdmin } from '../../composables/useAdmin'

const {
  adminTab,
  notifyForm, adminNotifications, adminNotifsLoading,
  sendNotification, loadAdminNotifications, deleteAdminNotification,
  annForm, adminAnnouncements,
  loadAdminAnnouncements, createAnnouncement, deleteAnnouncement,
} = useAdmin()

const notifCollapsed = ref(false)
const annCollapsed = ref(false)

async function handleSendNotification() {
  await sendNotification()
  await loadAdminNotifications()
}

async function handleCreateAnnouncement() {
  await createAnnouncement()
  await loadAdminAnnouncements()
}

function formatTime(dateStr: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return isNaN(d.getTime()) ? dateStr : d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}
</script>
