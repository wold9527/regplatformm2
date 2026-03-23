<template>
  <div v-bind="attrs" class="flex flex-col min-h-0 overflow-y-auto scroll-thin glass rounded-xl" :class="[mobile ? 'w-full pb-4' : 'w-72 flex-none mobile-hide', vipCardGlow]">

    <!-- 用户信息 -->
    <div class="flex-none px-3 pt-3 pb-2.5">
      <div class="flex items-center gap-2.5">
        <!-- 头像：稍大一点，增加视觉锚点 -->
        <div class="relative flex-none">
          <img :src="user.avatar_url || undefined" class="w-10 h-10 rounded-full" :class="vipAvatarRing"
            @error="(e: any) => e.target.src='data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 44 44%22><rect fill=%22%23334155%22 width=%2244%22 height=%2244%22 rx=%2222%22/><text x=%2250%25%22 y=%2258%25%22 text-anchor=%22middle%22 fill=%22%23fff%22 font-size=%2216%22>?</text></svg>'">
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-1.5">
            <span class="text-sm font-bold truncate leading-tight">{{ user.name || user.username }}</span>
            <span class="badge-base tip" :class="vipBadgeClass" data-tip="您的账户等级">{{ vipLabel }}</span>
          </div>
          <div class="text-[11px] text-t-faint mt-0.5 truncate">@{{ user.username }}</div>
        </div>
      </div>

      <!-- 积分条：重新设计，标签用 letter-spacing 增加层次 -->
      <div class="mt-2.5 rounded-xl overflow-hidden border border-b-panel"
        :class="{ 'shimmer-gold': user.role >= 10, 'shimmer-red': user.role >= 100 }">
        <div class="grid gap-0 divide-x divide-b-panel"
          :class="user.mode === 'newapi' ? 'grid-cols-3' : 'grid-cols-2'"
          style="background: var(--bg-inset)">
          <!-- 每日上限 -->
          <div class="px-2.5 py-2 tip" data-tip="当前平台每日最大注册总量（凌晨6点重置），0 表示不限">
            <div class="text-[9px] text-t-faint uppercase tracking-widest font-medium">上限</div>
            <div class="text-sm font-bold text-t-secondary mt-0.5 font-mono">{{ currentPlatformMaxDisplay || '不限' }}</div>
          </div>
          <!-- 可用次数：核心指标，字号最大 -->
          <div class="px-2.5 py-2 tip" :class="user.mode === 'newapi' ? 'text-center' : 'text-right'"
            data-tip="内部积分 + 免费试用剩余，每次注册消耗 1 次">
            <div class="text-[9px] text-t-faint uppercase tracking-widest font-medium">次数</div>
            <div class="text-xl font-black mt-0.5 font-mono leading-none tabular-nums"
              :class="(user.registrations_available || 0) + (freeTrial.remaining || 0) > 0 ? 'text-warn' : 'text-t-muted'">
              {{ (user.registrations_available || 0) + (freeTrial.remaining || 0) }}
            </div>
            <div v-if="freeTrial.remaining > 0" class="text-[9px] text-amber-400/80 mt-0.5">含 {{ freeTrial.remaining }} 免费</div>
          </div>
          <!-- NewAPI 余额 -->
          <div v-if="user.mode === 'newapi'" class="px-2.5 py-2 text-right tip"
            data-tip="您在 New-API 的 USD 余额，可用于购买注册次数">
            <div class="text-[9px] text-t-faint uppercase tracking-widest font-medium">余额</div>
            <div class="text-sm font-bold mt-0.5 font-mono" :class="user.newapi_balance > 0 ? 'text-emerald-400' : 'text-t-secondary'">
              {{ user.newapi_balance_display || '$0' }}
            </div>
            <div v-if="user.newapi_available > 0" class="text-[9px] text-emerald-400/60 mt-0.5 tip tip-bottom"
              data-tip="当前 API 余额按单价可购买的次数">可购 {{ user.newapi_available }} 次</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 新用户免费试用横幅 -->
    <div v-if="freeTrial.eligible || freeTrial.remaining > 0" class="flex-none px-3 py-1.5 section-divide">
      <div class="glass-light rounded-lg p-2.5 border border-amber-500/20">
        <template v-if="freeTrial.eligible && !freeTrial.claimed">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <!-- 礼物图标：SVG 替代 emoji，风格一致 -->
              <div class="w-7 h-7 rounded-lg bg-amber-500/15 flex items-center justify-center flex-none">
                <svg class="w-3.5 h-3.5 text-warn" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v13m0-13V6a2 2 0 112 2h-2zm0 0V5.5A2.5 2.5 0 109.5 8H12zm-7 4h14M5 12a2 2 0 110-4h14a2 2 0 110 4M5 12v7a2 2 0 002 2h10a2 2 0 002-2v-7"/>
                </svg>
              </div>
              <div>
                <div class="text-xs font-bold text-warn leading-tight">新用户福利</div>
                <div class="text-[10px] text-t-secondary mt-0.5">免费赠送 <span class="text-warn font-bold font-mono">{{ freeTrial.total }}</span> 次注册体验</div>
              </div>
            </div>
            <button @click="claimFreeTrial"
              class="px-3 py-1.5 bg-gradient-to-r from-amber-500 to-orange-500 text-white text-[11px] font-bold rounded-lg hover:from-amber-400 hover:to-orange-400 transition shadow-lg shadow-amber-500/20 flex-none">
              领取
            </button>
          </div>
        </template>
        <template v-else-if="freeTrial.claimed || (!freeTrial.eligible && freeTrial.remaining > 0)">
          <div class="flex items-center gap-2">
            <!-- 勾选图标 -->
            <div class="w-5 h-5 rounded-full bg-amber-500/15 flex items-center justify-center flex-none">
              <svg class="w-3 h-3 text-warn" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
              </svg>
            </div>
            <div class="text-[11px] text-warn">
              免费试用剩余 <span class="font-bold font-mono">{{ freeTrial.remaining }}</span> 次
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- 兑换码 / 购买次数（合并输入框） -->
    <div class="flex-none px-3 py-1.5 space-y-1.5">
      <div class="flex gap-2">
        <input v-model="smartInput" type="text"
          :placeholder="user.mode === 'newapi' ? '兑换码 或 购买次数' : '输入兑换码'"
          class="flex-1 bg-s-inset border border-b-panel rounded-lg px-2.5 py-1 text-xs outline-none transition placeholder:text-t-faint"
          :class="smartInputMode === 'purchase' ? 'focus:border-emerald-500/60' : 'focus:border-amber-500/60'"
          @keydown.enter="smartAction">
        <button @click="smartAction($event)" :disabled="!smartInput.trim() || purchaseProcessing"
          class="text-white text-xs font-medium px-3 py-1.5 rounded-lg transition disabled:bg-gray-700 disabled:text-gray-500"
          :class="smartInputMode === 'purchase' ? 'bg-emerald-600/80 hover:bg-emerald-500/80' : 'bg-amber-600/80 hover:bg-amber-500/80'">
          {{ purchaseProcessing ? '...' : smartInputMode === 'purchase' ? '购买' : '兑换' }}
        </button>
      </div>
      <div v-if="smartInputMode === 'purchase' && smartInputAmount > 0 && currentPlatformPrice > 0"
        class="text-[10px] text-emerald-400/80 px-0.5 tip" data-tip="购买次数 × 单价 = 预扣金额">
        {{ smartInputAmount }} 次 × ${{ currentPlatformPrice }}/次 = ${{ (smartInputAmount * currentPlatformPrice).toFixed(4) }}
      </div>
    </div>

    <!-- 任务配置 -->
    <div class="flex-none p-3 section-divide space-y-2">
      <!-- 平台选择 pill -->
      <div class="flex bg-s-inset rounded-lg p-0.5 gap-0.5">
        <button @click="limits.platform_grok_enabled && (taskForm.platform = 'grok')" :disabled="isRunning || isQueued || !limits.platform_grok_enabled"
          :class="[taskForm.platform === 'grok' ? 'bg-blue-600/80 text-white shadow' : 'text-gray-500 hover:text-gray-300', !limits.platform_grok_enabled ? 'opacity-30 cursor-not-allowed line-through' : '']"
          class="flex-1 text-[11px] font-medium py-1.5 rounded-md transition disabled:opacity-60 tip tip-bottom flex items-center justify-center gap-1" data-tip="选择要注册的平台，灰色表示已关闭">
          <span class="w-1.5 h-1.5 rounded-full flex-none" :class="platformHealthColor('grok')"></span>
          Grok
          <span v-if="isPlatformFreeByKey('grok')" class="text-[8px] px-1 py-0 rounded bg-amber-500/30 text-amber-300 font-bold leading-tight">免费</span>
        </button>
        <button @click="limits.platform_openai_enabled && (taskForm.platform = 'openai')" :disabled="isRunning || isQueued || !limits.platform_openai_enabled"
          :class="[taskForm.platform === 'openai' ? 'bg-green-600/80 text-white shadow' : 'text-gray-500 hover:text-gray-300', !limits.platform_openai_enabled ? 'opacity-30 cursor-not-allowed line-through' : '']"
          class="flex-1 text-[11px] font-medium py-1.5 rounded-md transition disabled:opacity-60 tip tip-bottom flex items-center justify-center gap-1" data-tip="选择要注册的平台，灰色表示已关闭">
          <span class="w-1.5 h-1.5 rounded-full flex-none" :class="platformHealthColor('openai')"></span>
          OpenAI
          <span v-if="isPlatformFreeByKey('openai')" class="text-[8px] px-1 py-0 rounded bg-amber-500/30 text-amber-300 font-bold leading-tight">免费</span>
        </button>
        <button @click="limits.platform_kiro_enabled && (taskForm.platform = 'kiro')" :disabled="isRunning || isQueued || !limits.platform_kiro_enabled"
          :class="[taskForm.platform === 'kiro' ? 'bg-amber-600/80 text-white shadow' : 'text-gray-500 hover:text-gray-300', !limits.platform_kiro_enabled ? 'opacity-30 cursor-not-allowed line-through' : '']"
          class="flex-1 text-[11px] font-medium py-1.5 rounded-md transition disabled:opacity-60 tip tip-bottom flex items-center justify-center gap-1" data-tip="选择要注册的平台，灰色表示已关闭">
          <span class="w-1.5 h-1.5 rounded-full flex-none" :class="platformHealthColor('kiro')"></span>
          Kiro
          <span v-if="isPlatformFreeByKey('kiro')" class="text-[8px] px-1 py-0 rounded bg-amber-500/30 text-amber-300 font-bold leading-tight">免费</span>
        </button>
        <button @click="limits.platform_gemini_enabled && (taskForm.platform = 'gemini')" :disabled="isRunning || isQueued || !limits.platform_gemini_enabled"
          :class="[taskForm.platform === 'gemini' ? 'bg-purple-600/80 text-white shadow' : 'text-gray-500 hover:text-gray-300', !limits.platform_gemini_enabled ? 'opacity-30 cursor-not-allowed line-through' : '']"
          class="flex-1 text-[11px] font-medium py-1.5 rounded-md transition disabled:opacity-60 tip tip-bottom flex items-center justify-center gap-1" data-tip="选择要注册的平台，灰色表示已关闭">
          <span class="w-1.5 h-1.5 rounded-full flex-none" :class="platformHealthColor('gemini')"></span>
          Gemini
          <span v-if="isPlatformFreeByKey('gemini')" class="text-[8px] px-1 py-0 rounded bg-amber-500/30 text-amber-300 font-bold leading-tight">免费</span>
        </button>
      </div>
      <div>
        <!-- 注册数量：居中大字高亮 -->
        <div class="text-center mb-2">
          <label class="text-[10px] text-t-muted">注册数量</label>
          <div class="text-3xl font-black font-mono tabular-nums leading-none mt-1 transition-all duration-300"
            :class="isRunning ? 'text-t-muted' : taskForm.platform === 'grok' ? 'text-blue-400' : taskForm.platform === 'openai' ? 'text-emerald-400' : taskForm.platform === 'gemini' ? 'text-purple-400' : 'text-amber-400'">
            {{ taskForm.target_count }}
          </div>
        </div>
        <div class="relative">
          <input v-model.number="taskForm.target_count" type="range" min="1" :max="sliderMax" step="1" :disabled="isRunning || isQueued"
            class="slider-input w-full h-1.5 rounded-full appearance-none cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            :style="`background: linear-gradient(to right, var(--slider-fill) 0%, var(--slider-fill) ${sliderMax > 1 ? ((taskForm.target_count - 1) / (sliderMax - 1)) * 100 : 100}%, var(--slider-track) ${sliderMax > 1 ? ((taskForm.target_count - 1) / (sliderMax - 1)) * 100 : 100}%, var(--slider-track) 100%)`">
          <div class="flex justify-between mt-1">
            <span class="text-[9px] text-t-faint">1</span>
            <div class="flex gap-1.5">
              <button v-for="q in quickTargets" :key="q" @click="taskForm.target_count = q" :disabled="isRunning || isQueued"
                class="text-[9px] px-1.5 py-0.5 rounded bg-s-hover text-t-muted hover:text-blue-400 hover:bg-s-hover transition disabled:opacity-30"
                :class="taskForm.target_count === q ? 'text-blue-400 bg-blue-900/30' : ''">{{ q }}</button>
            </div>
            <span class="text-[9px] text-t-faint">{{ sliderMax }}</span>
          </div>
        </div>
      </div>
      <div v-if="estimatedTime" class="text-[10px] text-t-muted text-right mt-0.5 tip" data-tip="根据历史平均速度估算的完成时间">
        预估 <span class="text-accent font-mono">{{ estimatedTime }}</span>
      </div>

      <!-- 排队中：取消排队 -->
      <button v-if="isQueued" @click="stopTask" :disabled="processing"
        class="action-btn action-btn-queue w-full relative overflow-hidden text-white text-sm font-bold py-2.5 rounded-xl flex items-center justify-center gap-2 tip shadow-lg shadow-amber-900/30 disabled:opacity-50 disabled:cursor-not-allowed"
        data-tip="取消排队，预扣的次数会自动退还">
        <svg class="w-4 h-4 flex-none" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
        <span>{{ processing ? '处理中...' : '取消排队' }}</span>
      </button>

      <!-- 运行中：停止任务 — 红色高对比 -->
      <button v-else-if="isRunning" @click="stopTask" :disabled="processing"
        class="action-btn action-btn-stop w-full relative overflow-hidden text-white text-sm font-bold py-2.5 rounded-xl flex items-center justify-center gap-2 tip shadow-lg shadow-red-900/40 disabled:opacity-50 disabled:cursor-not-allowed"
        data-tip="停止当前任务，未使用的次数会自动退还">
        <span class="absolute inset-0 opacity-10" style="background: radial-gradient(circle at 30% 50%, rgba(255,255,255,0.4) 0%, transparent 60%)"></span>
        <svg class="w-4 h-4 flex-none relative z-10" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd"/></svg>
        <span class="relative z-10">停止任务</span>
      </button>

      <!-- 空闲：开始注册 — 主操作按钮，最高视觉权重 -->
      <button v-else @click="handleStart"
        :disabled="processing || !currentPlatformEnabled || (limits.daily_remaining === 0) || (currentPlatformLimit?.daily_remaining === 0 && currentPlatformLimit?.daily_limit > 0) || (currentPlatformFree && taskMode === 'free' && !currentFreeModeAvailable) || (!currentPlatformFree && ((user.registrations_available || 0) + (freeTrial.remaining || 0)) < taskForm.target_count) || (currentPlatformFree && taskMode === 'paid' && ((user.registrations_available || 0) + (freeTrial.remaining || 0)) < taskForm.target_count)"
        class="action-btn action-btn-start w-full relative overflow-hidden text-white text-sm font-bold py-2.5 rounded-xl flex items-center justify-center gap-2 tip shadow-lg shadow-blue-900/40 disabled:shadow-none disabled:opacity-50 disabled:cursor-not-allowed"
        data-tip="创建并启动注册任务，会预扣对应次数">
        <span class="absolute inset-0 action-btn-shine opacity-0 transition-opacity duration-300"></span>
        <svg class="w-4 h-4 flex-none relative z-10" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd"/></svg>
        <span class="relative z-10">
          {{ processing ? '处理中...' : !currentPlatformEnabled ? `${platformLabel} 已关闭` : (limits.daily_remaining === 0 || (currentPlatformLimit?.daily_remaining === 0 && currentPlatformLimit?.daily_limit > 0)) ? '今日已达上限' : (currentPlatformFree && taskMode === 'free') ? `开始注册 ${platformLabel}（免费）` : `开始注册 ${platformLabel}` }}
        </span>
      </button>

      <div v-if="!(currentPlatformFree && taskMode === 'free') && ((user.registrations_available || 0) + (freeTrial.remaining || 0)) < taskForm.target_count && limits.max_target > 0"
        class="text-[11px] text-warn">余额不足，请{{ user.mode === 'newapi' ? '在上方输入数量购买或使用兑换码' : '到 new-api 充值或使用兑换码' }}</div>
      <!-- 今日注册：优先显示平台级，回退全局 -->
      <div v-if="currentPlatformDailyText" class="text-[11px]"
        :class="(currentPlatformLimit?.daily_remaining === 0 && currentPlatformLimit?.daily_limit > 0) || limits.daily_remaining === 0 ? 'text-err' : 'text-accent'">
        今日注册: {{ currentPlatformDailyText }}
        <span v-if="(currentPlatformLimit?.daily_remaining === 0 && currentPlatformLimit?.daily_limit > 0) || limits.daily_remaining === 0">（已达上限，凌晨 6 点重置）</span>
        <span v-else>（剩余 {{ currentPlatformLimit?.daily_remaining ?? limits.daily_remaining }} 次）</span>
      </div>
      <div v-if="!currentPlatformEnabled" class="text-[11px] text-err">
        {{ platformLabel }} 平台注册已关闭
      </div>
      <div v-if="currentPlatformFree" class="space-y-1.5">
        <!-- 免费/付费模式切换 -->
        <div class="flex items-center gap-1.5">
          <button @click="taskMode = 'free'" :disabled="isRunning || isQueued"
            :class="taskMode === 'free' ? 'bg-amber-600/80 text-white shadow' : 'text-t-muted hover:text-amber-300 bg-s-panel'"
            class="flex-1 text-[10px] font-bold py-1 rounded transition">
            免费模式
          </button>
          <button @click="taskMode = 'paid'" :disabled="isRunning || isQueued"
            :class="taskMode === 'paid' ? 'bg-blue-600/80 text-white shadow' : 'text-t-muted hover:text-blue-300 bg-s-panel'"
            class="flex-1 text-[10px] font-bold py-1 rounded transition">
            付费模式
          </button>
        </div>
        <!-- 免费模式状态 -->
        <div v-if="taskMode === 'free' && currentFreeMode" class="text-[10px]">
          <div class="flex items-center gap-2" :class="currentFreeModeAvailable ? 'text-warn' : 'text-err'">
            <span v-if="currentFreeMode.daily_limit > 0">免费 <span class="font-mono font-bold text-white">{{ currentFreeMode.daily_used }}</span>/<span class="text-t-secondary">{{ currentFreeMode.daily_limit }}</span></span>
            <span v-else>免费 不限量</span>
            <span v-if="currentFreeMode.task_limit > 0">单次 ≤<span class="font-mono font-bold text-white">{{ currentFreeMode.task_limit }}</span></span>
            <span v-if="currentFreeMode.cooldown_remaining > 0" class="text-err font-bold">⏳<span class="font-mono">{{ Math.ceil(currentFreeMode.cooldown_remaining / 60) }}</span>分</span>
          </div>
          <div v-if="!currentFreeModeAvailable" class="text-err mt-0.5">
            {{ currentFreeMode.cooldown_remaining > 0 ? '冷却中' : '额度已用完' }}，可切换<span class="text-info font-bold cursor-pointer" @click="taskMode = 'paid'">付费模式</span>
          </div>
        </div>
        <div v-else-if="taskMode === 'paid'" class="text-[10px] text-info">
          付费模式：无限制，消耗积分
          <div v-if="currentPlatformPrice > 0 && taskForm.target_count > 0" class="text-[11px] text-accent mt-1">
            预估费用: ${{ (taskForm.target_count * currentPlatformPrice).toFixed(1) }}
            ({{ taskForm.target_count }} 次 × {{ currentPlatformPriceDisplay }}/次)
            · 失败自动退还次数
          </div>
        </div>
      </div>
      <div v-else-if="currentPlatformPrice > 0 && taskForm.target_count > 0"
        class="text-[11px] text-accent tip" data-tip="注册数 × 单价，失败的会自动退还">
        预估费用: ${{ (taskForm.target_count * currentPlatformPrice).toFixed(1) }}
        ({{ taskForm.target_count }} 次 × {{ currentPlatformPriceDisplay }}/次)
        · 失败自动退还次数
      </div>
    </div>

    <!-- 通知公告（交易记录） -->
    <div class="flex-1 min-h-[160px] flex flex-col section-divide">
      <div class="flex-none px-3 py-1.5 flex items-center justify-between">
        <div class="flex items-center gap-1.5">
          <svg class="w-3.5 h-3.5 text-warn" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/></svg>
          <span class="text-xs font-bold text-t-muted uppercase tracking-wider tip" data-tip="积分变动记录：购买、消费、退还、兑换等">通知</span>
        </div>
        <span class="text-[10px] text-t-faint">{{ txHistory.length }} 条</span>
      </div>
      <div class="flex-1 overflow-y-auto scroll-thin px-3 pb-2 space-y-1">
        <div v-if="!txHistoryLoaded" class="text-t-faint text-center py-4 text-[10px]">加载中...</div>
        <div v-else-if="txHistory.length === 0" class="text-t-faint text-center py-4 text-[10px]">暂无通知</div>
        <div v-for="tx in txHistory.slice(0, txDisplayCount)" :key="tx.id"
          class="flex items-start gap-2 px-2 py-1.5 rounded-lg hover:bg-s-hover transition">
          <div class="flex-none mt-0.5 w-5 h-5 rounded-full flex items-center justify-center"
            :class="tx.amount > 0 ? 'bg-ok-dim' : 'bg-err-dim'">
            <svg v-if="tx.amount > 0" class="w-3 h-3 text-ok" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v12m6-6H6"/></svg>
            <svg v-else class="w-3 h-3 text-err" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18 12H6"/></svg>
          </div>
          <div class="flex-1 min-w-0">
            <div class="text-[11px] text-t-primary truncate">{{ tx.description }}</div>
            <div class="text-[10px] text-t-faint mt-0.5">{{ new Date(tx.created_at).toLocaleString() }}</div>
          </div>
          <span class="text-[11px] font-mono font-bold flex-none"
            :class="tx.amount > 0 ? 'text-ok' : 'text-err'">
            {{ tx.amount > 0 ? '+' : '' }}{{ tx.amount }}
          </span>
        </div>
        <!-- 查看更多 -->
        <div v-if="txHistoryLoaded && txHistory.length > txDisplayCount"
          class="px-2 py-1.5 text-center">
          <button @click="txDisplayCount += 20"
            class="text-[10px] px-3 py-1 rounded-lg text-t-secondary hover:text-t-primary hover:bg-s-hover transition">
            查看更多（{{ Math.min(txDisplayCount, txHistory.length) }} / {{ txHistory.length }}）
          </button>
        </div>
      </div>
    </div>

  </div>
  <ProxyDialog v-if="showProxyDialog" :platform="taskForm.platform" @close="showProxyDialog = false" @confirm="onProxyConfirm" />

  <!-- 排队等待弹窗 -->
  <Teleport to="body">
    <Transition name="queue-modal">
      <div v-if="queueModal.show" class="fixed inset-0 z-[9999] flex items-center justify-center" @click.self="queueModal.show = false">
        <div class="absolute inset-0 bg-black/60 backdrop-blur-sm"></div>
        <div class="relative w-80 rounded-2xl overflow-hidden shadow-2xl" style="background:var(--bg-admin);border:1px solid var(--border-glass)">
          <!-- 顶部动画条 -->
          <div class="h-1 bg-gradient-to-r from-amber-500 via-orange-400 to-amber-500 queue-bar-anim"></div>
          <div class="p-5 text-center space-y-4">
            <!-- 排队动画图标 -->
            <div class="flex justify-center">
              <div class="w-16 h-16 rounded-full bg-amber-500/10 flex items-center justify-center queue-pulse">
                <svg class="w-8 h-8 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
                </svg>
              </div>
            </div>
            <div class="text-sm font-bold text-t-primary">排队等待中</div>
            <div v-if="queueModal.position > 0" class="text-xs text-t-secondary">
              前方 <span class="text-warn font-bold font-mono text-base">{{ queueModal.position }}</span> 位用户
            </div>
            <div v-if="queueModal.waitTime" class="text-xs text-t-muted">
              预计等待 <span class="text-accent font-mono font-semibold">{{ queueModal.waitTime }}</span>
            </div>
            <!-- 进度提示 -->
            <div class="text-[10px] text-t-faint leading-relaxed">
              前方用户完成后将自动开始您的任务<br>无需等待，可先做其他事情
            </div>
            <button @click="queueModal.show = false"
              class="w-full py-2 rounded-lg text-xs font-medium text-t-secondary glass-light hover:bg-s-hover transition">
              知道了，后台等待
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, useAttrs } from 'vue'
import { useDashboard } from '../../composables/useDashboard'
import { useTaskEngine } from '../../composables/useTaskEngine'
import ProxyDialog from './ProxyDialog.vue'

