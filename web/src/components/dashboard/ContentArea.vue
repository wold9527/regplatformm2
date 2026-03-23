<template>
  <div class="flex-1 flex flex-col min-h-0 gap-2.5 mobile-full" @click="exportMenuOpen = null">

    <!-- 任务通知条 -->
    <div v-if="taskNotice.msg" class="flex-none glass rounded-xl px-4 py-2 flex items-center gap-2.5 animate-in"
      :class="{
        'border border-green-500/20': taskNotice.type === 'success',
        'border border-red-500/20': taskNotice.type === 'error',
        'border border-blue-500/20': taskNotice.type === 'info',
      }">
      <div class="flex-none w-5 h-5 rounded-full flex items-center justify-center"
        :class="{
          'bg-ok-dim': taskNotice.type === 'success',
          'bg-err-dim': taskNotice.type === 'error',
          'bg-blue-900/40': taskNotice.type === 'info',
        }">
        <svg v-if="taskNotice.type === 'success'" class="w-3 h-3 text-ok" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/></svg>
        <svg v-else-if="taskNotice.type === 'error'" class="w-3 h-3 text-err" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"/></svg>
        <svg v-else class="w-3 h-3 text-info" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"/></svg>
      </div>
      <span class="text-xs flex-1"
        :class="{
          'text-ok': taskNotice.type === 'success',
          'text-err': taskNotice.type === 'error',
          'text-info': taskNotice.type === 'info',
        }">{{ taskNotice.msg }}</span>
      <span class="text-[10px] text-t-faint flex-none">{{ taskNotice.time }}</span>
    </div>

    <!-- 任务信息卡 -->
    <div class="flex-none glass rounded-xl task-card-glow overflow-hidden"
      :class="isRunning ? 'task-card-running' : isQueued ? 'task-card-running' : ''">
      <div class="px-4 py-3 flex items-center gap-4">
        <!-- 左：状态指示器 -->
        <div class="flex items-center gap-2.5 flex-none">
          <div class="relative">
            <div class="w-9 h-9 rounded-lg flex items-center justify-center"
              :class="isRunning ? 'bg-green-500/15' : isQueued ? 'bg-amber-500/15' : taskStatus.task_id && taskStatus.is_done ? 'bg-gray-500/15' : taskStatus.task_id ? 'bg-amber-500/15' : 'bg-blue-500/15'">
              <svg v-if="isRunning" class="w-4 h-4 text-ok" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd"/></svg>
              <svg v-else-if="isQueued" class="w-4 h-4 text-warn" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
              <svg v-else-if="taskStatus.task_id && taskStatus.is_done" class="w-4 h-4 text-t-secondary" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
              <svg v-else-if="taskStatus.task_id" class="w-4 h-4 text-warn" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd"/></svg>
              <svg v-else class="w-4 h-4 text-info" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg>
            </div>
            <span v-if="isRunning" class="absolute -top-0.5 -right-0.5 flex h-2.5 w-2.5">
              <span class="pulse-ring absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
              <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-green-500"></span>
            </span>
            <span v-else-if="isQueued" class="absolute -top-0.5 -right-0.5 flex h-2.5 w-2.5">
              <span class="pulse-ring absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
              <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-amber-500"></span>
            </span>
          </div>
          <div>
            <div class="flex items-center gap-1.5">
              <span class="text-sm font-bold" :class="isRunning ? 'text-ok' : isQueued ? 'text-warn' : taskStatus.task_id && taskStatus.is_done ? 'text-t-secondary' : taskStatus.task_id ? 'text-warn' : 'text-info'">
                {{ isRunning ? '运行中' : isQueued ? '排队中' : taskStatus.task_id && taskStatus.is_done ? '已完成' : taskStatus.task_id ? '已停止' : '待启动' }}
              </span>
              <span v-if="taskStatus.task_id" class="text-[10px] font-mono text-t-faint">#{{ taskStatus.task_id }}</span>
            </div>
            <div class="text-[10px] text-t-muted mt-0.5">{{ ({ grok: 'Grok', openai: 'OpenAI', kiro: 'Kiro', gemini: 'Gemini' }[taskForm.platform]) || taskForm.platform }}</div>
          </div>
        </div>

        <!-- 中：指标面板 -->
        <div class="flex-1 flex items-center gap-2">
          <div class="flex-1 grid gap-2" :class="refundAmount > 0 ? 'grid-cols-3' : 'grid-cols-2'">
            <div class="text-center px-2 py-1.5 rounded-lg bg-s-panel tip" data-tip="本次任务的目标注册数量">
              <div class="text-[10px] text-t-muted uppercase tracking-wider">目标</div>
              <div class="text-sm font-bold text-t-primary font-mono mt-0.5">{{ isRunning ? taskStatus.target : taskForm.target_count }}</div>
            </div>
            <div class="text-center px-2 py-1.5 rounded-lg bg-s-panel tip" data-tip="并发线程数，系统根据数量自动分配">
              <div class="text-[10px] text-t-muted uppercase tracking-wider">线程</div>
              <div class="text-sm font-bold text-t-primary font-mono mt-0.5">{{ taskForm.thread_count }}</div>
            </div>
            <div v-if="refundAmount > 0" class="text-center px-2 py-1.5 rounded-lg bg-cyan-900/20 tip" data-tip="任务结束后退还的未使用次数">
              <div class="text-[10px] text-accent uppercase tracking-wider">退还</div>
              <div class="text-sm font-bold text-accent font-mono mt-0.5">{{ refundAmount }}</div>
            </div>
          </div>
        </div>

        <!-- 中右：实时计数 — 带背景色块，提升识别度 -->
        <div class="flex items-center gap-2 flex-none">
          <div class="text-center px-3 py-1.5 rounded-lg tip" style="background: var(--c-success-dim)" data-tip="已成功的注册数">
            <div class="text-xl font-black font-mono text-ok leading-none tabular-nums">{{ taskStatus.success_count }}</div>
            <div class="text-[9px] text-ok/70 mt-0.5 uppercase tracking-wider font-semibold">成功</div>
          </div>
          <div class="text-t-faint text-xs font-light">/</div>
          <div class="text-center px-3 py-1.5 rounded-lg tip" style="background: var(--c-danger-dim)" data-tip="已失败的注册数">
            <div class="text-xl font-black font-mono text-err leading-none tabular-nums">{{ taskStatus.fail_count }}</div>
            <div class="text-[9px] text-err/70 mt-0.5 uppercase tracking-wider font-semibold">失败</div>
          </div>
        </div>

        <!-- 右：用时 + 保存 -->
        <div class="flex items-center gap-2.5 flex-none">
          <div v-if="isRunning || elapsedSeconds > 0" class="text-center tip" data-tip="任务已运行时长">
            <div class="text-sm font-mono font-bold text-accent">{{ formatElapsed(elapsedSeconds) }}</div>
            <div class="text-[10px] text-accent/70">用时</div>
          </div>
          <div v-if="estimatedRemaining" class="text-center tip" data-tip="根据当前速度估算的剩余时间">
            <div class="text-[11px] font-mono text-warn">{{ estimatedRemaining }}</div>
          </div>
          <button v-if="taskStatus.success_count > 0" @click="saveCurrentTaskResults"
            :disabled="!!exportingMap['save']"
            class="px-3 py-1.5 rounded-lg text-[11px] font-semibold bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white transition-all shadow-lg shadow-emerald-900/30 flex items-center gap-1.5 disabled:opacity-50 tip" data-tip="将注册结果导出为 ZIP（Grok→token.txt, OpenAI/Kiro/Gemini→JSON）">
            <svg v-if="exportingMap['save']" class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
            </svg>
            <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>
            {{ exportingMap['save'] ? '导出中...' : `保存 ${taskStatus.success_count} 个` }}
          </button>
        </div>
      </div>
      <!-- 迷你进度条 -->
      <div v-if="isRunning && taskStatus.target > 0" class="h-0.5 w-full bg-s-hover">
        <div class="h-full transition-all duration-500 progress-bar-glow"
          :style="`width:${Math.min(100, (taskStatus.success_count / (taskStatus.target || 1)) * 100)}%`"></div>
      </div>
    </div>

    <!-- 公告栏（可折叠，独立于平台网格） -->
    <div v-if="announcements.length > 0" class="flex-none glass rounded-xl overflow-hidden">
      <div class="flex items-center justify-between px-3 py-2 cursor-pointer hover:bg-s-hover transition" @click="announcementOpen = !announcementOpen">
        <div class="flex items-center gap-1.5">
          <svg class="w-3.5 h-3.5 text-warn" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z"/></svg>
          <span class="text-xs font-bold text-warn">公告</span>
          <span class="text-[10px] text-t-faint font-mono">{{ announcements.length }}</span>
          <!-- 折叠时显示最新公告标题预览 -->
          <span v-if="!announcementOpen && announcements.length > 0" class="text-[10px] text-t-muted truncate max-w-[200px]">— {{ announcements[0].title }}</span>
        </div>
        <div class="flex items-center gap-1">
          <button @click.stop="loadAnnouncements" class="text-[10px] text-t-faint hover:text-t-secondary transition px-1.5 py-0.5 rounded hover:bg-s-hover">刷新</button>
          <svg class="w-3 h-3 text-t-faint transition-transform" :class="announcementOpen ? 'rotate-180' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
        </div>
      </div>
      <div v-if="announcementOpen" class="max-h-32 overflow-y-auto scroll-thin px-3 pb-2 space-y-2">
        <div v-for="ann in announcements" :key="ann.id"
          class="glass-light rounded-lg px-3 py-2.5 space-y-1">
          <div class="flex items-center justify-between">
            <span class="text-[11px] font-bold text-warn">{{ ann.title }}</span>
            <span class="text-[10px] text-t-faint">{{ new Date(ann.created_at).toLocaleDateString() }}</span>
          </div>
          <div class="text-[10px] text-t-secondary leading-relaxed whitespace-pre-wrap">{{ ann.content }}</div>
        </div>
      </div>
    </div>

    <!-- 2×2 平台网格 -->
    <div class="flex-1 grid grid-cols-2 grid-rows-2 min-h-0 gap-2.5 mobile-grid-1">

      <!-- 平台列 -->
      <div v-for="p in platformCols" :key="p.key" class="flex flex-col min-h-0 glass rounded-xl overflow-hidden">
        <!-- 头部：左侧色条 + 标签 + 计数 + 操作 -->
        <div class="flex-none flex items-center justify-between px-3 py-2 relative">
          <!-- 左侧平台色条 -->
          <div class="absolute left-0 top-0 bottom-0 w-0.5 rounded-l-xl" :class="p.dotClass"></div>
          <div class="flex items-center gap-2">
            <span class="text-xs font-bold tracking-tight" :class="p.textClass">{{ p.label }}</span>
            <!-- 计数胶囊：有数据时用平台色调高亮 -->
            <span class="text-[10px] font-mono font-semibold px-1.5 py-0.5 rounded-md leading-none"
              :class="platformResults[p.key].length > 0 ? p.countBadgeClass : 'text-t-faint'">
              {{ platformResults[p.key].length }}
            </span>
          </div>
          <div class="flex items-center gap-0.5">
            <!-- Kiro 独立检测按钮 -->
            <button v-if="p.key === 'kiro'" @click="validateKiro()"
              :disabled="platformResults[p.key].length === 0 || !!exportingMap['kiro_validate']"
              class="text-t-muted hover:text-cyan-400 disabled:opacity-30 p-1 rounded hover:bg-s-hover transition tip tip-bottom"
              data-tip="检测账号状态，删除失效的（不归档）">
              <svg v-if="exportingMap['kiro_validate']" class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
              </svg>
              <svg v-else class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
            </button>
            <!-- Gemini 独立检测按钮 -->
            <button v-if="p.key === 'gemini'" @click="validateGemini()"
              :disabled="platformResults[p.key].length === 0 || !!exportingMap['gemini_validate']"
              class="text-t-muted hover:text-cyan-400 disabled:opacity-30 p-1 rounded hover:bg-s-hover transition tip tip-bottom"
              data-tip="检测账号状态，删除失效的（不归档）">
              <svg v-if="exportingMap['gemini_validate']" class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
              </svg>
              <svg v-else class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
            </button>
            <button @click="archivePlatform(p.key)"
              :disabled="platformResults[p.key].length === 0 || ((p.key === 'kiro' && !!exportingMap['kiro_validate']) || (p.key === 'gemini' && !!exportingMap['gemini_validate']))"
              class="text-t-muted hover:text-amber-400 disabled:opacity-30 p-1 rounded hover:bg-s-hover transition tip tip-bottom"
              :data-tip="(p.key === 'kiro' || p.key === 'gemini') ? '验活并归档正常账号，删除失效的' : '将当前所有账号移入归档区'">
              <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"/></svg>
            </button>
            <div class="relative">
              <button @click.stop="exportMenuOpen = exportMenuOpen === p.key ? null : p.key"
                :disabled="platformResults[p.key].length === 0 || !!exportingMap[p.key] || (p.key === 'kiro' && !!exportingMap['kiro_validate']) || (p.key === 'gemini' && !!exportingMap['gemini_validate'])"
                class="disabled:opacity-30 p-1 rounded hover:bg-s-hover transition tip tip-bottom"
                :class="p.exportClass" :data-tip="exportingMap[p.key] || (p.key === 'kiro' && exportingMap['kiro_validate']) || (p.key === 'gemini' && exportingMap['gemini_validate']) ? '正在处理中...' : '导出选项'">
                <svg v-if="exportingMap[p.key] || (p.key === 'kiro' && exportingMap['kiro_validate']) || (p.key === 'gemini' && exportingMap['gemini_validate'])" class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                </svg>
                <svg v-else class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                </svg>
              </button>
              <!-- 导出下拉菜单 -->
              <div v-if="exportMenuOpen === p.key"
                class="absolute right-0 top-full mt-1 z-50 w-36 rounded-lg glass py-1 shadow-lg shadow-black/30">
                <button @click.stop="exportPlatformResults(p.key); exportMenuOpen = null"
                  class="w-full text-left px-3 py-1.5 text-[11px] text-t-secondary hover:text-t-primary hover:bg-s-hover transition tip tip-left"
                  data-tip="导出为 ZIP 文件，不影响当前列表">
                  仅导出
                </button>
                <button @click.stop="exportAndArchive(p.key); exportMenuOpen = null"
                  class="w-full text-left px-3 py-1.5 text-[11px] text-t-secondary hover:text-amber-400 hover:bg-s-hover transition tip tip-left"
                  data-tip="导出 ZIP 后自动将账号移入归档区">
                  导出并归档
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- 账号列表 -->
        <div class="flex-1 overflow-y-auto scroll-thin">
          <!-- 空状态：带平台色调圆形图标 -->
          <div v-if="platformResults[p.key].length === 0"
            class="flex flex-col items-center justify-center h-full py-6 gap-2 select-none">
            <div class="w-8 h-8 rounded-full flex items-center justify-center"
              :class="p.key === 'grok' ? 'bg-blue-500/10' : p.key === 'openai' ? 'bg-green-500/10' : p.key === 'gemini' ? 'bg-purple-500/10' : 'bg-amber-500/10'">
              <svg class="w-3.5 h-3.5 opacity-50" :class="p.textClass" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/>
              </svg>
            </div>
            <span class="text-[10px] text-t-faint">暂无 {{ p.label }} 账号</span>
          </div>
          <!-- 账号行：hover 时左侧微色条 + 复制按钮始终可见（低不透明度） -->
          <div v-for="r in displayedPlatformResults[p.key]" :key="r.id"
            class="px-3 py-1.5 border-b border-b-panel last:border-b-0 hover:bg-s-hover transition-colors duration-150 group relative"
            :class="r.disabled ? 'bg-red-950/20' : ''">
            <div class="flex items-center gap-1.5">
              <div class="flex-1 min-w-0 cursor-pointer tip tip-right" @click="copyEmailPassword(r)"
                :data-tip="r.disabled ? `已禁用：${r.disabled_reason || '未知原因'}` : '点击复制 邮箱----密码'">
                <div class="flex items-center gap-1.5 min-w-0">
                  <div class="text-[11px] truncate font-medium leading-tight"
                    :class="r.disabled ? 'text-t-muted line-through' : 'text-t-primary'">{{ r.email }}</div>
                  <!-- 禁用标签 -->
                  <span v-if="r.disabled"
                    class="flex-none text-[9px] font-bold px-1.5 py-0.5 rounded-full bg-red-500/20 text-err leading-none whitespace-nowrap">
                    已禁用
                  </span>
                </div>
                <div class="text-[10px] font-mono text-t-faint truncate mt-0.5 tracking-tight">{{ getPassword(r) }}</div>
                <!-- 禁用原因 -->
                <div v-if="r.disabled && r.disabled_reason"
                  class="text-[10px] text-err/80 truncate mt-0.5 leading-tight">
                  {{ r.disabled_reason }}
                </div>
                <!-- 最后验活时间 -->
                <div v-if="r.last_validated_at"
                  class="text-[9px] text-t-faint mt-0.5 leading-tight">
                  {{ formatRelativeTime(r.last_validated_at) }}验活
                </div>
              </div>
              <!-- 恢复按钮（仅禁用账号显示） -->
              <button v-if="r.disabled" @click.stop="reEnableAccount(r)"
                :disabled="reEnableLoading[r.id]"
                class="shrink-0 text-[10px] px-1.5 py-0.5 rounded-md border border-transparent text-ok hover:text-emerald-300 hover:bg-emerald-900/30 hover:border-emerald-500/20 transition-all tip disabled:opacity-50 disabled:pointer-events-none"
                data-tip="恢复此账号，解除禁用状态">
                {{ reEnableLoading[r.id] ? '...' : '恢复' }}
              </button>
              <button v-else @click.stop="fetchOTP(r.email)"
                class="shrink-0 text-[10px] px-1.5 py-0.5 rounded-md border border-transparent text-accent hover:text-cyan-300 hover:bg-cyan-900/30 hover:border-cyan-500/20 transition-all tip"
                :class="otpLoading[r.email] ? 'opacity-50 pointer-events-none' : ''"
                data-tip="从邮箱获取该账号的最新验证码">
                {{ otpLoading[r.email] ? '...' : 'OTP' }}
              </button>
              <span class="opacity-20 group-hover:opacity-100 transition-opacity duration-150 text-[10px] shrink-0 cursor-pointer font-semibold" :class="p.hoverClass"
                @click.stop="copyEmailPassword(r)">复制</span>
            </div>
          </div>
          <!-- 加载更多 -->
          <div v-if="displayedPlatformResults[p.key].length < platformResults[p.key].length"
            class="px-3 py-2.5 text-center">
            <button @click="showMoreResults(p.key)"
              class="text-[10px] px-3 py-1 rounded-lg text-t-secondary hover:text-t-primary hover:bg-s-hover transition">
              加载更多（已显示 {{ displayedPlatformResults[p.key].length }} / {{ platformResults[p.key].length }}）
            </button>
          </div>
        </div>

        <!-- 归档区 -->
        <div class="flex-none section-divide">
          <div class="px-3 py-1.5 flex items-center justify-between tip tip-right" data-tip="已归档的历史账号">
            <div class="flex items-center gap-1.5">
              <svg class="w-2.5 h-2.5 text-t-faint" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"/></svg>
              <span class="text-[10px] text-t-faint">归档</span>
              <svg v-if="!archivedCountLoaded" class="w-2.5 h-2.5 animate-spin text-t-faint" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
              </svg>
              <span v-else class="text-[10px] font-mono font-bold" :class="p.textClass">{{ platformArchivedCount[p.key] }}</span>
              <span v-if="lastArchivedCount[p.key] > 0" class="text-[9px] font-mono px-1 py-0.5 rounded" :class="p.archiveBadge">
                +{{ lastArchivedCount[p.key] }}
              </span>
            </div>
            <button v-if="platformArchivedCount[p.key] > 0" @click.stop="exportArchivedPlatform(p.key)"
              :disabled="!!exportingMap[p.key + '_archived'] || (p.key === 'kiro' && !!exportingMap['kiro_validate']) || (p.key === 'gemini' && !!exportingMap['gemini_validate'])"
              class="flex items-center gap-1 text-[10px] text-t-muted hover:text-t-secondary px-1.5 py-0.5 rounded hover:bg-s-hover transition disabled:opacity-30 tip tip-left"
              :data-tip="(p.key === 'kiro' || p.key === 'gemini') ? '先验活再导出，失效的自动删除' : '导出归档账号'">
              <svg v-if="exportingMap[p.key + '_archived']" class="w-2.5 h-2.5 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
              </svg>
              <svg v-else class="w-2.5 h-2.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>
              导出
            </button>
          </div>
        </div>
      </div>

    </div><!-- /2×2 网格 -->
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useDashboard } from '../../composables/useDashboard'
import { resultApi } from '../../api/client'

