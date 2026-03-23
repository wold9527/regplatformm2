<template>
  <div class="w-[360px] xl:w-[420px] flex-none flex flex-col min-w-0 glass rounded-xl mobile-log-panel">
    <!-- 顶栏 -->
    <div class="flex-none flex items-center justify-between px-4 py-2.5">
      <div class="flex items-center gap-2.5">
        <span class="text-xs font-bold text-t-muted uppercase tracking-wider tip tip-bottom" data-tip="注册任务的实时运行日志">实时日志</span>
        <template v-if="isRunning">
          <span class="relative flex h-1.5 w-1.5">
            <span class="pulse-ring absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-green-500"></span>
          </span>
        </template>
        <span class="text-[10px] text-t-faint">{{ logs.length }} 条</span>
      </div>
      <button @click="logs.splice(0)" class="text-[10px] text-t-faint hover:text-gray-400 transition px-2 py-0.5 tip tip-bottom" data-tip="清空当前显示的日志">清空</button>
    </div>

    <!-- 日志区域 -->
    <div class="flex-1 log-panel overflow-y-auto scroll-thin p-3 space-y-0.5" ref="logPanel">
      <!-- 空状态：可爱的等待提示 -->
      <div v-if="logs.length === 0 && !isRunning" class="flex flex-col items-center justify-center py-14 gap-2">
        <span class="text-2xl">☕</span>
        <span class="text-t-faint text-xs">等待任务启动...</span>
      </div>

      <!-- 运行中但还没日志：loading 动画 -->
      <div v-else-if="logs.length === 0 && isRunning" class="flex flex-col items-center justify-center py-14 gap-3">
        <div class="log-spinner"></div>
        <span class="text-t-muted text-xs">正在连接...</span>
      </div>

      <!-- 日志条目 -->
      <template v-else>
        <div v-for="(line, i) in logs" :key="i"
          class="animate-in px-1.5 py-0.5 text-[10px] font-mono leading-4 rounded flex items-start gap-1.5"
          :class="logClass(line)">
          <span class="flex-shrink-0 w-3 text-center">{{ logIcon(line) }}</span>
          <span class="flex-1 break-all">{{ line }}</span>
        </div>
      </template>

      <!-- 注册进行中（日志列表最底部，始终可见） -->
      <div v-if="showHeartbeat" class="px-2 py-1.5 mt-1 rounded-lg flex items-center gap-2.5 log-heartbeat" style="background: var(--bg-inset)">
        <div class="log-breath"></div>
        <span class="text-[10px] text-t-secondary">{{ heartbeatText }}</span>
      </div>

      <!-- 排队状态（日志列表最底部） -->
      <div v-if="isQueued" class="px-2 py-1.5 mt-1 rounded-lg flex items-center gap-2.5" style="background: var(--bg-inset)">
        <div class="log-bounce">
          <span class="log-bounce-dot"></span>
          <span class="log-bounce-dot"></span>
          <span class="log-bounce-dot"></span>
        </div>
        <span class="text-[10px] text-t-secondary">排队等待中...</span>
      </div>
    </div>

    <!-- 进度条 -->
    <div v-if="isRunning || (taskStatus.target > 0 && !taskStatus.is_done)"
      class="flex-none px-4 py-2 flex items-center gap-3 section-divide tip" data-tip="当前任务完成进度">
      <div class="flex-1 h-2 rounded-full overflow-hidden" style="background:var(--bg-inset)">
        <div class="h-full rounded-full transition-all duration-500 relative progress-bar-glow"
          :style="`width:${progressPct}%`"></div>
      </div>
      <span class="font-mono font-bold text-accent text-[10px]">{{ progressPct }}%</span>
      <span class="text-[10px] text-t-muted">{{ taskStatus.success_count }}/{{ taskStatus.target }}</span>
    </div>

    <!-- 全站动态（底部独立区域） -->
    <div v-if="recentCompletions.length > 0" class="flex-none rc-section">
      <!-- 分割标题：渐变线 + 标签 -->
      <div class="flex items-center gap-2 px-3 pt-3 pb-1.5">
        <div class="flex-1 h-px" style="background: linear-gradient(to right, var(--border-subtle), transparent)"></div>
        <div class="flex items-center gap-1.5 flex-none">
          <span class="relative flex h-1.5 w-1.5">
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-60"></span>
            <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-cyan-500"></span>
          </span>
          <span class="text-[9px] font-bold text-t-faint uppercase tracking-[0.12em]">全站动态</span>
        </div>
        <div class="flex-1 h-px" style="background: linear-gradient(to left, var(--border-subtle), transparent)"></div>
      </div>
      <TransitionGroup name="rc-list" tag="div" class="overflow-y-auto scroll-thin max-h-[220px] px-2 pb-2 flex flex-col gap-1.5 relative">
        <div v-for="(item, idx) in recentCompletions" :key="item.task_id"
          class="rc-item" :class="{ 'rc-item-latest': idx === 0 }">
          <!-- 左侧平台渐变条 -->
          <div class="rc-accent" :class="rcAccentClass(item.platform)"></div>
          <div class="flex items-center gap-2.5 px-3 py-2.5">
            <!-- 用户头像（圆形，失败时显示默认首字母头像） -->
            <div class="rc-avatar-wrap flex-shrink-0">
              <img
                v-if="item.avatar_url"
                :src="item.avatar_url"
                class="rc-avatar"
                :alt="item.name || item.username"
                @error="($event.target as HTMLImageElement).style.display = 'none'"
              />
              <div class="rc-avatar-fallback" :class="rcAvatarFallbackClass(item.platform)">
                {{ rcAvatarLetter(item) }}
              </div>
            </div>
            <!-- 用户信息 -->
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-1.5">
                <span class="text-[11px] font-semibold text-t-primary truncate">{{ item.name || item.username || '匿名' }}</span>
                <span v-if="idx === 0" class="rc-latest-badge">最新</span>
              </div>
              <div class="flex items-center gap-1.5 mt-0.5">
                <!-- 平台名称：对应颜色高亮 -->
                <span class="text-[10px] font-semibold" :class="rcPlatformTextClass(item.platform)">{{ rcPlatformLabel(item.platform) }}</span>
                <span class="rc-dot"></span>
                <span class="text-[10px] text-t-faint font-mono" v-if="item.duration_sec">{{ rcDuration(item.duration_sec) }}</span>
              </div>
              <!-- 随机赞美语 -->
              <div class="text-[9px] text-t-faint mt-0.5 italic">{{ rcPraise(item.task_id) }}</div>
            </div>
            <!-- 注册数 + 时间 -->
            <div class="flex-none text-right">
              <div class="rc-count" :class="rcCountClass(item.platform)">+{{ item.success_count }}</div>
              <div class="text-[9px] text-t-faint font-mono mt-0.5">{{ rcTimeAgo(item.stopped_at) }}</div>
            </div>
          </div>
        </div>
      </TransitionGroup>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch, onUnmounted } from 'vue'
