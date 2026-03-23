<template>
  <div v-show="adminTab === 'hfspace'" class="space-y-6">

    <!-- 概览卡片 -->
    <div class="grid grid-cols-4 gap-3 mobile-grid-2">
      <div v-if="overviewLoading" class="col-span-4 text-center text-t-faint text-[10px] py-4">
        <span class="animate-pulse">加载概览中...</span>
      </div>
      <div v-else-if="!overviewHasData" class="col-span-4 text-center text-t-faint text-[10px] py-4">暂无 Space 数据，请先部署 Space</div>
      <template v-else>
        <div v-for="o in overview" :key="o.service" class="glass-light rounded-xl p-3">
          <div class="flex items-center justify-between mb-1.5">
            <span class="text-[11px] font-bold uppercase"
              :class="{ 'text-info': o.service === 'grok', 'text-ok': o.service === 'openai', 'text-warn': o.service === 'kiro', 'text-purple-400': o.service === 'gemini', 'text-gray-400': o.service === 'ts' }">
              {{ ({ grok: 'Grok', openai: 'OpenAI', kiro: 'Kiro', gemini: 'Gemini', ts: 'TS' } as Record<string, string>)[o.service] || o.service }}
            </span>
            <span class="text-[10px] text-t-faint">{{ o.total }} 个</span>
          </div>
          <div class="flex items-baseline gap-2">
            <span class="text-lg font-bold text-ok">{{ o.healthy }}</span>
            <span class="text-[10px] text-t-muted">healthy</span>
          </div>
          <div class="flex gap-2 mt-1 text-[10px] flex-wrap">
            <span v-if="o.building" class="text-blue-400">{{ o.building }} building</span>
            <span v-if="o.banned" class="text-err">{{ o.banned }} banned</span>
            <span v-if="o.sleeping" class="text-warn">{{ o.sleeping }} sleep</span>
            <span v-if="o.dead" class="text-t-faint">{{ o.dead }} dead</span>
            <span v-if="o.unknown" class="text-gray-500">{{ o.unknown }} unknown</span>
          </div>
        </div>
      </template>
    </div>

    <!-- 自动发现 Space -->
    <div class="glass-light rounded-xl p-4">
      <div class="flex items-center justify-between">
        <div class="text-xs font-bold text-t-secondary uppercase tracking-wider">自动发现 Space</div>
        <div class="flex items-center gap-2">
          <select v-model="discoverDefaultService"
            class="bg-s-inset border border-b-panel rounded-lg px-2 py-1 text-[10px] text-white focus:outline-none">
            <option value="">不指定服务（标记 unknown）</option>
            <option value="openai">默认归类 OpenAI</option>
            <option value="grok">默认归类 Grok</option>
            <option value="kiro">默认归类 Kiro</option>
            <option value="gemini">默认归类 Gemini</option>
            <option value="ts">默认归类 TS</option>
          </select>
          <button @click="discoverSpaces(discoverDefaultService || undefined)" :disabled="discoverLoading"
            class="px-4 py-1.5 rounded-lg text-xs font-bold bg-gradient-to-r from-amber-600 to-orange-600 hover:from-amber-500 hover:to-orange-500 disabled:from-gray-700 disabled:to-gray-700 disabled:text-gray-500 text-white transition">
            {{ discoverLoading ? '扫描中...' : '扫描所有 Token' }}
          </button>
          <button @click="redetectService()" :disabled="redetectLoading"
            class="px-4 py-1.5 rounded-lg text-xs font-bold bg-cyan-600/80 hover:bg-cyan-500 disabled:bg-gray-700 disabled:text-gray-500 text-white transition">
            {{ redetectLoading ? '识别中...' : '重新识别 unknown' }}
          </button>
        </div>
      </div>
      <div v-if="discoverResult" class="mt-2 text-[10px] text-t-secondary">
        扫描 <span class="text-accent font-bold">{{ discoverResult.scanned }}</span> 个 Token
        · 找到 <span class="text-accent font-bold">{{ discoverResult.found }}</span> 个 Space
        · 新导入 <span class="text-ok font-bold">{{ discoverResult.imported }}</span>
        · 跳过 <span class="text-t-faint">{{ discoverResult.skipped }}</span>
        <span v-if="discoverResult.errors?.length" class="text-err ml-1">（{{ discoverResult.errors.length }} 个错误）</span>
      </div>
      <div v-if="redetectResult" class="mt-2 text-[10px] text-t-secondary">
        共 <span class="text-accent font-bold">{{ redetectResult.found }}</span> 个 unknown
        · 成功识别 <span class="text-ok font-bold">{{ redetectResult.imported }}</span>
        · 未识别 <span class="text-t-faint">{{ redetectResult.skipped }}</span>
        <span v-if="redetectResult.errors?.length" class="text-err ml-1">（{{ redetectResult.errors.length }} 个错误）</span>
      </div>
      <div v-if="discoverResult?.errors?.length" class="mt-1 max-h-16 overflow-y-auto scroll-thin space-y-0.5">
        <div v-for="(e, i) in discoverResult.errors" :key="i" class="text-[10px] text-err">{{ e }}</div>
      </div>
    </div>

    <!-- Token 管理 -->
    <div class="glass-light rounded-xl p-4 space-y-3">
      <div class="flex items-center justify-between">
        <div class="text-xs font-bold text-t-secondary uppercase tracking-wider">HF Token 管理</div>
        <button @click="validateAllTokens" :disabled="validateAllLoading"
          class="px-3 py-1 rounded-lg text-[10px] font-bold bg-blue-600/80 hover:bg-blue-500 disabled:bg-gray-700 disabled:text-gray-500 text-white transition">
          {{ validateAllLoading ? '验证中...' : '批量验证全部' }}
        </button>
      </div>
      <div class="grid grid-cols-3 gap-3">
        <input v-model="tokenForm.label" type="text" placeholder="标签（如：账号1）"
          class="bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-purple-500 focus:outline-none">
        <input v-model="tokenForm.token" type="password" placeholder="hf_xxxxxxxxxx"
          class="bg-s-inset border border-b-panel rounded-lg px-3 py-2 text-sm text-white focus:border-purple-500 focus:outline-none">
        <button @click="createToken" :disabled="!tokenForm.label.trim() || !tokenForm.token.trim()"
          class="py-2 rounded-lg text-xs font-bold bg-gradient-to-r from-purple-600 to-indigo-600 hover:from-purple-500 hover:to-indigo-500 disabled:from-gray-700 disabled:to-gray-700 disabled:text-gray-500 text-white transition">
          添加 Token
        </button>
      </div>
      <table v-if="tokens.length > 0" class="w-full text-sm">
        <thead class="text-[10px] text-t-muted border-b border-b-panel">
          <tr><th class="px-2 py-2 text-left">标签</th><th class="px-2 py-2 text-left">用户名</th><th class="px-2 py-2 text-center">Token</th><th class="px-2 py-2 text-center">状态</th><th class="px-2 py-2 text-center">Space</th><th class="px-2 py-2 text-right">操作</th></tr>
        </thead>
        <tbody class="divide-y divide-b-panel">
          <tr v-for="t in tokens" :key="t.id" class="hover:bg-s-hover transition">
            <td class="px-2 py-2 text-xs">{{ t.label }}</td>
            <td class="px-2 py-2 text-xs font-mono text-accent">{{ t.username }}</td>
            <td class="px-2 py-2 text-center text-[10px] font-mono text-t-muted">{{ t.token }}</td>
            <td class="px-2 py-2 text-center">
              <span class="text-[10px] px-1.5 py-0.5 rounded" :class="t.is_valid ? 'bg-ok-dim text-ok' : 'bg-red-900/30 text-err'">
                {{ t.is_valid ? '有效' : '无效' }}
              </span>
            </td>
            <td class="px-2 py-2 text-center text-xs font-mono">{{ t.space_used }}</td>
            <td class="px-2 py-2 text-right space-x-1">
              <button @click="validateToken(t.id)" class="text-[10px] px-2 py-1 rounded bg-blue-600/60 hover:bg-blue-500 text-white transition">验证</button>
              <button @click="deleteToken(t.id)" class="text-[10px] px-2 py-1 rounded bg-red-600/60 hover:bg-red-500 text-white transition">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else-if="tokensLoading" class="text-t-faint text-center py-4 text-[10px] animate-pulse">加载 Token 中...</div>
      <div v-else class="text-t-faint text-center py-4 text-[10px]">暂无 Token</div>
    </div>

    <!-- Space 管理 -->
    <div class="glass-light rounded-xl p-4 space-y-3">
      <div class="flex items-center justify-between">
        <div class="text-xs font-bold text-t-secondary uppercase tracking-wider">Space 管理</div>
        <div class="flex items-center gap-2">
          <select v-model="spaceFilter" @change="onFilterChange"
            class="bg-s-inset border border-b-panel rounded-lg px-2 py-1 text-[10px] text-white focus:outline-none">
            <option value="">全部服务</option>
            <option value="openai">OpenAI</option>
            <option value="grok">Grok</option>
            <option value="kiro">Kiro</option>
            <option value="gemini">Gemini</option>
            <option value="ts">TS</option>
          </select>
          <select v-model="statusFilter" @change="onFilterChange"
            class="bg-s-inset border border-b-panel rounded-lg px-2 py-1 text-[10px] text-white focus:outline-none">
            <option value="">全部状态</option>
            <option value="healthy">healthy</option>
            <option value="building">building</option>
            <option value="banned">banned</option>
            <option value="sleeping">sleeping</option>
            <option value="dead">dead</option>
            <option value="unknown">unknown</option>
          </select>
          <button @click="checkHealth(spaceFilter || undefined)" :disabled="healthLoading"
            class="text-[10px] px-2.5 py-1 rounded-md bg-cyan-600/60 hover:bg-cyan-500 disabled:bg-gray-700 disabled:text-gray-500 text-white transition">
            {{ healthLoading ? '检查中...' : '健康检查' }}
          </button>
          <button @click="purgeSpaces(spaceFilter || undefined)" :disabled="purgeLoading"
            class="text-[10px] px-2.5 py-1 rounded-md bg-red-600/60 hover:bg-red-500 disabled:bg-gray-700 disabled:text-gray-500 text-white transition">
            {{ purgeLoading ? '清理中...' : '清理不可用' }}
          </button>
          <button v-if="selectedSpaceIds.length > 0" @click="doBatchDelete" :disabled="batchDeleteLoading"
            class="text-[10px] px-2.5 py-1 rounded-md bg-red-700/80 hover:bg-red-600 disabled:bg-gray-700 disabled:text-gray-500 text-white transition">
            {{ batchDeleteLoading ? '删除中...' : `批量删除 (${selectedSpaceIds.length})` }}
          </button>
          <button @click="reloadSpaces" class="text-[10px] text-t-muted hover:text-white transition px-2 py-1 rounded hover:bg-s-hover">刷新</button>
        </div>
      </div>

      <!-- 手动添加 Space -->
      <div class="flex gap-2 items-end">
        <select v-model="addSpaceForm.service" class="bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:outline-none w-24">
          <option value="openai">OpenAI</option>
          <option value="grok">Grok</option>
          <option value="kiro">Kiro</option>
          <option value="gemini">Gemini</option>
          <option value="ts">TS</option>
        </select>
        <input v-model="addSpaceForm.url" type="text" placeholder="Space URL"
          class="flex-1 bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:border-purple-500 focus:outline-none">
        <input v-model="addSpaceForm.repo_id" type="text" placeholder="user/space-name"
          class="w-44 bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:border-purple-500 focus:outline-none">
        <select v-model.number="addSpaceForm.token_id" class="bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:outline-none w-36">
          <option :value="0">不关联 Token</option>
          <option v-for="t in tokens" :key="t.id" :value="t.id">{{ t.label || t.username }}</option>
        </select>
        <button @click="addSpace" :disabled="!addSpaceForm.url.trim() || !addSpaceForm.repo_id.trim()"
          class="text-[10px] px-3 py-1.5 rounded-md bg-purple-600/60 hover:bg-purple-500 disabled:bg-gray-700 disabled:text-gray-500 text-white transition whitespace-nowrap">
          手动添加
        </button>
      </div>

      <!-- Space 表格 -->
      <div class="relative">
        <div v-if="spacesLoading" class="absolute inset-0 bg-black/20 rounded-lg flex items-center justify-center z-10">
          <span class="text-[10px] text-t-faint animate-pulse">加载中...</span>
        </div>
        <table v-if="spaces.length > 0" class="w-full text-sm">
          <thead class="text-[10px] text-t-muted border-b border-b-panel">
            <tr>
              <th class="px-2 py-2 text-center w-8">
                <input type="checkbox" :checked="isAllSelected" @change="toggleSelectAll"
                  class="w-3 h-3 rounded border-gray-600 bg-gray-800 text-purple-500 focus:ring-purple-500/30 cursor-pointer">
              </th>
              <th class="px-2 py-2 text-left">服务</th><th class="px-2 py-2 text-left">URL</th><th class="px-2 py-2 text-left">Repo</th><th class="px-2 py-2 text-center">状态</th><th class="px-2 py-2 text-center">最后检查</th><th class="px-2 py-2 text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-b-panel">
            <tr v-for="sp in spaces" :key="sp.id" class="hover:bg-s-hover transition"
              :class="{ 'bg-purple-900/10': selectedSpaceIds.includes(sp.id) }">
              <td class="px-2 py-2 text-center">
                <input type="checkbox" :checked="selectedSpaceIds.includes(sp.id)" @change="toggleSelectSpace(sp.id)"
                  class="w-3 h-3 rounded border-gray-600 bg-gray-800 text-purple-500 focus:ring-purple-500/30 cursor-pointer">
              </td>
              <td class="px-2 py-2">
                <span class="text-[10px] px-1.5 py-0.5 rounded font-bold"
                  :class="{ 'bg-blue-900/40 text-info': sp.service === 'grok', 'bg-green-900/40 text-ok': sp.service === 'openai', 'bg-amber-900/40 text-warn': sp.service === 'kiro', 'bg-purple-900/40 text-purple-400': sp.service === 'gemini', 'bg-gray-800 text-gray-400': sp.service === 'ts' }">
                  {{ sp.service }}
                </span>
              </td>
              <td class="px-2 py-2 text-[10px] font-mono text-accent truncate max-w-[200px]" :title="sp.url">{{ sp.url }}</td>
              <td class="px-2 py-2 text-[10px] font-mono text-t-muted truncate max-w-[160px]">{{ sp.repo_id }}</td>
              <td class="px-2 py-2 text-center">
                <span class="text-[10px] px-1.5 py-0.5 rounded font-bold"
                  :class="{
                    'bg-ok-dim text-ok': sp.status === 'healthy',
                    'bg-blue-900/30 text-blue-400': sp.status === 'building',
                    'bg-red-900/30 text-err': sp.status === 'banned',
                    'bg-amber-900/30 text-warn': sp.status === 'sleeping',
                    'bg-gray-800 text-t-faint': sp.status === 'dead' || sp.status === 'unknown',
                  }">
                  {{ sp.status }}
                </span>
              </td>
              <td class="px-2 py-2 text-center text-[10px] text-t-faint">{{ sp.last_check_at ? new Date(sp.last_check_at).toLocaleString() : '-' }}</td>
              <td class="px-2 py-2 text-right">
                <button @click="deleteSpace(sp.id)" class="text-[10px] px-2 py-1 rounded bg-red-600/60 hover:bg-red-500 text-white transition">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-else-if="spacesLoading" class="text-t-faint text-center py-6 text-[10px] animate-pulse">加载 Space 中...</div>
        <div v-else class="text-t-faint text-center py-6 text-[10px]">暂无 Space</div>
      </div>

      <!-- 分页 -->
      <div v-if="spaceTotal > 0" class="flex items-center justify-between pt-1">
        <span class="text-[10px] text-t-faint">共 {{ spaceTotal }} 条</span>
        <div class="flex items-center gap-1">
          <button @click="spaceGoPage(spacePage - 1)" :disabled="spacePage <= 1"
            class="text-[10px] px-2 py-0.5 rounded disabled:text-gray-600 text-t-muted hover:text-white hover:bg-s-hover transition">&lt;</button>
          <template v-for="p in spaceTotalPages" :key="p">
            <button v-if="p === 1 || p === spaceTotalPages || (p >= spacePage - 1 && p <= spacePage + 1)"
              @click="spaceGoPage(p)"
              class="text-[10px] min-w-[24px] py-0.5 rounded transition"
              :class="p === spacePage ? 'bg-purple-600/80 text-white font-bold' : 'text-t-muted hover:text-white hover:bg-s-hover'">
              {{ p }}
            </button>
            <span v-else-if="p === spacePage - 2 || p === spacePage + 2" class="text-[10px] text-t-faint px-0.5">...</span>
          </template>
          <button @click="spaceGoPage(spacePage + 1)" :disabled="spacePage >= spaceTotalPages"
            class="text-[10px] px-2 py-0.5 rounded disabled:text-gray-600 text-t-muted hover:text-white hover:bg-s-hover transition">&gt;</button>
        </div>
      </div>
    </div>

    <!-- 部署 + 弹性管理 -->
    <div class="grid grid-cols-2 gap-3 mobile-grid-1">
      <!-- 批量部署 -->
      <div class="glass-light rounded-xl p-4 space-y-3">
        <div class="text-xs font-bold text-t-secondary uppercase tracking-wider">批量部署</div>
        <div class="space-y-2">
          <div class="flex gap-2">
            <select v-model="deployForm.service" class="bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:outline-none w-28">
              <option value="openai">OpenAI</option>
              <option value="grok">Grok</option>
              <option value="kiro">Kiro</option>
              <option value="gemini">Gemini</option>
              <option value="ts">TS</option>
            </select>
            <select v-model.number="deployForm.token_id" class="bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:outline-none flex-1 min-w-0">
              <option :value="0">全部 Token（轮询）</option>
              <option v-for="t in tokens" :key="t.id" :value="t.id">{{ t.label || t.username }}</option>
            </select>
            <input v-model.number="deployForm.count" type="number" min="1" max="50" placeholder="数量"
              class="w-20 bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:outline-none">
          </div>
          <input v-model="deployForm.release_url" type="text" placeholder="GitHub Release URL"
            class="w-full bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:border-purple-500 focus:outline-none">
        </div>
        <button @click="deploySpaces" :disabled="deployLoading || !deployForm.release_url.trim()"
          class="w-full py-2 rounded-lg text-xs font-bold bg-gradient-to-r from-purple-600 to-indigo-600 hover:from-purple-500 hover:to-indigo-500 disabled:from-gray-700 disabled:to-gray-700 disabled:text-gray-500 text-white transition">
          {{ deployLoading ? '部署中...' : '开始部署' }}
        </button>
        <!-- 部署结果详情 -->
        <div v-if="deployResult" class="glass-light rounded-lg p-3 border border-purple-500/20 animate-in text-[10px] space-y-1">
          <div class="font-bold" :class="deployResult.failed > 0 ? 'text-err' : 'text-ok'">
            部署: {{ deployResult.success }} 成功 / {{ deployResult.failed }} 失败
          </div>
          <div v-if="deployResult.deployed?.length" class="space-y-0.5">
            <div v-for="(sp, i) in deployResult.deployed" :key="i" class="text-ok">✓ {{ sp.repo_id }} → {{ sp.url }}</div>
          </div>
          <div v-if="deployResult.errors?.length" class="space-y-0.5 mt-1">
            <div v-for="(e, i) in deployResult.errors" :key="i" class="text-err">✗ {{ e }}</div>
          </div>
        </div>
        <div class="flex gap-2">
          <select v-model="updateService" class="bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:outline-none w-28">
            <option value="all">全部服务</option>
            <option value="openai">OpenAI</option>
            <option value="grok">Grok</option>
            <option value="kiro">Kiro</option>
            <option value="gemini">Gemini</option>
            <option value="ts">TS</option>
          </select>
          <button @click="updateSpaces(updateService)" :disabled="updateLoading"
            class="flex-1 py-2 rounded-lg text-xs font-bold bg-gradient-to-r from-amber-600 to-orange-600 hover:from-amber-500 hover:to-orange-500 disabled:from-gray-700 disabled:to-gray-700 disabled:text-gray-500 text-white transition">
            {{ updateLoading ? '更新中...' : '更新现有 Space' }}
          </button>
        </div>
        <div v-if="updateResult" class="glass-light rounded-lg p-3 border border-amber-500/20 animate-in text-[10px] space-y-1">
          <div class="font-bold text-amber-400">更新: {{ updateResult.updated }} 成功 / {{ updateResult.failed }} 失败</div>
          <div v-if="updateResult.logs?.length" class="max-h-48 overflow-y-auto scroll-thin space-y-0.5 mt-1">
            <div v-for="(log, i) in updateResult.logs" :key="i" class="text-t-muted">{{ log }}</div>
          </div>
        </div>
      </div>

      <!-- 弹性管理 -->
      <div class="glass-light rounded-xl p-4 space-y-3">
        <div class="text-xs font-bold text-t-secondary uppercase tracking-wider">弹性管理</div>
        <div class="space-y-2">
          <div class="flex gap-2">
            <select v-model="autoscaleForm.service" class="bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:outline-none w-28">
              <option value="all">全部服务</option>
              <option value="openai">OpenAI</option>
              <option value="grok">Grok</option>
              <option value="kiro">Kiro</option>
              <option value="gemini">Gemini</option>
              <option value="ts">TS</option>
            </select>
            <input v-model.number="autoscaleForm.target" type="number" min="0" placeholder="目标数（0=默认）"
              class="flex-1 bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs text-white focus:outline-none">
          </div>
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" v-model="autoscaleForm.dry_run"
              class="w-3.5 h-3.5 rounded border-gray-600 bg-gray-800 text-cyan-500 focus:ring-cyan-500/30">
            <span class="text-xs text-t-secondary">Dry Run（只检查不修改）</span>
          </label>
        </div>
        <div class="flex gap-2">
          <button @click="triggerAutoscale" :disabled="autoscaleLoading"
            class="flex-1 py-2 rounded-lg text-xs font-bold bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 disabled:from-gray-700 disabled:to-gray-700 disabled:text-gray-500 text-white transition">
            {{ autoscaleLoading ? '执行中...' : '执行弹性管理' }}
          </button>
          <button @click="syncCF()" :disabled="syncCFLoading"
            class="px-4 py-2 rounded-lg text-xs font-bold bg-orange-600/80 hover:bg-orange-500 disabled:bg-gray-700 disabled:text-gray-500 text-white transition">
            {{ syncCFLoading ? '同步中...' : '同步 CF' }}
          </button>
        </div>
        <!-- 弹性管理结果 -->
        <div v-if="autoscaleResult" class="glass-light rounded-lg p-3 border border-cyan-500/20 animate-in text-[10px] space-y-1">
          <div class="font-bold text-accent">{{ autoscaleResult.dry_run ? '[Dry Run]' : '' }} {{ autoscaleResult.service }}</div>
          <div class="text-t-secondary">节点: {{ autoscaleResult.before }} → {{ autoscaleResult.after }} · 健康: {{ autoscaleResult.healthy_now }}</div>
          <div v-if="autoscaleResult.logs" class="max-h-48 overflow-y-auto scroll-thin space-y-0.5 mt-1">
            <div v-for="(log, i) in autoscaleResult.logs" :key="i" class="text-t-muted">
              <span class="text-t-faint">[{{ log.step }}]</span> {{ log.message }}
            </div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAdmin } from '../../composables/useAdmin'
