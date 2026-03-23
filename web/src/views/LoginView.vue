<template>
  <div class="login-bg flex items-center justify-center min-h-screen">
    <div class="glass rounded-2xl p-10 max-w-md w-full mx-4">
      <div class="mb-8 text-center">
        <h1 class="text-4xl font-bold text-t-primary mb-2">多平台注册系统</h1>
        <p class="text-t-secondary">支持 Grok / OpenAI / Kiro 多平台批量注册</p>
      </div>

      <!-- 登录 / 注册 切换 -->
      <div class="flex mb-6 bg-bg-card rounded-lg p-1">
        <button
          @click="mode = 'login'"
          :class="[
            'flex-1 py-2 rounded-md text-sm font-medium transition-all',
            mode === 'login' ? 'bg-accent text-white shadow-sm' : 'text-t-secondary hover:text-t-primary'
          ]"
        >登录</button>
        <button
          @click="mode = 'register'"
          :class="[
            'flex-1 py-2 rounded-md text-sm font-medium transition-all',
            mode === 'register' ? 'bg-accent text-white shadow-sm' : 'text-t-secondary hover:text-t-primary'
          ]"
        >注册</button>
      </div>

      <form @submit.prevent="handleSubmit" class="space-y-4">
        <div>
          <label class="block text-t-secondary text-sm mb-1.5">用户名</label>
          <input
            v-model="username"
            type="text"
            autocomplete="username"
            class="w-full px-4 py-2.5 rounded-lg bg-bg-card border border-border text-t-primary placeholder-t-muted focus:outline-none focus:border-accent transition-colors"
            placeholder="请输入用户名"
            required
          />
        </div>
        <div>
          <label class="block text-t-secondary text-sm mb-1.5">密码</label>
          <input
            v-model="password"
            type="password"
            autocomplete="current-password"
            class="w-full px-4 py-2.5 rounded-lg bg-bg-card border border-border text-t-primary placeholder-t-muted focus:outline-none focus:border-accent transition-colors"
            placeholder="请输入密码"
            required
          />
        </div>
        <div v-if="mode === 'register'">
          <label class="block text-t-secondary text-sm mb-1.5">确认密码</label>
          <input
            v-model="confirmPassword"
            type="password"
            autocomplete="new-password"
            class="w-full px-4 py-2.5 rounded-lg bg-bg-card border border-border text-t-primary placeholder-t-muted focus:outline-none focus:border-accent transition-colors"
            placeholder="请再次输入密码"
            required
          />
        </div>

        <!-- 错误提示 -->
        <div v-if="errorMsg" class="text-err text-sm bg-err-dim border border-red-500/20 rounded-lg px-4 py-2">
          {{ errorMsg }}
        </div>

        <!-- 成功提示 -->
        <div v-if="successMsg" class="text-green-400 text-sm bg-green-500/10 border border-green-500/20 rounded-lg px-4 py-2">
          {{ successMsg }}
        </div>

        <button
          type="submit"
          :disabled="loading"
          class="w-full py-2.5 rounded-lg bg-accent hover:bg-accent/90 text-white font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <span v-if="loading">处理中...</span>
          <span v-else>{{ mode === 'login' ? '登录' : '注册' }}</span>
        </button>
      </form>

      <p class="mt-4 text-center text-t-muted text-xs">
        {{ mode === 'login' ? '没有账号？' : '已有账号？' }}
        <button @click="toggleMode" class="text-accent hover:underline">
          {{ mode === 'login' ? '立即注册' : '立即登录' }}
        </button>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { authApi } from '../api/client'

const router = useRouter()
const auth = useAuthStore()

const mode = ref<'login' | 'register'>('login')
const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const errorMsg = ref('')
const successMsg = ref('')
const loading = ref(false)

function toggleMode() {
  mode.value = mode.value === 'login' ? 'register' : 'login'
  errorMsg.value = ''
  successMsg.value = ''
}

async function handleSubmit() {
  errorMsg.value = ''
  successMsg.value = ''

  if (mode.value === 'register' && password.value !== confirmPassword.value) {
    errorMsg.value = '两次输入的密码不一致'
    return
  }

  loading.value = true
  try {
    if (mode.value === 'register') {
      const { data } = await authApi.register(username.value, password.value)
      auth.setToken(data.token)
      await auth.fetchMe()
      router.push('/dashboard')
    } else {
      const { data } = await authApi.login(username.value, password.value)
      auth.setToken(data.token)
      await auth.fetchMe()
      router.push('/dashboard')
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.detail || '操作失败，请重试'
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  // 已登录则跳转
  if (auth.token) {
    try {
      await auth.fetchMe()
      if (auth.user) {
        router.push('/dashboard')
      }
    } catch { /* 未登录，留在登录页 */ }
  }
})
</script>

<style scoped>
.login-bg {
  background: linear-gradient(135deg, var(--bg-page) 0%, var(--bg-glass) 100%);
  min-height: 100vh;
  min-height: 100dvh;
}
</style>