import { useDashboard } from '../../composables/useDashboard'

const { isRunning, isQueued, logs, logPanel, taskStatus, lastLogAt, elapsedSeconds, recentCompletions } = useDashboard()

// ── 全站动态：平台样式 & 相对时间 ──

const rcAccentMap: Record<string, string> = { grok: 'rc-accent-grok', openai: 'rc-accent-openai', kiro: 'rc-accent-kiro', gemini: 'rc-accent-gemini' }
const rcCountMap: Record<string, string> = { grok: 'rc-count-grok', openai: 'rc-count-openai', kiro: 'rc-count-kiro', gemini: 'rc-count-gemini' }
const rcLabelMap: Record<string, string> = { grok: 'Grok', openai: 'OpenAI', kiro: 'Kiro', gemini: 'Gemini' }
// 平台名称对应的文字高亮色 class
const rcPlatformTextMap: Record<string, string> = {
  grok: 'rc-text-grok',
  openai: 'rc-text-openai',
  kiro: 'rc-text-kiro',
  gemini: 'rc-text-gemini',
}
// 头像 fallback 背景色
const rcAvatarFallbackMap: Record<string, string> = {
  grok: 'rc-avatar-fb-grok',
  openai: 'rc-avatar-fb-openai',
  kiro: 'rc-avatar-fb-kiro',
  gemini: 'rc-avatar-fb-gemini',
}

// 随机赞美语池
const rcPraisePool = [
  '恭喜🎉', '太棒了🎊', '厉害👏', '666🔥', '大佬牛🐂',
  '起飞✈️', '冲冲冲🚀', '无敌了💪', '稳住🎯', '绝了✨',
  '芜湖🦅', '燃！🔥', '满分操作👍', '顶！🏆', '牛啊🌟',
]
// 用 task_id 做伪随机种子，保证同一条数据每次渲染赞美语一致（不乱跳）
function rcPraise(taskId: number): string {
  return rcPraisePool[taskId % rcPraisePool.length]
}

