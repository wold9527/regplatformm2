<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
    role="dialog" aria-modal="true" tabindex="-1"
    @click.self="$emit('close')" @keydown.escape="$emit('close')">
    <div class="glass rounded-2xl w-[420px] max-h-[85vh] overflow-y-auto p-5 shadow-2xl">
      <h3 class="text-base font-bold text-t-primary mb-3 flex items-center gap-2">
        <svg class="w-5 h-5" :class="platform === 'gemini' ? 'text-purple-400' : 'text-amber-400'" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
        {{ platformLabel }} 注册 — 代理设置
      </h3>

      <!-- 说明 -->
      <div class="glass-light rounded-lg p-3 mb-4 text-[11px] text-t-secondary leading-relaxed space-y-1.5">
        <p>{{ platformDesc }}：</p>
        <ul class="list-disc pl-4 space-y-0.5">
          <li><span class="text-amber-400 font-medium">推荐使用家宽 IP / 住宅代理</span>，成功率最高</li>
          <li>数据中心 IP（机房代理）容易被风控拦截</li>
          <li>不填则使用系统默认代理</li>
        </ul>
        <p class="text-t-faint">支持 HTTP、HTTPS、SOCKS5 协议<br>格式：<code class="text-xs bg-s-inset px-1 rounded">协议://用户名:密码@地址:端口</code></p>
      </div>

      <!-- 已保存的代理列表 -->
      <div class="mb-3">
        <div class="text-xs font-medium text-t-secondary mb-2">选择代理</div>
        <div class="space-y-1.5">
          <!-- 系统默认 -->
          <label class="flex items-center gap-2.5 glass-light rounded-lg px-3 py-2 cursor-pointer hover:bg-s-hover transition"
            :class="selectedId === 0 ? 'ring-1 ring-blue-500/50' : ''">
            <input type="radio" :value="0" v-model="selectedId" class="accent-blue-500">
            <div class="flex-1 min-w-0">
              <div class="text-xs font-medium text-t-primary">系统默认代理</div>
              <div class="text-[10px] text-t-faint">使用管理员配置的全局代理</div>
            </div>
          </label>

          <!-- 已保存代理 -->
          <label v-for="p in proxies" :key="p.id"
            class="flex items-center gap-2.5 glass-light rounded-lg px-3 py-2 cursor-pointer hover:bg-s-hover transition"
            :class="selectedId === p.id ? 'ring-1 ring-blue-500/50' : ''">
            <input type="radio" :value="p.id" v-model="selectedId" class="accent-blue-500">
            <div class="flex-1 min-w-0">
              <div class="text-xs font-medium text-t-primary">{{ p.name || `${p.protocol}://${p.host}:${p.port}` }}</div>
              <div class="text-[10px] text-t-faint font-mono">{{ p.protocol }}://{{ p.username ? '***@' : '' }}{{ p.host }}:{{ p.port }}</div>
            </div>
            <button @click.prevent="deleteProxy(p.id)" class="text-t-faint hover:text-red-400 transition p-1" title="删除">
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
            </button>
          </label>
        </div>
      </div>

      <!-- 添加新代理 -->
      <details class="mb-4">
        <summary class="text-xs font-medium text-blue-400 cursor-pointer hover:text-blue-300 transition">+ 添加新代理</summary>
        <div class="mt-2 glass-light rounded-lg p-3 space-y-2">
          <div class="flex gap-2">
            <select v-model="form.protocol" class="bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs outline-none w-24">
              <option value="socks5">SOCKS5</option>
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
            </select>
            <input v-model="form.host" placeholder="地址（IP 或域名）" class="flex-1 bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs outline-none">
            <input v-model.number="form.port" type="number" placeholder="端口" class="w-20 bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs outline-none">
          </div>
          <div class="flex gap-2">
            <input v-model="form.username" placeholder="用户名（可选）" class="flex-1 bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs outline-none">
            <input v-model="form.password" type="password" placeholder="密码（可选）" class="flex-1 bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs outline-none">
          </div>
          <div class="flex gap-2">
            <input v-model="form.name" placeholder="备注名称（如：家宽-北京）" class="flex-1 bg-s-inset border border-b-panel rounded-lg px-2 py-1.5 text-xs outline-none">
            <button @click="testNewProxy" :disabled="testingNew || !form.host || !form.port"
              class="px-3 py-1.5 text-xs font-medium rounded-lg transition"
              :class="testingNew ? 'bg-gray-700 text-gray-400' : 'bg-blue-600 hover:bg-blue-500 text-white'">
              {{ testingNew ? '测试中...' : '测试' }}
            </button>
            <button @click="addProxy" :disabled="!form.host || !form.port"
              class="px-3 py-1.5 bg-emerald-600 hover:bg-emerald-500 disabled:bg-gray-700 disabled:text-gray-500 text-white text-xs font-medium rounded-lg transition">
              保存
            </button>
          </div>
          <div v-if="testResult" class="text-[11px] px-1" :class="testResult.ok ? 'text-emerald-400' : 'text-red-400'">
            {{ testResult.message }}
          </div>
        </div>
      </details>

      <!-- 测试已选代理 + 确认 -->
      <div v-if="proxyTestError" class="text-[11px] text-red-400 mb-2 px-1">{{ proxyTestError }}</div>
      <div class="flex gap-2">
        <button @click="$emit('close')"
          class="flex-1 py-2 text-sm font-medium text-t-secondary glass-light rounded-lg hover:bg-s-hover transition">
          取消
        </button>
        <button @click="confirmStart" :disabled="confirming"
          class="flex-1 py-2 text-sm font-semibold text-white rounded-lg transition"
          :class="confirming ? 'bg-gray-700 text-gray-400' : 'bg-gradient-to-r from-amber-500 to-orange-500 hover:from-amber-400 hover:to-orange-400'">
          {{ confirming ? '检测代理中...' : '确认并开始注册' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { proxyApi } from '../../api/client'

const props = defineProps<{
  platform?: string
}>()

const emit = defineEmits<{
  close: []
  confirm: [proxyId?: number]
}>()

const platformLabel = computed(() => {
  const labels: Record<string, string> = { kiro: 'Kiro', gemini: 'Gemini' }
  return labels[props.platform || ''] || 'Kiro'
})

const platformDesc = computed(() => {
  const descs: Record<string, string> = {
    kiro: 'Kiro（AWS）注册对 IP 质量要求较高',
    gemini: 'Gemini（Google）注册对 IP 质量要求较高',
  }
  return descs[props.platform || ''] || 'Kiro（AWS）注册对 IP 质量要求较高'
})

interface ProxyItem {
  id: number; name: string; protocol: string; host: string; port: number; username: string; password: string
}

const proxies = ref<ProxyItem[]>([])
const selectedId = ref(0)
const confirming = ref(false)
const proxyTestError = ref('')
const testingNew = ref(false)
const testResult = ref<{ ok: boolean; message: string } | null>(null)

const form = reactive({
  name: '', protocol: 'socks5', host: '', port: null as number | null, username: '', password: '',
})

onMounted(async () => {
  try {
    const { data } = await proxyApi.list()
    proxies.value = data
  } catch (e: any) {
    console.warn('加载代理列表失败:', e.message || e)
  }
})

async function addProxy() {
  if (!form.host || !form.port) return
  try {
    const { data } = await proxyApi.create({
      name: form.name, protocol: form.protocol, host: form.host, port: form.port,
      username: form.username || undefined, password: form.password || undefined,
    })
    proxies.value.unshift(data)
    selectedId.value = data.id
    form.name = ''; form.host = ''; form.port = null; form.username = ''; form.password = ''
    testResult.value = null
  } catch (e: any) {
    testResult.value = { ok: false, message: e.response?.data?.detail || '保存失败' }
  }
}

async function deleteProxy(id: number) {
  try {
    await proxyApi.delete(id)
    proxies.value = proxies.value.filter(p => p.id !== id)
    if (selectedId.value === id) selectedId.value = 0
  } catch (e: any) {
    testResult.value = { ok: false, message: e.response?.data?.detail || '删除失败' }
  }
}

async function testNewProxy() {
  if (!form.host || !form.port) return
  testingNew.value = true
  testResult.value = null
  try {
    const { data } = await proxyApi.test({
      protocol: form.protocol, host: form.host, port: form.port,
      username: form.username || undefined, password: form.password || undefined,
    })
    testResult.value = data
  } catch (e: any) {
    testResult.value = { ok: false, message: e.response?.data?.detail || '测试失败' }
  } finally {
    testingNew.value = false
  }
}

async function confirmStart() {
  proxyTestError.value = ''
  // 选了用户代理，先测试连通性
  if (selectedId.value > 0) {
    confirming.value = true
    try {
      const { data } = await proxyApi.test({ proxy_id: selectedId.value })
      if (!data.ok) {
        proxyTestError.value = `代理不可用: ${data.message}，请更换代理或使用系统默认`
        confirming.value = false
        return
      }
    } catch (e: any) {
      proxyTestError.value = '代理测试失败，请检查网络'
      confirming.value = false
      return
    }
  }
  confirming.value = false
  emit('confirm', selectedId.value > 0 ? selectedId.value : undefined)
}
</script>
