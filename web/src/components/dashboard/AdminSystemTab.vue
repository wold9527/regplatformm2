<template>
  <!-- 系统设置 Tab -->
  <div v-show="adminTab === 'settings'" class="space-y-4">
    <!-- 顶部工具栏：搜索 + 操作 -->
    <div class="flex items-center gap-3">
      <div class="flex-1 relative">
        <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-t-faint" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
        <input v-model="settingsSearch" type="text" placeholder="搜索设置项..."
          class="w-full bg-s-inset border border-b-panel rounded-lg pl-9 pr-3 py-2 text-xs focus:border-blue-500 outline-none transition">
      </div>
      <span class="text-[10px] text-t-faint flex-none">修改后点击底部保存</span>
      <button @click="loadAdminSettings" class="text-[10px] text-t-muted hover:text-white transition px-2.5 py-1.5 rounded-lg hover:bg-s-hover flex-none">刷新</button>
    </div>

    <div v-if="adminSettingsLoading" class="text-t-muted text-center py-12 text-xs">加载中...</div>
    <div v-else class="space-y-3">
      <!-- 分组卡片 -->
      <div v-for="group in settingsGroups()" :key="group">
        <template v-if="filteredSettingsInGroup(group).length > 0">
          <!-- 分组头部（可折叠） -->
          <button @click="toggleGroup(group)"
            class="w-full flex items-center gap-3 glass-light rounded-xl px-4 py-3 hover:bg-s-hover transition group">
            <span class="text-base flex-none">{{ groupIcon(group) }}</span>
            <div class="flex-1 text-left">
              <div class="text-xs font-bold text-t-primary">{{ groupLabel(group) }}</div>
              <div class="text-[10px] text-t-faint">
                已配置 {{ groupStats(group).configured }}/{{ groupStats(group).total }}
              </div>
            </div>
            <!-- 配置进度条 -->
            <div class="w-20 h-1.5 rounded-full bg-s-panel overflow-hidden flex-none">
              <div class="h-full rounded-full transition-all duration-300"
                :class="groupStats(group).configured === groupStats(group).total ? 'bg-green-500' : 'bg-blue-500'"
                :style="`width:${groupStats(group).total > 0 ? (groupStats(group).configured / groupStats(group).total) * 100 : 0}%`"></div>
            </div>
            <svg class="w-4 h-4 text-t-muted transition-transform flex-none" :class="isGroupCollapsed(group) ? '' : 'rotate-180'"
              fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
          </button>

          <!-- 分组内容 -->
          <div v-show="!isGroupCollapsed(group)" class="mt-1 space-y-1 pl-2">
            <div v-for="s in filteredSettingsInGroup(group)" :key="s.key"
              class="glass-light rounded-lg px-4 py-3 transition hover:bg-s-hover/50">
              <!-- 第一行：标签 + 状态 -->
              <div class="flex items-center gap-2 mb-2">
                <span class="flex-none w-1.5 h-1.5 rounded-full" :class="s.has_value ? 'bg-green-500' : 'bg-gray-600'"></span>
                <span class="text-xs font-medium text-t-primary">{{ s.label }}</span>
                <span v-if="s.read_only" class="text-[9px] px-1.5 py-0.5 rounded-full bg-cyan-900/20 text-cyan-400/70">自动</span>
                <span v-if="s._dirty" class="text-[9px] px-1.5 py-0.5 rounded-full bg-amber-900/30 text-amber-400 font-medium">已修改</span>
                <span v-if="s.is_secret" class="text-[9px] px-1.5 py-0.5 rounded-full bg-red-900/20 text-red-400/70">敏感</span>
              </div>
              <!-- 描述 -->
              <div v-if="s.description" class="text-[10px] text-t-faint mb-2 leading-relaxed">{{ s.description }}</div>
              <!-- 输入区域 -->
              <div class="flex items-center gap-2">
                <!-- boolean 类型：开关 -->
                <template v-if="s.type === 'boolean'">
                  <button @click="!s.read_only && (s._editValue = (s._editValue !== undefined ? s._editValue : s.value) === 'true' ? 'false' : 'true', s._dirty = true)"
                    class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors flex-none"
                    :class="[
                      (s._editValue !== undefined ? s._editValue : s.value) === 'true' ? 'bg-green-500' : 'bg-s-panel',
                      s.read_only ? 'opacity-50 cursor-not-allowed' : ''
                    ]">
                    <span class="inline-block h-4 w-4 rounded-full bg-white transition-transform"
                      :class="(s._editValue !== undefined ? s._editValue : s.value) === 'true' ? 'translate-x-6' : 'translate-x-1'"></span>
                  </button>
                  <span class="text-xs" :class="(s._editValue !== undefined ? s._editValue : s.value) === 'true' ? 'text-ok' : 'text-t-muted'">
                    {{ (s._editValue !== undefined ? s._editValue : s.value) === 'true' ? '已开启' : '已关闭' }}
                  </span>
                </template>
                <!-- 其他类型：文本输入 -->
                <template v-else>
                  <input
                    :type="s.is_secret && !s._showRaw ? 'password' : 'text'"
                    :value="s._editValue !== undefined ? s._editValue : (s._showRaw ? (s._rawValue || '') : s.value)"
                    @input="!s.read_only && (s._editValue = ($event.target as HTMLInputElement).value, s._dirty = true)"
                    :disabled="s.read_only"
                    :placeholder="s.default_value || '未设置'"
                    class="flex-1 bg-s-inset border border-b-panel rounded-lg px-3 py-1.5 text-xs font-mono focus:border-blue-500 outline-none transition"
                    :class="[s._dirty ? 'border-amber-500/60' : '', s.read_only ? 'opacity-50 cursor-not-allowed' : '']">
                  <button v-if="s.is_secret" @click="toggleSettingVisibility(s)"
                    class="text-t-muted hover:text-gray-300 p-1.5 rounded-lg hover:bg-s-hover transition flex-none"
                    :title="s._showRaw ? '隐藏' : '显示'">
                    <svg v-if="!s._showRaw" class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/></svg>
                    <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"/></svg>
                  </button>
                </template>
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- 搜索无结果 -->
      <div v-if="settingsSearch && settingsGroups().every(g => filteredSettingsInGroup(g).length === 0)"
        class="text-t-faint text-center py-8 text-xs">没有匹配的设置项</div>
    </div>

    <!-- 底部浮动保存栏 -->
    <Transition name="slide-up">
      <div v-if="dirtyCount > 0"
        class="sticky bottom-0 z-10 glass-light rounded-xl px-4 py-3 flex items-center justify-between border border-amber-500/30 shadow-lg">
        <span class="text-xs text-amber-400">{{ dirtyCount }} 项设置已修改</span>
        <div class="flex items-center gap-2">
          <button @click="loadAdminSettings" class="text-[10px] text-t-muted hover:text-white px-3 py-1.5 rounded-lg hover:bg-s-hover transition">放弃修改</button>
          <button @click="saveAllDirtySettings"
            class="text-xs px-4 py-1.5 rounded-lg font-bold bg-gradient-to-r from-blue-600 to-cyan-600 hover:from-blue-500 hover:to-cyan-500 text-white transition">
            保存全部
          </button>
        </div>
      </div>
    </Transition>
  </div>

  <!-- 数据清理 Tab -->
  <div v-show="adminTab === 'cleanup'" class="space-y-6">
    <!-- 数据量概览 -->
    <div class="glass-light rounded-xl p-4 space-y-3">
      <div class="text-xs font-bold text-t-secondary uppercase tracking-wider flex items-center gap-2">
        <svg class="w-3.5 h-3.5 text-err" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
        数据量统计
      </div>
      <div v-if="dataStatsLoading" class="text-t-muted text-center py-4 text-xs">加载中...</div>
      <div v-else class="grid grid-cols-5 gap-3">
        <div class="glass-light rounded-lg p-2.5 text-center">
          <div class="text-[10px] text-t-muted">任务</div>
          <div class="text-lg font-bold">{{ dataStats.tasks || 0 }}</div>
        </div>
        <div class="glass-light rounded-lg p-2.5 text-center">
          <div class="text-[10px] text-t-muted">注册结果</div>
          <div class="text-lg font-bold text-ok">{{ dataStats.results || 0 }}</div>
        </div>
        <div class="glass-light rounded-lg p-2.5 text-center">
          <div class="text-[10px] text-t-muted">已归档</div>
          <div class="text-lg font-bold text-warn">{{ dataStats.archived_results || 0 }}</div>
        </div>
        <div class="glass-light rounded-lg p-2.5 text-center">
          <div class="text-[10px] text-t-muted">交易记录</div>
          <div class="text-lg font-bold text-accent">{{ dataStats.transactions || 0 }}</div>
        </div>
        <div class="glass-light rounded-lg p-2.5 text-center">
          <div class="text-[10px] text-t-muted">兑换码</div>
          <div class="text-lg font-bold text-purple-400">{{ dataStats.codes || 0 }}</div>
        </div>
      </div>
      <!-- 时间段分布 -->
      <div v-if="dataStats.older_than_30d" class="grid grid-cols-2 gap-3 mt-2">
        <div class="glass-light rounded-lg p-2.5">
          <div class="text-[10px] text-t-muted mb-1">30天前的数据</div>
          <div class="text-[10px] text-t-secondary">
            注册结果 <span class="font-mono font-bold text-warn">{{ dataStats.older_than_30d?.results || 0 }}</span> 条 ·
            已完成任务 <span class="font-mono font-bold text-warn">{{ dataStats.older_than_30d?.tasks || 0 }}</span> 个
          </div>
        </div>
        <div class="glass-light rounded-lg p-2.5">
          <div class="text-[10px] text-t-muted mb-1">90天前的数据</div>
          <div class="text-[10px] text-t-secondary">
            注册结果 <span class="font-mono font-bold text-err">{{ dataStats.older_than_90d?.results || 0 }}</span> 条 ·
            已完成任务 <span class="font-mono font-bold text-err">{{ dataStats.older_than_90d?.tasks || 0 }}</span> 个
          </div>
        </div>
      </div>
    </div>

    <!-- 清理操作 -->
    <div class="glass-light rounded-xl p-4 space-y-4">
      <div class="text-xs font-bold text-t-secondary uppercase tracking-wider">清理设置</div>
      <div class="space-y-3">
        <div>
          <label class="text-[10px] text-t-muted mb-1 block">清理多少天前的数据</label>
          <div class="flex items-center gap-2">
            <input v-model.number="cleanupForm.days" type="number" min="1" max="3650"
              class="w-32 bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-red-500 focus:outline-none">
            <span class="text-[10px] text-t-muted">天前</span>
            <div class="flex gap-1.5 ml-2">
              <button @click="cleanupForm.days = 30" class="text-[10px] px-2 py-1 rounded" :class="cleanupForm.days === 30 ? 'bg-red-600/60 text-white' : 'glass-light text-t-muted hover:text-gray-300'">30天</button>
              <button @click="cleanupForm.days = 60" class="text-[10px] px-2 py-1 rounded" :class="cleanupForm.days === 60 ? 'bg-red-600/60 text-white' : 'glass-light text-t-muted hover:text-gray-300'">60天</button>
              <button @click="cleanupForm.days = 90" class="text-[10px] px-2 py-1 rounded" :class="cleanupForm.days === 90 ? 'bg-red-600/60 text-white' : 'glass-light text-t-muted hover:text-gray-300'">90天</button>
              <button @click="cleanupForm.days = 180" class="text-[10px] px-2 py-1 rounded" :class="cleanupForm.days === 180 ? 'bg-red-600/60 text-white' : 'glass-light text-t-muted hover:text-gray-300'">180天</button>
            </div>
          </div>
        </div>
        <div class="space-y-2">
          <label class="text-[10px] text-t-muted block">清理范围</label>
          <div class="flex flex-wrap gap-3">
            <label class="flex items-center gap-2 cursor-pointer group">
              <input type="checkbox" v-model="cleanupForm.clean_results"
                class="w-3.5 h-3.5 rounded border-gray-600 bg-gray-800 text-green-500 focus:ring-green-500/30">
              <span class="text-xs text-t-secondary group-hover:text-t-primary transition">注册结果</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer group">
              <input type="checkbox" v-model="cleanupForm.clean_tasks"
                class="w-3.5 h-3.5 rounded border-gray-600 bg-gray-800 text-blue-500 focus:ring-blue-500/30">
              <span class="text-xs text-t-secondary group-hover:text-t-primary transition">已完成任务</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer group">
              <input type="checkbox" v-model="cleanupForm.clean_tx"
                class="w-3.5 h-3.5 rounded border-gray-600 bg-gray-800 text-cyan-500 focus:ring-cyan-500/30">
              <span class="text-xs text-t-secondary group-hover:text-t-primary transition">交易记录</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer group">
              <input type="checkbox" v-model="cleanupForm.clean_archived_only"
                class="w-3.5 h-3.5 rounded border-gray-600 bg-gray-800 text-amber-500 focus:ring-amber-500/30">
              <span class="text-xs text-t-secondary group-hover:text-t-primary transition">仅清理已归档的结果</span>
            </label>
          </div>
        </div>
      </div>
      <div class="flex items-center gap-3 pt-2 border-t border-b-panel">
        <button @click="executeCleanup" :disabled="cleanupProcessing || (!cleanupForm.clean_results && !cleanupForm.clean_tasks && !cleanupForm.clean_tx)"
          class="px-6 py-2 rounded-lg text-xs font-bold bg-gradient-to-r from-red-600 to-red-500 hover:from-red-500 hover:to-red-400 disabled:from-gray-700 disabled:to-gray-700 disabled:text-gray-500 text-white transition">
          {{ cleanupProcessing ? '清理中...' : '执行清理' }}
        </button>
        <span class="text-[10px] text-err">此操作不可撤销，请谨慎操作</span>
      </div>
      <!-- 清理结果 -->
      <div v-if="cleanupResult" class="glass-light rounded-lg p-3 border border-green-500/20 animate-in">
        <div class="text-[10px] text-ok font-bold mb-1">{{ cleanupResult.message }}</div>
        <div class="flex gap-4 text-[10px] text-t-secondary">
          <span v-if="cleanupResult.cleaned?.results !== undefined">注册结果: <span class="font-mono font-bold text-ok">{{ cleanupResult.cleaned.results }}</span> 条</span>
          <span v-if="cleanupResult.cleaned?.tasks !== undefined">任务: <span class="font-mono font-bold text-ok">{{ cleanupResult.cleaned.tasks }}</span> 个</span>
          <span v-if="cleanupResult.cleaned?.transactions !== undefined">交易: <span class="font-mono font-bold text-ok">{{ cleanupResult.cleaned.transactions }}</span> 条</span>
        </div>
      </div>
    </div>
  </div>

  <!-- 实时状态 Tab -->
  <div v-show="adminTab === 'realtime'" class="space-y-6">
    <!-- 顶部指标卡片 -->
    <div class="grid grid-cols-3 gap-3">
      <div class="glass-light rounded-xl p-3">
        <div class="text-[10px] text-t-muted mb-1">运行中任务</div>
        <div class="text-xl font-bold" :class="realtimeTasks.length > 0 ? 'text-accent' : 'text-t-faint'">{{ realtimeTasks.length }}</div>
      </div>
      <div class="glass-light rounded-xl p-3">
        <div class="text-[10px] text-t-muted mb-1">在线用户</div>
        <div class="text-xl font-bold text-info">{{ realtimeOnlineUsers }}</div>
      </div>
      <div class="glass-light rounded-xl p-3">
        <div class="text-[10px] text-t-muted mb-1">自动刷新</div>
        <div class="flex items-center gap-1.5">
          <span class="relative flex h-2 w-2">
            <span class="pulse-ring absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2 w-2 bg-cyan-500"></span>
          </span>
          <span class="text-sm font-bold text-accent">每 3 秒</span>
        </div>
      </div>
    </div>

    <!-- 运行任务表格 -->
    <div class="glass-light rounded-xl overflow-hidden">
      <div class="px-4 py-2.5 flex items-center justify-between border-b border-b-panel">
        <span class="text-xs font-bold text-t-secondary uppercase tracking-wider">运行中任务</span>
        <button @click="loadRealtimeTasks" class="text-[10px] text-t-muted hover:text-white transition px-2 py-1 rounded hover:bg-s-hover">刷新</button>
      </div>
      <table class="w-full text-sm" v-if="realtimeTasks.length > 0">
        <thead class="text-[10px] text-t-muted border-b border-b-panel">
          <tr>
            <th class="px-3 py-2 text-left">用户</th>
            <th class="px-3 py-2 text-left">平台</th>
            <th class="px-3 py-2 text-center">目标</th>
            <th class="px-3 py-2 text-center">成功/失败</th>
            <th class="px-3 py-2 text-center">积分</th>
            <th class="px-3 py-2 text-center">耗时</th>
            <th class="px-3 py-2 text-right">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-b-panel">
          <tr v-for="t in realtimeTasks" :key="t.task_id" class="hover:bg-s-hover transition">
            <td class="px-3 py-2">
              <div class="flex items-center gap-2">
                <img :src="t.avatar_url || undefined" class="w-6 h-6 rounded-full flex-none"
                  @error="(e: any) => e.target.src='data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 44 44%22><rect fill=%22%23334155%22 width=%2244%22 height=%2244%22 rx=%2222%22/><text x=%2250%25%22 y=%2258%25%22 text-anchor=%22middle%22 fill=%22%23fff%22 font-size=%2216%22>?</text></svg>'">
                <div>
                  <div class="text-xs font-medium">{{ t.username }}</div>
                  <div class="text-[10px] text-t-faint font-mono">#{{ t.task_id }}</div>
                </div>
              </div>
            </td>
            <td class="px-3 py-2">
              <span class="text-[10px] px-2 py-0.5 rounded font-bold"
                :class="{ 'bg-blue-900/40 text-info': t.platform === 'grok', 'bg-green-900/40 text-ok': t.platform === 'openai', 'bg-amber-900/40 text-warn': t.platform === 'kiro', 'bg-purple-900/40 text-purple-400': t.platform === 'gemini' }">
                {{ ({ grok: 'Grok', openai: 'OpenAI', kiro: 'Kiro', gemini: 'Gemini' } as any)[t.platform] || t.platform }}
              </span>
            </td>
            <td class="px-3 py-2 text-center font-mono text-xs">{{ t.target }}</td>
            <td class="px-3 py-2 text-center">
              <span class="text-ok font-mono font-bold text-xs">{{ t.success_count }}</span>
              <span class="text-t-faint mx-0.5">/</span>
              <span class="text-err font-mono font-bold text-xs">{{ t.fail_count }}</span>
            </td>
            <td class="px-3 py-2 text-center">
              <span v-if="t.credits_reserved > 0" class="text-warn font-mono text-xs">{{ t.credits_reserved }}</span>
              <span v-else class="text-[10px] px-1.5 py-0.5 rounded bg-amber-900/30 text-warn font-bold">免费</span>
            </td>
            <td class="px-3 py-2 text-center text-xs font-mono text-accent">{{ formatElapsedSec(t.elapsed_sec) }}</td>
            <td class="px-3 py-2 text-right space-x-1">
              <button v-if="!t.stopping" @click="adminStopTask(t.task_id)"
                class="text-[10px] px-2 py-1 rounded bg-red-600/80 hover:bg-red-500 text-white transition">停止</button>
              <span v-else class="text-[10px] text-warn">停止中...</span>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="text-t-faint text-center py-8 text-[10px]">当前没有运行中的任务</div>
    </div>

    <!-- 最近注册活动 -->
    <div class="glass-light rounded-xl overflow-hidden">
      <div class="px-4 py-2.5 flex items-center justify-between border-b border-b-panel">
        <span class="text-xs font-bold text-t-secondary uppercase tracking-wider">最近注册活动</span>
        <button @click="loadRecentActivity" class="text-[10px] text-t-muted hover:text-white transition px-2 py-1 rounded hover:bg-s-hover">刷新</button>
      </div>
      <div v-if="recentActivityLoading && recentActivity.length === 0" class="text-t-muted text-center py-8 text-xs">加载中...</div>
      <table class="w-full text-sm" v-else-if="recentActivity.length > 0">
        <thead class="text-[10px] text-t-muted border-b border-b-panel">
          <tr>
            <th class="px-3 py-2 text-left">用户</th>
            <th class="px-3 py-2 text-left">平台</th>
            <th class="px-3 py-2 text-center">注册量</th>
            <th class="px-3 py-2 text-center">消费</th>
            <th class="px-3 py-2 text-center">状态</th>
            <th class="px-3 py-2 text-right">时间</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-b-panel">
          <tr v-for="a in recentActivity" :key="a.task_id" class="hover:bg-s-hover transition">
            <td class="px-3 py-2">
              <div class="flex items-center gap-2">
                <img :src="a.avatar_url || undefined" class="w-5 h-5 rounded-full flex-none"
                  @error="(e: any) => e.target.src='data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 44 44%22><rect fill=%22%23334155%22 width=%2244%22 height=%2244%22 rx=%2222%22/><text x=%2250%25%22 y=%2258%25%22 text-anchor=%22middle%22 fill=%22%23fff%22 font-size=%2216%22>?</text></svg>'">
                <span class="text-xs">{{ a.username }}</span>
              </div>
            </td>
            <td class="px-3 py-2">
              <span class="text-[10px] px-2 py-0.5 rounded font-bold"
                :class="{ 'bg-blue-900/40 text-info': a.platform === 'grok', 'bg-green-900/40 text-ok': a.platform === 'openai', 'bg-amber-900/40 text-warn': a.platform === 'kiro', 'bg-purple-900/40 text-purple-400': a.platform === 'gemini' }">
                {{ ({ grok: 'Grok', openai: 'OpenAI', kiro: 'Kiro', gemini: 'Gemini' } as any)[a.platform] || a.platform }}
              </span>
            </td>
            <td class="px-3 py-2 text-center">
              <span class="text-ok font-mono font-bold text-xs">{{ a.success_count }}</span>
              <span class="text-t-faint text-[10px]"> / {{ a.target }}</span>
              <span v-if="a.fail_count > 0" class="text-err text-[10px] ml-1">({{ a.fail_count }} 失败)</span>
            </td>
            <td class="px-3 py-2 text-center">
              <span v-if="a.credits_reserved > 0" class="text-warn font-mono text-xs">{{ a.credits_reserved }}</span>
              <span v-else class="text-[10px] px-1.5 py-0.5 rounded bg-amber-900/30 text-warn font-bold">免费</span>
            </td>
            <td class="px-3 py-2 text-center">
              <span class="text-[10px] px-2 py-0.5 rounded font-medium"
                :class="{
                  'bg-green-900/30 text-ok': a.status === 'completed',
                  'bg-cyan-900/30 text-accent': a.status === 'running' || a.status === 'stopping',
                  'bg-gray-700/30 text-t-muted': a.status === 'stopped',
                  'bg-red-900/30 text-err': a.status === 'failed',
                }">
                {{ ({ completed: '已完成', running: '运行中', stopping: '停止中', stopped: '已停止', failed: '失败' } as any)[a.status] || a.status }}
              </span>
            </td>
            <td class="px-3 py-2 text-right text-[10px] text-t-muted font-mono">
              {{ a.completed_at || a.created_at }}
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="text-t-faint text-center py-8 text-[10px]">暂无注册活动</div>

      <!-- 分页 -->
      <div v-if="recentActivityTotalPages > 1" class="px-4 py-2.5 flex items-center justify-between border-t border-b-panel">
        <span class="text-[10px] text-t-faint">共 {{ recentActivityTotal }} 条</span>
        <div class="flex items-center gap-1">
          <button @click="recentActivityGoPage(recentActivityPage - 1)" :disabled="recentActivityPage <= 1"
            class="text-[10px] px-2 py-1 rounded hover:bg-s-hover disabled:opacity-30 disabled:cursor-not-allowed text-t-muted transition">上一页</button>
          <span class="text-[10px] text-t-secondary font-mono px-2">{{ recentActivityPage }} / {{ recentActivityTotalPages }}</span>
          <button @click="recentActivityGoPage(recentActivityPage + 1)" :disabled="recentActivityPage >= recentActivityTotalPages"
            class="text-[10px] px-2 py-1 rounded hover:bg-s-hover disabled:opacity-30 disabled:cursor-not-allowed text-t-muted transition">下一页</button>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import { useAdmin } from '../../composables/useAdmin'

const {
  adminTab,
  // 设置
  adminSettingsLoading, settingsSearch, dirtyCount,
  loadAdminSettings, settingsGroups, groupLabel, groupIcon,
  filteredSettingsInGroup, groupStats, toggleGroup, isGroupCollapsed,
  toggleSettingVisibility, saveAllDirtySettings,
  // 清理
  dataStats, dataStatsLoading, cleanupForm, cleanupProcessing, cleanupResult,
  loadDataStats, executeCleanup,
  // 实时
  realtimeTasks, realtimeOnlineUsers,
  loadRealtimeTasks, adminStopTask, formatElapsedSec,
  // 最近活动
  recentActivity, recentActivityTotal, recentActivityPage,
  recentActivityTotalPages, recentActivityLoading,
  loadRecentActivity, recentActivityGoPage,
} = useAdmin()
</script>
