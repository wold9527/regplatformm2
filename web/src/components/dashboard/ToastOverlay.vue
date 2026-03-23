<template>
  <Teleport to="body">
    <div class="fixed bottom-6 right-6 z-[60] flex flex-col items-end gap-2">
      <!-- 普通 toast -->
      <div v-for="t in toasts" :key="t.id"
        class="animate-in glass-light rounded-lg px-3.5 py-2 text-xs shadow-xl max-w-xs"
        :class="t.type === 'success' ? 'border border-green-500/30 text-ok' : t.type === 'error' ? 'border border-red-500/30 text-err' : 'text-t-primary'">
        {{ t.msg }}
      </div>

      <!-- 完成通知 toast（队列，一次显示一个） -->
      <Transition name="completion-toast">
        <div v-if="activeCompletion && completionVisible" class="completion-card">
          <div class="flex items-center gap-3">
            <!-- 平台图标 -->
            <div class="flex-none w-9 h-9 rounded-xl flex items-center justify-center text-base font-bold"
              :class="{
                'bg-blue-500/20 text-blue-400': activeCompletion.platform === 'Grok',
                'bg-green-500/20 text-green-400': activeCompletion.platform === 'OpenAI',
                'bg-amber-500/20 text-amber-400': activeCompletion.platform === 'Kiro',
                'bg-purple-500/20 text-purple-400': activeCompletion.platform === 'Gemini',
              }">
              {{ activeCompletion.platform === 'Grok' ? 'G' : activeCompletion.platform === 'OpenAI' ? 'O' : activeCompletion.platform === 'Gemini' ? 'Ge' : 'K' }}
            </div>
            <!-- 内容 -->
            <div class="flex-1 min-w-0">
              <div class="text-xs font-bold text-t-primary truncate">{{ activeCompletion.username }}</div>
              <div class="text-[11px] text-t-secondary">
                注册了
                <span class="font-bold text-ok">{{ activeCompletion.success }}</span>
                个
                <span class="font-bold" :class="{
                  'text-blue-400': activeCompletion.platform === 'Grok',
                  'text-green-400': activeCompletion.platform === 'OpenAI',
                  'text-amber-400': activeCompletion.platform === 'Kiro',
                  'text-purple-400': activeCompletion.platform === 'Gemini',
                }">{{ activeCompletion.platform }}</span>
                账号
              </div>
            </div>
            <!-- 时间 -->
            <div class="flex-none text-[10px] text-t-faint font-mono">{{ activeCompletion.time }}</div>
          </div>
          <!-- 底部进度条（自动缩短，指示剩余展示时间） -->
          <div class="completion-progress"></div>
        </div>
      </Transition>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useDashboard } from '../../composables/useDashboard'

const { toasts, activeCompletion, completionVisible } = useDashboard()
</script>

<style scoped>
.completion-card {
  position: relative;
  overflow: hidden;
  min-width: 260px;
  max-width: 320px;
  padding: 12px 14px 14px;
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.92), rgba(30, 41, 59, 0.88));
  border: 1px solid rgba(56, 189, 248, 0.15);
  box-shadow:
    0 8px 32px rgba(0, 0, 0, 0.4),
    0 0 0 1px rgba(255, 255, 255, 0.03),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
  backdrop-filter: blur(16px);
}

.completion-progress {
  position: absolute;
  bottom: 0;
  left: 0;
  height: 2px;
  width: 100%;
  background: linear-gradient(90deg, #38bdf8, #22d3ee, #34d399);
  animation: progress-shrink 4s linear forwards;
  border-radius: 0 0 14px 14px;
}

@keyframes progress-shrink {
  from { width: 100%; }
  to { width: 0%; }
}

/* 入场：从右滑入 + 淡入 */
.completion-toast-enter-active {
  transition: all 0.35s cubic-bezier(0.16, 1, 0.3, 1);
}
.completion-toast-enter-from {
  opacity: 0;
  transform: translateX(80px) scale(0.92);
}

/* 退场：向右滑出 + 淡出 */
.completion-toast-leave-active {
  transition: all 0.35s cubic-bezier(0.55, 0, 1, 0.45);
}
.completion-toast-leave-to {
  opacity: 0;
  transform: translateX(60px) scale(0.95);
}
</style>