function rcAccentClass(p: string) { return rcAccentMap[p] || rcAccentMap.grok }
function rcCountClass(p: string) { return rcCountMap[p] || rcCountMap.grok }
function rcPlatformLabel(p: string) { return rcLabelMap[p] || p }
function rcPlatformTextClass(p: string) { return rcPlatformTextMap[p] || rcPlatformTextMap.grok }
function rcAvatarFallbackClass(p: string) { return rcAvatarFallbackMap[p] || rcAvatarFallbackMap.grok }

// 头像 fallback 显示首字：优先 name，其次 username，都没有取平台首字母
function rcAvatarLetter(item: { name?: string; username?: string; platform: string }): string {
  const src = item.name || item.username || item.platform
  return src.charAt(0).toUpperCase()
}

function rcTimeAgo(ts: number): string {
  if (!ts) return ''
  const diff = Math.floor(Date.now() / 1000) - ts
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
  return `${Math.floor(diff / 86400)}天前`
}

function rcDuration(sec: number): string {
  if (!sec || sec <= 0) return ''
  if (sec < 60) return `${sec}秒`
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return s > 0 ? `${m}分${s}秒` : `${m}分钟`
}

// 进度百分比
const progressPct = computed(() =>
  taskStatus.target ? Math.min(100, Math.round((taskStatus.success_count / (taskStatus.target || 1)) * 100)) : 0
)

// ── 活跃指示器：任务在跑但超过 5 秒没新日志时显示 ──
const idleSeconds = ref(0)
let idleTimer: ReturnType<typeof setInterval> | null = null

// 是否显示活跃指示器（任务运行期间始终显示）
const showHeartbeat = computed(() =>
  isRunning.value && !isQueued.value
)

// 活跃指示器文案：根据进度动态变化
const heartbeatText = computed(() => {
  const done = taskStatus.success_count
  const total = taskStatus.target
  if (done > 0 && total > 0) return `正在注册第 ${done + 1}/${total} 个...`
  return `注册进行中...`
})

function startIdleTimer() {
  stopIdleTimer()
  idleSeconds.value = 0
  idleTimer = setInterval(() => { idleSeconds.value++ }, 1000)
}
function stopIdleTimer() {
  if (idleTimer) { clearInterval(idleTimer); idleTimer = null }
  idleSeconds.value = 0
}

// 新日志到达时重置空闲计时
watch(lastLogAt, () => { idleSeconds.value = 0 })
// 任务启停时控制计时器
watch(isRunning, (v) => { v ? startIdleTimer() : stopIdleTimer() }, { immediate: true })
onUnmounted(stopIdleTimer)

// 日志分类样式
function logClass(line: string): string {
  if (line.includes('[✓]') || line.includes('注册成功') || line.includes('注册完成')) return 'text-ok font-bold log-success'
  if (line.includes('[+]') || line.includes('[OK]')) return 'text-ok'
  if (line.includes('[-]') || line.includes('失败')) return 'text-err'
  if (line.includes('[!]')) return 'text-warn'
  if (line.includes('[*]') || line.includes('进度')) return 'text-info'
  if (line.includes('════')) return 'text-t-secondary'
  return 'text-t-muted'
}

// 日志图标
function logIcon(line: string): string {
  if (line.includes('[✓]') || line.includes('注册成功') || line.includes('注册完成')) return '✅'
  if (line.includes('[+]') || line.includes('[OK]')) return '🟢'
  if (line.includes('[-]') || line.includes('失败')) return '🔴'
  if (line.includes('[!]')) return '⚠'
  if (line.includes('排队') || line.includes('等待')) return '⏳'
  if (line.includes('任务完成')) return '🎉'
  if (line.includes('第 ') || line.includes('进度')) return '📊'
  if (line.includes('耗时')) return '⏱'
  if (line.includes('════')) return '─'
  return '·'
}

</script>

<style scoped>
/* 三点弹跳动画 */
.log-bounce {
  display: flex;
  gap: 3px;
  align-items: center;
}
.log-bounce-dot {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--c-accent);
  animation: logBounce 1.2s ease-in-out infinite;
}
.log-bounce-dot:nth-child(2) { animation-delay: 0.15s; }
.log-bounce-dot:nth-child(3) { animation-delay: 0.3s; }
@keyframes logBounce {
  0%, 80%, 100% { transform: translateY(0); opacity: 0.4; }
  40% { transform: translateY(-5px); opacity: 1; }
}