defineOptions({ inheritAttrs: false })
const attrs = useAttrs()

defineProps<{ mobile?: boolean }>()

const {
  user, limits, taskForm, taskMode, isRunning, isQueued, processing,
  freeTrial, smartInput, purchaseProcessing, smartInputMode, smartInputAmount,
  currentPlatformEnabled, currentPlatformFree, currentFreeMode, currentFreeModeAvailable,
  currentPlatformPrice, currentPlatformPriceDisplay, platformLabel,
  currentPlatformDailyText, currentPlatformMaxDisplay,
  currentPlatformLimit,
  sliderMax, quickTargets, estimatedTime,
  vipLabel, vipBadgeClass, vipCardGlow, vipAvatarRing,
  isPlatformFreeByKey, platformHealthColor,
  claimFreeTrial, smartAction, txHistory, txHistoryLoaded, txDisplayCount,
  queueModal,
} = useDashboard()

const { startTask, stopTask } = useTaskEngine()

const showProxyDialog = ref(false)

function handleStart() {
  if (taskForm.platform === 'kiro' || taskForm.platform === 'gemini') {
    showProxyDialog.value = true
  } else {
    startTask()
  }
}

function onProxyConfirm(proxyId?: number) {
  showProxyDialog.value = false
  startTask(proxyId)
}
</script>