import { useHFSpace } from '../../composables/useHFSpace'

const { adminTab } = useAdmin()
const {
  tokens, spaces, overview, spaceFilter, statusFilter,
  tokensLoading, spacesLoading, overviewLoading, healthLoading,
  deployLoading, autoscaleLoading, syncCFLoading, updateLoading, updateService,
  tokenForm, addSpaceForm, deployForm, autoscaleForm, autoscaleResult, updateResult,
  deployResult,
  spacePage, spaceTotal, spaceTotalPages,
  loadSpaces, reloadSpaces, spaceGoPage,
  createToken, deleteToken, validateToken, validateAllTokens,
  addSpace, deleteSpace, batchDeleteSpaces, batchDeleteLoading,
  checkHealth, deploySpaces, updateSpaces,
  triggerAutoscale, syncCF, purgeSpaces,
  discoverLoading, discoverResult, discoverSpaces,
  redetectLoading, redetectResult, redetectService,
  validateAllLoading, purgeLoading,
} = useHFSpace()

/** overview 是否有实际数据 */
const overviewHasData = computed(() => Array.isArray(overview.value) && overview.value.some((o: any) => o.total > 0))

/** 切换筛选时重置到第 1 页 */
function onFilterChange() {
  spacePage.value = 1
  selectedSpaceIds.value = []
  loadSpaces(spaceFilter.value || undefined, statusFilter.value || undefined)
}

/** 自动发现默认服务类型 */
const discoverDefaultService = ref('')

/** 批量选择 */
const selectedSpaceIds = ref<number[]>([])

const isAllSelected = computed(() =>
  spaces.value.length > 0 && spaces.value.every((sp: any) => selectedSpaceIds.value.includes(sp.id))
)

function toggleSelectAll() {
  if (isAllSelected.value) {
    selectedSpaceIds.value = []
  } else {
    selectedSpaceIds.value = spaces.value.map((sp: any) => sp.id)
  }
}

function toggleSelectSpace(id: number) {
  const idx = selectedSpaceIds.value.indexOf(id)
  if (idx >= 0) {
    selectedSpaceIds.value.splice(idx, 1)
  } else {
    selectedSpaceIds.value.push(id)
  }
}

async function doBatchDelete() {
  await batchDeleteSpaces(selectedSpaceIds.value)
  selectedSpaceIds.value = []
}
</script>