const {
  taskNotice, isRunning, isQueued, taskStatus, taskForm, elapsedSeconds, formatElapsed,
  estimatedRemaining, refundAmount, saveCurrentTaskResults,
  platformResults, displayedPlatformResults, showMoreResults,
  platformCols, announcements, loadAnnouncements,
  archivePlatform, exportPlatformResults, exportAndArchive, exportArchivedPlatform,
  platformArchivedCount, archivedCountLoaded, lastArchivedCount,
  otpLoading, fetchOTP, copyEmailPassword, getPassword,
  exporting, exportingMap, validateKiro, validateGemini,
  toast, loadResults,
} = useDashboard()

// 控制哪个平台的导出下拉菜单打开（null 表示全部关闭）
const exportMenuOpen = ref<string | null>(null)

// 公告栏折叠状态（默认折叠，节省空间）
const announcementOpen = ref(false)

// 软禁用：恢复账号中的加载状态（按账号 ID 区分）
const reEnableLoading = reactive<Record<number, boolean>>({})

/** 恢复被禁用的账号 */
async function reEnableAccount(r: { id: number; email: string }) {
  if (reEnableLoading[r.id]) return
  reEnableLoading[r.id] = true
  try {
    await resultApi.reEnable(r.id)
    toast(`账号 ${r.email} 已恢复`, 'success')
    await loadResults()
  } catch (e: any) {
    toast(e.response?.data?.detail || '恢复失败', 'error')
  } finally {
    reEnableLoading[r.id] = false
  }
}

/** 相对时间格式化（用于 last_validated_at） */
function formatRelativeTime(dateStr: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const diff = Date.now() - d.getTime()
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return '刚刚'
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}分钟前`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}小时前`
  const day = Math.floor(hr / 24)
  if (day < 7) return `${day}天前`
  return d.toLocaleDateString()
}
</script>