/* 连接中 spinner */
.log-spinner {
  width: 20px;
  height: 20px;
  border: 2px solid var(--border-subtle);
  border-top-color: var(--c-accent);
  border-radius: 50%;
  animation: logSpin 0.8s linear infinite;
}
@keyframes logSpin {
  to { transform: rotate(360deg); }
}

/* 活跃指示器：呼吸圆点 */
.log-breath {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-info);
  animation: logBreath 2s ease-in-out infinite;
}
@keyframes logBreath {
  0%, 100% { opacity: 0.3; transform: scale(0.85); }
  50% { opacity: 1; transform: scale(1.1); }
}
.log-heartbeat {
  animation: fadeIn 0.3s ease-out;
}

/* ── 全站动态 ── */

.rc-section {
  border-top: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.06));
}

/* 普通条目 */
.rc-item {
  position: relative;
  border-radius: 10px;
  background: rgba(30, 41, 59, 0.35);
  border: 1px solid rgba(148, 163, 184, 0.08);
  overflow: hidden;
  transition: all 0.25s ease;
  flex-shrink: 0;
}
.rc-item:hover {
  background: rgba(30, 41, 59, 0.5);
  border-color: rgba(148, 163, 184, 0.15);
  transform: translateX(2px);
}

/* ★ 最新一条：渐变发光边框 + 微光动画 */
.rc-item-latest {
  background: rgba(30, 41, 59, 0.5);
  border-color: transparent;
  box-shadow:
    0 0 0 1px rgba(56, 189, 248, 0.2),
    0 0 20px rgba(56, 189, 248, 0.06),
    0 4px 16px rgba(0, 0, 0, 0.12);
}
.rc-item-latest::before {
  content: '';
  position: absolute;
  inset: -1px;
  border-radius: 11px;
  padding: 1px;
  background: linear-gradient(135deg, rgba(56, 189, 248, 0.4), rgba(139, 92, 246, 0.3), rgba(56, 189, 248, 0.1));
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  animation: rcGlow 3s ease-in-out infinite;
  pointer-events: none;
}
@keyframes rcGlow {
  0%, 100% { opacity: 0.6; }
  50% { opacity: 1; }
}
.rc-item-latest:hover {
  box-shadow:
    0 0 0 1px rgba(56, 189, 248, 0.3),
    0 0 28px rgba(56, 189, 248, 0.1),
    0 4px 20px rgba(0, 0, 0, 0.15);
  transform: translateX(2px);
}

/* LATEST 小徽章 */
.rc-latest-badge {
  font-size: 8px;
  font-weight: 700;
  letter-spacing: 0.05em;
  padding: 1px 5px;
  border-radius: 4px;
  background: linear-gradient(135deg, rgba(56, 189, 248, 0.2), rgba(139, 92, 246, 0.15));
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.2);
  flex-shrink: 0;
  line-height: 1.4;
}

