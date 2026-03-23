import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '../api/client'

export interface UserInfo {
  id: number
  username: string
  name: string
  email: string
  avatar_url: string
  trust_level: number
  role: number
  credits: number
  free_trial_used: boolean
  free_trial_remaining: number
  is_admin: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<UserInfo | null>(null)
  const token = ref(localStorage.getItem('token') || '')

  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.is_admin ?? false)

  function setToken(t: string) {
    token.value = t
    localStorage.setItem('token', t)
  }

  async function fetchMe() {
    try {
      const { data } = await authApi.me()
      user.value = data
    } catch {
      logout()
    }
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
  }

  return { user, token, isLoggedIn, isAdmin, setToken, fetchMe, logout }
})
