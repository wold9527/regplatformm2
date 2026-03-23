<template>
  <div v-show="adminTab === 'proxypool'" class="space-y-4">

    <!-- 统计卡片 -->
    <div class="grid grid-cols-4 gap-3 mobile-grid-2">
      <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
        <div class="flex items-center justify-between">
          <span class="text-[10px] text-t-muted font-medium tracking-wide">总代理</span>
          <span class="w-5 h-5 rounded-md bg-blue-500/15 flex items-center justify-center">
            <svg class="w-3 h-3 text-info" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/>
            </svg>
          </span>
        </div>
        <div class="text-2xl font-bold tabular-nums leading-none">{{ proxyStats.total }}</div>
        <div class="text-[10px] text-t-faint">代理池总量</div>
      </div>

      <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
        <div class="flex items-center justify-between">
          <span class="text-[10px] text-t-muted font-medium tracking-wide">健康</span>
          <span class="w-5 h-5 rounded-md bg-green-500/15 flex items-center justify-center">
            <svg class="w-3 h-3 text-ok" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="20 6 9 17 4 12"/>
            </svg>
          </span>
        </div>
        <div class="text-2xl font-bold tabular-nums text-ok leading-none">{{ proxyStats.healthy }}</div>
        <div class="text-[10px] text-t-faint">
          占比 <span class="text-ok font-medium">
            {{ proxyStats.total > 0 ? ((proxyStats.healthy / proxyStats.total) * 100).toFixed(0) : 0 }}%
          </span>
        </div>
      </div>

      <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
        <div class="flex items-center justify-between">
          <span class="text-[10px] text-t-muted font-medium tracking-wide">不健康</span>
          <span class="w-5 h-5 rounded-md bg-red-500/15 flex items-center justify-center">
            <svg class="w-3 h-3 text-err" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </span>
        </div>
        <div class="text-2xl font-bold tabular-nums text-err leading-none">{{ proxyStats.unhealthy }}</div>
        <div class="text-[10px] text-t-faint">可清除</div>
      </div>

      <div class="glass-light rounded-xl p-3 flex flex-col gap-1 overview-card">
        <div class="flex items-center justify-between">
          <span class="text-[10px] text-t-muted font-medium tracking-wide">活跃中</span>
          <span class="w-5 h-5 rounded-md flex items-center justify-center"
            :class="proxyStats.active > 0 ? 'bg-cyan-500/15' : 'bg-s-hover'">
            <span v-if="proxyStats.active > 0" class="w-2 h-2 rounded-full bg-accent overview-pulse"></span>
            <span v-else class="w-2 h-2 rounded-full bg-t-faint/40"></span>
          </span>
        </div>
        <div class="text-2xl font-bold tabular-nums leading-none"
          :class="proxyStats.active > 0 ? 'text-accent' : 'text-t-faint'">
          {{ proxyStats.active }}
        </div>
        <div class="text-[10px] text-t-faint">正在使用中</div>
      </div>
    </div>

    <!-- 平台代理策略 -->
    <div class="glass-light rounded-xl overflow-hidden">
      <div class="px-3 py-2 border-b border-b-panel flex items-center justify-between cursor-pointer select-none hover:bg-s-hover/50 transition-colors"
        @click="proxyStrategyCollapsed = !proxyStrategyCollapsed">
        <div class="flex items-center gap-1.5">
          <svg class="w-3.5 h-3.5 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"/>
          </svg>
          <span class="text-xs font-bold text-t-primary">平台代理策略</span>
          <span class="text-[10px] text-t-faint">每个平台独立配置代理模式</span>
          <svg class="w-3 h-3 text-t-faint transition-transform duration-200" :class="proxyStrategyCollapsed ? '' : 'rotate-180'" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
          </svg>
        </div>
        <button v-if="Object.values(platformProxyConfigs).some(c => c.dirty)"
          @click.stop="saveAllProxyConfigs()"
          class="text-[10px] px-2 py-0.5 rounded-md font-bold bg-accent/20 text-accent hover:bg-accent/30 transition">
          保存全部
        </button>
      </div>

      <div v-if="!proxyStrategyCollapsed">
        <!-- 加载中 -->
        <div v-if="proxyConfigLoading" class="py-4 text-center">
          <span class="text-[11px] text-t-faint">加载中...</span>
        </div>

        <!-- 平台卡片列表 -->
        <div v-else class="divide-y divide-b-panel">
          <div v-for="p in PLATFORMS" :key="p"
            class="px-3 py-2.5 hover:bg-s-hover/30 transition-colors">
            <div class="flex items-center gap-3">
              <!-- 平台名称 -->
              <div class="w-16 flex-none">
                <span class="text-[11px] font-bold" :class="PLATFORM_COLORS[p]">{{ PLATFORM_LABELS[p] }}</span>
              </div>

              <!-- 模式选择按钮组 -->
              <div class="flex bg-s-inset rounded-lg p-0.5 gap-0.5 flex-none">
                <button v-for="m in PROXY_MODES" :key="m.value"
                  @click="setProxyMode(p, m.value)"
                  class="text-[10px] px-2 py-1 rounded-md transition font-medium relative group/btn"
                  :class="platformProxyConfigs[p].mode === m.value
                    ? `${m.color} text-white shadow`
                    : 'text-t-muted hover:text-t-primary'">
                  {{ m.label }}
                  <!-- tooltip -->
                  <span class="absolute -top-7 left-1/2 -translate-x-1/2 px-1.5 py-0.5 rounded text-[9px] bg-black/80 text-white whitespace-nowrap opacity-0 group-hover/btn:opacity-100 transition pointer-events-none">
                    {{ m.desc }}
                  </span>
                </button>
              </div>

              <!-- 固定代理输入框（仅 fixed 模式显示） -->
              <div v-if="platformProxyConfigs[p].mode === 'fixed'" class="flex-1 min-w-0">
                <input :value="platformProxyConfigs[p].fixedProxy"
                  @input="setFixedProxy(p, ($event.target as HTMLInputElement).value)"
                  type="text"
                  :placeholder="`${PLATFORM_LABELS[p]} 专用代理 (http/socks5://user:pass@host:port)`"
                  class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1 text-[11px] font-mono outline-none transition placeholder:text-t-faint focus:border-purple-500/60">
              </div>

              <!-- 当前模式描述（非 fixed 模式） -->
              <div v-else class="flex-1 min-w-0">
                <span class="text-[10px] text-t-faint">
                  {{ platformProxyConfigs[p].mode === 'pool' ? '使用代理池轮询分配'
                    : platformProxyConfigs[p].mode === 'direct' ? '不使用代理，直接连接'
                    : platformProxyConfigs[p].mode === 'smart' ? '根据延迟智能选择最优代理'
                    : '' }}
                </span>
              </div>

              <!-- 保存按钮 -->
              <button v-if="platformProxyConfigs[p].dirty"
                @click="saveProxyConfig(p)"
                :disabled="platformProxyConfigs[p].saving"
                class="text-[10px] px-2.5 py-1 rounded-lg font-bold transition flex-none flex items-center gap-1"
                :class="platformProxyConfigs[p].saving
                  ? 'bg-gray-700 text-gray-500'
                  : 'bg-purple-600/80 hover:bg-purple-500/80 text-white'">
                <svg v-if="platformProxyConfigs[p].saving" class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                </svg>
                {{ platformProxyConfigs[p].saving ? '保存中' : '保存' }}
              </button>
              <!-- 已保存标记 -->
              <span v-else-if="!proxyConfigLoading" class="text-[10px] text-t-faint flex-none">
                <svg class="w-3 h-3 text-ok inline-block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <polyline points="20 6 9 17 4 12" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 操作栏 -->
    <div class="flex items-center gap-2 flex-wrap">
      <!-- 过滤器 -->
      <div class="flex bg-s-inset rounded-lg p-0.5 gap-0.5">
        <button @click="currentPage = 1; filterMode = ''; loadProxies()"
          class="text-[11px] px-3 py-1 rounded-md transition font-medium"
          :class="filterMode === '' ? 'bg-blue-600/80 text-white shadow' : 'text-t-muted hover:text-t-primary'">
          全部
        </button>
        <button @click="currentPage = 1; filterMode = 'healthy'; loadProxies()"
          class="text-[11px] px-3 py-1 rounded-md transition font-medium"
          :class="filterMode === 'healthy' ? 'bg-green-600/80 text-white shadow' : 'text-t-muted hover:text-t-primary'">
          健康
        </button>
        <button @click="currentPage = 1; filterMode = 'unhealthy'; loadProxies()"
          class="text-[11px] px-3 py-1 rounded-md transition font-medium"
          :class="filterMode === 'unhealthy' ? 'bg-red-600/80 text-white shadow' : 'text-t-muted hover:text-t-primary'">
          不健康
        </button>
      </div>

      <div class="flex-1"></div>

      <!-- 批量删除（有选中时显示） -->
      <button v-if="selectedIds.size > 0" @click="batchDelete"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-red-600/80 hover:bg-red-500/80 text-white transition flex items-center gap-1.5">
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
        </svg>
        删除选中 ({{ selectedIds.size }})
      </button>

      <!-- 添加代理 -->
      <button @click="addDialogOpen = true"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-blue-600/80 hover:bg-blue-500/80 text-white transition flex items-center gap-1.5">
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
        </svg>
        添加代理
      </button>

      <!-- 批量导入 -->
      <button @click="importDialogOpen = true"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-amber-600/80 hover:bg-amber-500/80 text-white transition flex items-center gap-1.5">
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"/>
        </svg>
        批量导入
      </button>

      <!-- 从 URL 抓取 -->
      <button @click="fetchUrlDialogOpen = true"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-emerald-600/80 hover:bg-emerald-500/80 text-white transition flex items-center gap-1.5">
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"/>
        </svg>
        从 URL 抓取
      </button>

      <!-- 健康检查 -->
      <button @click="triggerHealthCheck" :disabled="healthCheckRunning"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-cyan-600/80 hover:bg-cyan-500/80 disabled:bg-gray-700 disabled:text-gray-500 text-white transition flex items-center gap-1.5">
        <svg v-if="healthCheckRunning" class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
        </svg>
        <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/>
        </svg>
        {{ healthCheckRunning ? '检查中...' : selectedIds.size > 0 ? `检查选中 (${selectedIds.size})` : '检查本页' }}
      </button>

      <!-- 清除不健康 -->
      <button @click="purgeUnhealthy" :disabled="purgeRunning || proxyStats.unhealthy === 0"
        class="px-3 py-1.5 rounded-lg text-xs font-bold bg-red-600/60 hover:bg-red-500/80 disabled:bg-gray-700 disabled:text-gray-500 text-white transition flex items-center gap-1.5">
        <svg v-if="purgeRunning" class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
        </svg>
        <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
        </svg>
        {{ purgeRunning ? '清除中...' : `清除不健康 (${proxyStats.unhealthy})` }}
      </button>

      <!-- 刷新 -->
      <button @click="loadAll" :disabled="loading"
        class="px-2 py-1.5 rounded-lg text-xs text-t-muted hover:text-t-secondary bg-s-inset hover:bg-s-hover transition flex items-center gap-1">
        <svg class="w-3.5 h-3.5" :class="loading ? 'animate-spin' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
        </svg>
        刷新
      </button>
    </div>

    <!-- 代理列表表格 -->
    <div class="glass-light rounded-xl overflow-hidden">
      <!-- 表头 -->
      <div class="grid gap-2 px-3 py-2 border-b border-b-panel text-[10px] text-t-faint uppercase tracking-wider font-semibold"
        style="grid-template-columns: 1.5rem 1fr 2.5fr 3.5fr 1.5rem 4rem 5rem 5rem 6rem 5.5rem">
        <!-- 全选复选框 -->
        <div class="flex items-center justify-center">
          <button @click="toggleSelectAll"
            class="w-3.5 h-3.5 rounded border flex items-center justify-center transition"
            :class="selectedIds.size > 0 && selectedIds.size === proxies.length
              ? 'bg-blue-600 border-blue-500'
              : selectedIds.size > 0
                ? 'bg-blue-600/50 border-blue-500'
                : 'border-b-panel hover:border-blue-500/60'">
            <svg v-if="selectedIds.size > 0" class="w-2.5 h-2.5 text-white" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
            </svg>
          </button>
        </div>
        <div>协议</div>
        <div>名称</div>
        <div>地址</div>
        <div>端口</div>
        <div>国家</div>
        <div>状态</div>
        <div>延迟</div>
        <div>最后检查</div>
        <div>操作</div>
      </div>

      <!-- 空状态 -->
      <div v-if="loading" class="py-12 text-center">
        <div class="flex flex-col items-center gap-2">
          <svg class="w-5 h-5 text-t-faint animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
          </svg>
          <span class="text-[11px] text-t-faint">加载中...</span>
        </div>
      </div>

      <div v-else-if="proxies.length === 0" class="py-12 text-center">
        <div class="flex flex-col items-center gap-3">
          <div class="w-10 h-10 rounded-full bg-s-hover flex items-center justify-center">
            <svg class="w-5 h-5 text-t-faint" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/>
            </svg>
          </div>
          <span class="text-[11px] text-t-faint">暂无代理，请添加或导入</span>
        </div>
      </div>

      <!-- 代理行 -->
      <div v-for="proxy in proxies" :key="proxy.id"
        class="grid gap-2 px-3 py-2 border-b border-b-panel last:border-b-0 hover:bg-s-hover transition-colors duration-100 items-center group"
        style="grid-template-columns: 1.5rem 1fr 2.5fr 3.5fr 1.5rem 4rem 5rem 5rem 6rem 5.5rem"
        :class="selectedIds.has(proxy.id) ? 'bg-blue-900/10' : ''">

        <!-- 选择框 -->
        <div class="flex items-center justify-center">
          <button @click="toggleSelect(proxy.id)"
            class="w-3.5 h-3.5 rounded border flex items-center justify-center transition"
            :class="selectedIds.has(proxy.id)
              ? 'bg-blue-600 border-blue-500'
              : 'border-b-panel hover:border-blue-500/60'">
            <svg v-if="selectedIds.has(proxy.id)" class="w-2.5 h-2.5 text-white" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
            </svg>
          </button>
        </div>

        <!-- 协议标签 -->
        <div>
          <span class="text-[9px] font-bold px-1.5 py-0.5 rounded uppercase tracking-wider"
            :class="proxy.protocol === 'socks5' ? 'bg-purple-500/15 text-purple-400'
              : proxy.protocol === 'http' ? 'bg-blue-500/15 text-info'
              : proxy.protocol === 'https' ? 'bg-teal-500/15 text-teal-400'
              : 'bg-s-hover text-t-secondary'">
            {{ proxy.protocol }}
          </span>
        </div>

        <!-- 名称 -->
        <div class="text-[11px] text-t-secondary truncate" :title="proxy.name">{{ proxy.name || '-' }}</div>

        <!-- 地址（含用户名/密码脱敏） -->
        <div class="flex flex-col gap-0.5 min-w-0">
          <div class="text-[11px] font-mono text-t-primary truncate">{{ proxy.host }}:{{ proxy.port }}</div>
          <div v-if="proxy.username" class="flex items-center gap-1 text-[10px] text-t-faint font-mono">
            <span class="truncate max-w-[80px]">{{ proxy.username }}</span>
            <span>:</span>
            <span v-if="isPasswordVisible(proxy.id)" class="truncate max-w-[80px]">{{ proxy.password || '' }}</span>
            <span v-else class="tracking-wider">••••</span>
            <button @click="togglePasswordVisible(proxy.id)"
              class="ml-0.5 text-t-faint hover:text-t-secondary transition opacity-0 group-hover:opacity-100 flex-none">
              <svg class="w-2.5 h-2.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path v-if="isPasswordVisible(proxy.id)" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"/>
                <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/>
              </svg>
            </button>
          </div>
        </div>

        <!-- 端口（仅移动端降级用，桌面已在地址列显示） -->
        <div class="text-[10px] text-t-faint font-mono">{{ proxy.port }}</div>

        <!-- 国家 -->
        <div class="text-[11px] text-t-secondary truncate">{{ proxy.country || '-' }}</div>

        <!-- 状态 -->
        <div>
          <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full flex items-center gap-1 w-fit"
            :class="proxy.is_healthy
              ? 'bg-green-500/15 text-ok'
              : 'bg-red-500/15 text-err'">
            <span class="w-1 h-1 rounded-full flex-none"
              :class="proxy.is_healthy ? 'bg-ok' : 'bg-err'"></span>
            {{ proxy.is_healthy ? '健康' : '不健康' }}
          </span>
          <div v-if="proxy.fail_count > 0" class="text-[9px] text-t-faint mt-0.5">失败 {{ proxy.fail_count }} 次</div>
        </div>

        <!-- 延迟 -->
        <div class="text-[11px] font-mono"
          :class="!proxy.latency_ms || proxy.latency_ms <= 0 ? 'text-t-faint'
            : proxy.latency_ms < 500 ? 'text-ok'
            : proxy.latency_ms < 1500 ? 'text-warn'
            : 'text-err'">
          {{ formatLatency(proxy.latency_ms) }}
        </div>

        <!-- 最后检查 -->
        <div class="text-[10px] text-t-faint">{{ formatLastChecked(proxy.last_checked_at) }}</div>

        <!-- 操作按钮 -->
        <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 max-sm:opacity-100 transition-opacity">
          <!-- 重置健康状态 -->
          <button @click="resetHealth(proxy.id)"
            class="p-1 rounded hover:bg-cyan-900/30 text-t-muted hover:text-cyan-400 transition tip tip-left"
            data-tip="重置健康状态（清除失败记录）">
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
            </svg>
          </button>
          <!-- 删除 -->
          <button @click="deleteProxy(proxy.id)"
            class="p-1 rounded hover:bg-red-900/30 text-t-muted hover:text-err transition tip tip-left"
            data-tip="删除此代理">
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
            </svg>
          </button>
        </div>
      </div>

      <!-- 分页栏 -->
      <div v-if="totalCount > 0" class="px-3 py-2 border-t border-b-panel flex items-center justify-between">
        <span class="text-[10px] text-t-faint tabular-nums">
          共 {{ totalCount }} 条
        </span>
        <div class="flex items-center gap-2">
          <button @click="goToPage(currentPage - 1)" :disabled="currentPage <= 1"
            class="px-2.5 py-1 rounded-md text-[11px] font-medium transition"
            :class="currentPage > 1 ? 'text-t-secondary hover:bg-s-hover' : 'text-t-faint opacity-40'">
            &larr; 上一页
          </button>
          <span class="text-[11px] text-t-secondary tabular-nums">{{ currentPage }} / {{ totalPages }}</span>
          <button @click="goToPage(currentPage + 1)" :disabled="currentPage >= totalPages"
            class="px-2.5 py-1 rounded-md text-[11px] font-medium transition"
            :class="currentPage < totalPages ? 'text-t-secondary hover:bg-s-hover' : 'text-t-faint opacity-40'">
            下一页 &rarr;
          </button>
        </div>
      </div>
    </div>

    <!-- 添加代理弹窗 -->
    <Teleport to="body">
      <Transition name="queue-modal">
        <div v-if="addDialogOpen"
          class="fixed inset-0 z-[9999] flex items-center justify-center"
          role="dialog" aria-modal="true" aria-label="添加代理" tabindex="-1"
          @click.self="addDialogOpen = false"
          @keydown.escape="addDialogOpen = false">
          <div class="absolute inset-0 bg-black/60 backdrop-blur-sm"></div>
          <div class="relative w-96 rounded-2xl overflow-hidden shadow-2xl"
            style="background:var(--bg-admin);border:1px solid var(--border-glass)">
            <div class="px-5 py-4 border-b border-b-panel flex items-center justify-between">
              <span class="text-sm font-bold text-t-primary">添加代理</span>
              <button @click="addDialogOpen = false" class="text-t-muted hover:text-white transition p-1">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="p-5 space-y-3">
              <!-- 协议选择 -->
              <div class="space-y-1">
                <label class="text-[10px] text-t-faint uppercase tracking-wider">协议</label>
                <div class="flex gap-1.5">
                  <button v-for="p in ['socks5', 'http', 'https']" :key="p"
                    @click="addForm.protocol = p"
                    class="px-3 py-1.5 rounded-lg text-xs font-bold transition"
                    :class="addForm.protocol === p
                      ? 'bg-blue-600/80 text-white'
                      : 'bg-s-inset text-t-muted hover:text-t-primary hover:bg-s-hover'">
                    {{ p.toUpperCase() }}
                  </button>
                </div>
              </div>

              <!-- Host / Port -->
              <div class="grid grid-cols-3 gap-2">
                <div class="col-span-2 space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">主机地址</label>
                  <input v-model="addForm.host" type="text" placeholder="127.0.0.1 或域名"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">端口</label>
                  <input v-model.number="addForm.port" type="number" placeholder="1080"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
              </div>

              <!-- 用户名 / 密码 -->
              <div class="grid grid-cols-2 gap-2">
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">用户名（可选）</label>
                  <input v-model="addForm.username" type="text" placeholder="留空不验证"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">密码（可选）</label>
                  <input v-model="addForm.password" type="password" placeholder="留空不验证"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
              </div>

              <!-- 国家 / 名称 -->
              <div class="grid grid-cols-2 gap-2">
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">国家（可选）</label>
                  <input v-model="addForm.country" type="text" placeholder="US / CN / JP ..."
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
                <div class="space-y-1">
                  <label class="text-[10px] text-t-faint uppercase tracking-wider">别名（可选）</label>
                  <input v-model="addForm.name" type="text" placeholder="自定义名称"
                    class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs outline-none transition placeholder:text-t-faint focus:border-blue-500/60">
                </div>
              </div>

              <div class="flex gap-2 pt-1">
                <button @click="addDialogOpen = false"
                  class="flex-1 py-2 rounded-xl text-xs font-medium text-t-secondary glass-light hover:bg-s-hover transition">
                  取消
                </button>
                <button @click="addProxy" :disabled="!addForm.host || !addForm.port || addLoading"
                  class="flex-1 py-2 rounded-xl text-xs font-bold text-white transition disabled:bg-gray-700 disabled:text-gray-500"
                  style="background: linear-gradient(135deg, #2563eb, #1d4ed8);">
                  {{ addLoading ? '添加中...' : '添加代理' }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- 批量导入弹窗 -->
    <Teleport to="body">
      <Transition name="queue-modal">
        <div v-if="importDialogOpen"
          class="fixed inset-0 z-[9999] flex items-center justify-center"
          role="dialog" aria-modal="true" aria-label="批量导入代理" tabindex="-1"
          @click.self="importDialogOpen = false"
          @keydown.escape="importDialogOpen = false">
          <div class="absolute inset-0 bg-black/60 backdrop-blur-sm"></div>
          <div class="relative w-[30rem] rounded-2xl overflow-hidden shadow-2xl"
            style="background:var(--bg-admin);border:1px solid var(--border-glass)">
            <div class="px-5 py-4 border-b border-b-panel flex items-center justify-between">
              <span class="text-sm font-bold text-t-primary">批量导入代理</span>
              <button @click="importDialogOpen = false" class="text-t-muted hover:text-white transition p-1">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="p-5 space-y-3">
              <!-- 协议 -->
              <div class="space-y-1">
                <label class="text-[10px] text-t-faint uppercase tracking-wider">默认协议</label>
                <div class="flex gap-1.5">
                  <button v-for="p in ['socks5', 'http', 'https']" :key="p"
                    @click="importProtocol = p"
                    class="px-3 py-1.5 rounded-lg text-xs font-bold transition"
                    :class="importProtocol === p
                      ? 'bg-amber-600/80 text-white'
                      : 'bg-s-inset text-t-muted hover:text-t-primary hover:bg-s-hover'">
                    {{ p.toUpperCase() }}
                  </button>
                </div>
              </div>

              <!-- 多行文本 -->
              <div class="space-y-1">
                <label class="text-[10px] text-t-faint uppercase tracking-wider">代理列表（每行一个）</label>
                <div class="text-[10px] text-t-faint mb-1.5 leading-relaxed">
                  格式：<code class="text-accent bg-s-inset px-1 rounded">host:port</code>
                  或 <code class="text-accent bg-s-inset px-1 rounded">user:pass@host:port</code>
                  或完整 URL <code class="text-accent bg-s-inset px-1 rounded">socks5://user:pass@host:port</code>
                </div>
                <textarea v-model="importText" rows="10"
                  placeholder="192.168.1.1:1080&#10;user:pass@192.168.1.2:1080&#10;socks5://user:pass@host:port"
                  class="w-full bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-xs font-mono outline-none transition placeholder:text-t-faint focus:border-amber-500/60 resize-none scroll-thin"></textarea>
                <div class="text-[10px] text-t-faint">
                  共 {{ importText.split('\n').filter(l => l.trim()).length }} 行
                </div>
              </div>

              <div class="flex gap-2 pt-1">
                <button @click="importDialogOpen = false"
                  class="flex-1 py-2 rounded-xl text-xs font-medium text-t-secondary glass-light hover:bg-s-hover transition">
                  取消
                </button>
                <button @click="importProxies" :disabled="!importText.trim() || importLoading"
                  class="flex-1 py-2 rounded-xl text-xs font-bold text-white transition disabled:bg-gray-700 disabled:text-gray-500"
                  style="background: linear-gradient(135deg, #d97706, #b45309);">
                  {{ importLoading ? '导入中...' : '开始导入' }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- 从 URL 抓取弹窗 -->
    <Teleport to="body">
      <Transition name="queue-modal">
        <div v-if="fetchUrlDialogOpen"
          class="fixed inset-0 z-[9999] flex items-center justify-center"
          role="dialog" aria-modal="true" aria-label="从 URL 抓取代理" tabindex="-1"
          @click.self="fetchUrlDialogOpen = false"
          @keydown.escape="fetchUrlDialogOpen = false">
          <div class="absolute inset-0 bg-black/60 backdrop-blur-sm"></div>
          <div class="relative w-[30rem] rounded-2xl overflow-hidden shadow-2xl"
            style="background:var(--bg-admin);border:1px solid var(--border-glass)">
            <div class="px-5 py-4 border-b border-b-panel flex items-center justify-between">
              <span class="text-sm font-bold text-t-primary">从 URL 抓取代理</span>
              <button @click="fetchUrlDialogOpen = false" class="text-t-muted hover:text-white transition p-1">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="p-5 space-y-3">
              <!-- URL 输入 -->
              <div class="space-y-1">
                <label class="text-[10px] text-t-faint uppercase tracking-wider">代理列表 URL *</label>
                <input v-model="fetchUrlForm.url" type="url"
                  placeholder="https://raw.githubusercontent.com/proxifly/free-proxy-list/main/proxies/all/data.txt"
                  class="w-full bg-s-inset border border-b-panel rounded-lg px-2.5 py-1.5 text-xs font-mono outline-none transition placeholder:text-t-faint focus:border-emerald-500/60">
                <div class="text-[10px] text-t-faint leading-relaxed">
                  支持纯文本格式（每行一个代理，格式 <code class="text-accent bg-s-inset px-1 rounded">protocol://host:port</code> 或 <code class="text-accent bg-s-inset px-1 rounded">host:port</code>）
                </div>
              </div>

              <!-- 默认协议 -->
              <div class="space-y-1">
                <label class="text-[10px] text-t-faint uppercase tracking-wider">默认协议（无前缀行使用）</label>
                <div class="flex gap-1.5">
                  <button v-for="p in ['socks5', 'http', 'https']" :key="p"
                    @click="fetchUrlForm.protocol = p"
                    class="px-3 py-1.5 rounded-lg text-xs font-bold transition"
                    :class="fetchUrlForm.protocol === p
                      ? 'bg-emerald-600/80 text-white'
                      : 'bg-s-inset text-t-muted hover:text-t-primary hover:bg-s-hover'">
                    {{ p.toUpperCase() }}
                  </button>
                </div>
              </div>

              <div class="flex gap-2 pt-1">
                <button @click="fetchUrlDialogOpen = false"
                  class="flex-1 py-2 rounded-xl text-xs font-medium text-t-secondary glass-light hover:bg-s-hover transition">
                  取消
                </button>
                <button @click="fetchFromURL" :disabled="!fetchUrlForm.url.trim() || fetchUrlLoading"
                  class="flex-1 py-2 rounded-xl text-xs font-bold text-white transition disabled:bg-gray-700 disabled:text-gray-500"
                  style="background: linear-gradient(135deg, #059669, #047857);">
                  {{ fetchUrlLoading ? '抓取中...' : '开始抓取' }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAdmin } from '../../composables/useAdmin'
import { useProxyPool } from '../../composables/useProxyPool'

const { adminTab } = useAdmin()

const {
  proxies, proxyStats, loading,
  healthCheckRunning, purgeRunning, filterMode, selectedIds,
  addForm, addDialogOpen, addLoading,
  importDialogOpen, importText, importProtocol, importLoading,
  // URL 抓取
  fetchUrlDialogOpen, fetchUrlLoading, fetchUrlForm,
  // 分页
  currentPage, totalCount, totalPages,
  // 平台代理策略
  platformProxyConfigs, proxyConfigLoading,
  PLATFORMS, PLATFORM_LABELS,
  loadProxyConfig, saveProxyConfig, saveAllProxyConfigs,
  // 方法
  loadProxies, loadAll,
  addProxy, deleteProxy, batchDelete,
  importProxies, fetchFromURL,
  triggerHealthCheck, purgeUnhealthy, resetHealth,
  toggleSelect, toggleSelectAll,
  togglePasswordVisible, isPasswordVisible,
  formatLatency, formatLastChecked,
  goToPage,
} = useProxyPool()

// 平台代理策略折叠状态
const proxyStrategyCollapsed = ref(false)

// 代理模式选项
const PROXY_MODES = [
  { value: 'pool', label: '代理池', desc: '轮询分配', color: 'bg-blue-600/80' },
  { value: 'fixed', label: '固定代理', desc: '指定代理', color: 'bg-amber-600/80' },
  { value: 'direct', label: '直连', desc: '不用代理', color: 'bg-green-600/80' },
  { value: 'smart', label: '智能', desc: '按延迟选', color: 'bg-purple-600/80' },
] as const

// 平台图标颜色
const PLATFORM_COLORS: Record<string, string> = {
  grok: 'text-orange-400',
  openai: 'text-green-400',
  kiro: 'text-cyan-400',
  gemini: 'text-blue-400',
}

function setProxyMode(platform: string, mode: string) {
  const cfg = platformProxyConfigs[platform as keyof typeof platformProxyConfigs]
  if (cfg) { cfg.mode = mode as any; cfg.dirty = true }
}
function setFixedProxy(platform: string, value: string) {
  const cfg = platformProxyConfigs[platform as keyof typeof platformProxyConfigs]
  if (cfg) { cfg.fixedProxy = value; cfg.dirty = true }
}

onMounted(() => {
  loadAll()
  loadProxyConfig()
})
</script>