/* 左侧平台渐变条 */
.rc-accent {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 2.5px;
  border-radius: 2px 0 0 2px;
}
.rc-accent-grok { background: linear-gradient(180deg, #3b82f6, #22d3ee); }
.rc-accent-openai { background: linear-gradient(180deg, #22c55e, #34d399); }
.rc-accent-kiro { background: linear-gradient(180deg, #f59e0b, #fb923c); }
.rc-accent-gemini { background: linear-gradient(180deg, #a855f7, #d946ef); }

/* ── 用户头像区域 ── */
.rc-avatar-wrap {
  position: relative;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  overflow: hidden;
}
/* 实际 img，覆盖在 fallback 上方 */
.rc-avatar {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 50%;
  z-index: 1;
}
/* 默认首字母头像（始终存在，img 加载成功后被遮住） */
.rc-avatar-fallback {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 700;
  border-radius: 50%;
  z-index: 0;
  user-select: none;
}
.rc-avatar-fb-grok   { background: rgba(59, 130, 246, 0.18); color: #60a5fa; }
.rc-avatar-fb-openai { background: rgba(34, 197, 94, 0.18);  color: #4ade80; }
.rc-avatar-fb-kiro   { background: rgba(245, 158, 11, 0.18); color: #fbbf24; }
.rc-avatar-fb-gemini { background: rgba(168, 85, 247, 0.18); color: #c084fc; }

/* 平台名称高亮文字色 */
.rc-text-grok   { color: #60a5fa; }
.rc-text-openai { color: #4ade80; }
.rc-text-kiro   { color: #fbbf24; }
.rc-text-gemini { color: #c084fc; }

/* 分隔小点 */
.rc-dot {
  width: 2px;
  height: 2px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.3;
  flex-shrink: 0;
}

/* 注册数量徽章 */
.rc-count {
  font-size: 12px;
  font-weight: 800;
  padding: 2px 8px;
  border-radius: 6px;
  line-height: 1.3;
  letter-spacing: -0.02em;
}
.rc-count-grok   { background: rgba(59, 130, 246, 0.15); color: #60a5fa; }
.rc-count-openai { background: rgba(34, 197, 94, 0.15);  color: #4ade80; }
.rc-count-kiro   { background: rgba(245, 158, 11, 0.15); color: #fbbf24; }
.rc-count-gemini { background: rgba(168, 85, 247, 0.15); color: #c084fc; }

/* ── 浅色模式适配 ── */
@media (prefers-color-scheme: light) {
  .rc-item {
    background: rgba(255, 255, 255, 0.7);
    border-color: rgba(148, 163, 184, 0.15);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  }
  .rc-item:hover {
    background: rgba(255, 255, 255, 0.9);
    border-color: rgba(148, 163, 184, 0.25);
    box-shadow: 0 3px 12px rgba(0, 0, 0, 0.06);
  }
  .rc-item-latest {
    background: rgba(255, 255, 255, 0.9);
    box-shadow:
      0 0 0 1px rgba(59, 130, 246, 0.15),
      0 0 16px rgba(59, 130, 246, 0.06),
      0 4px 16px rgba(0, 0, 0, 0.06);
  }
  .rc-item-latest::before {
    background: linear-gradient(135deg, rgba(59, 130, 246, 0.35), rgba(139, 92, 246, 0.25), rgba(59, 130, 246, 0.1));
  }
  .rc-item-latest:hover {
    box-shadow:
      0 0 0 1px rgba(59, 130, 246, 0.2),
      0 0 24px rgba(59, 130, 246, 0.08),
      0 4px 20px rgba(0, 0, 0, 0.08);
  }
  .rc-latest-badge {
    background: linear-gradient(135deg, rgba(59, 130, 246, 0.12), rgba(139, 92, 246, 0.08));
    color: #2563eb;
    border-color: rgba(59, 130, 246, 0.15);
  }
  /* 头像 fallback：浅色模式加深背景对比 */
  .rc-avatar-fb-grok   { background: rgba(59, 130, 246, 0.12); color: #2563eb; }
  .rc-avatar-fb-openai { background: rgba(34, 197, 94, 0.12);  color: #16a34a; }
  .rc-avatar-fb-kiro   { background: rgba(245, 158, 11, 0.12); color: #d97706; }
  .rc-avatar-fb-gemini { background: rgba(168, 85, 247, 0.12); color: #7c3aed; }
  /* 平台高亮文字：浅色模式加深 */
  .rc-text-grok   { color: #2563eb; }
  .rc-text-openai { color: #16a34a; }
  .rc-text-kiro   { color: #d97706; }
  .rc-text-gemini { color: #7c3aed; }
  /* 注册数徽章 */
  .rc-count-grok   { background: rgba(59, 130, 246, 0.1); color: #2563eb; }
  .rc-count-openai { background: rgba(34, 197, 94, 0.1);  color: #16a34a; }
  .rc-count-kiro   { background: rgba(245, 158, 11, 0.1); color: #d97706; }
  .rc-count-gemini { background: rgba(168, 85, 247, 0.1); color: #7c3aed; }
}

/* TransitionGroup：新条目从右侧滑入，已有条目被挤下去 */
.rc-list-enter-active {
  transition: all 0.5s cubic-bezier(0.16, 1, 0.3, 1);
}
.rc-list-enter-from {
  opacity: 0;
  transform: translateX(80px) scale(0.95);
}
.rc-list-move {
  transition: transform 0.5s cubic-bezier(0.16, 1, 0.3, 1);
}
.rc-list-leave-active {
  transition: all 0.3s ease-in;
  position: absolute;
  left: 8px;
  right: 8px;
}
.rc-list-leave-to {
  opacity: 0;
  transform: translateX(-30px) scale(0.95);
}
</style>
