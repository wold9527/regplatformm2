/**
 * 任务引擎 — SSE 连接、任务启动/停止、计时器
 * 依赖 useDashboard 的共享状态
 */
import { nextTick, watch } from 'vue'
import { taskApi } from '../api/client'
import { useDashboard } from './useDashboard'

const SSE_MAX_RETRIES = 5
let sseSource: EventSource | null = null
let sseRetryCount = 0
let sseRetryTimer: ReturnType<typeof setTimeout> | null = null
let elapsedTimer: ReturnType<typeof setInterval> | null = null
let refreshDebounceTimer: ReturnType<typeof setTimeout> | null = null
let queuePollTimer: ReturnType<typeof setInterval> | null = null

export function useTaskEngine() {
  const d = useDashboard()

  // ── 计时器 ──
  function startElapsedTimer() {
    stopElapsedTimer()
    d.elapsedSeconds.value = 0
    elapsedTimer = setInterval(() => { d.elapsedSeconds.value++ }, 1000)
  }
  function stopElapsedTimer() {
    if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null }
  }

  // ── 防抖刷新（trailing-edge：最后一次触发后 2s 执行，保证最终状态被刷新）──
  function debouncedRefresh() {
    if (refreshDebounceTimer) clearTimeout(refreshDebounceTimer)
    refreshDebounceTimer = setTimeout(() => {
      refreshDebounceTimer = null
      d.loadResults()
      d.loadTxHistory()
      d.loadBalance()
    }, 2000)
  }

  // ── 排队轮询（每 5 秒检查任务是否已被调度启动）──
  function startQueuePolling(taskId: number) {
    stopQueuePolling()
    queuePollTimer = setInterval(async () => {
      try {
        const { data } = await taskApi.current(d.taskForm.platform)
        if (data && data.task_id === taskId && data.status === 'running') {
          stopQueuePolling()
          d.queueModal.show = false
          d.isQueued.value = false
          d.isRunning.value = true
          sseRetryCount = 0
          startElapsedTimer()
          connectSSE(taskId)
          const msg = `${d.platformLabel.value} 任务 #${taskId} 已启动`
          d.toast(msg, 'success')
          d.setTaskNotice(msg, 'success')
        } else if (data && data.task_id === taskId && (data.status === 'queued' || data.status === 'pending')) {
          // 更新排队位置和预计等待时间
          if (data.queue_position) d.queueModal.position = data.queue_position
          if (data.queue_wait_sec) {
            const min = Math.ceil(data.queue_wait_sec / 60)
            d.queueModal.waitTime = min >= 1 ? `约 ${min} 分钟` : '即将开始'
          }
        } else if (!data || data.task_id !== taskId || data.status === 'stopped' || data.status === 'completed') {
          // 任务已结束或被取消
          stopQueuePolling()
          d.queueModal.show = false
          d.isQueued.value = false
          d.loadBalance(); d.loadGlobalStats()
        }
      } catch { /* ignore */ }
    }, 5000)
  }
  function stopQueuePolling() {
    if (queuePollTimer) { clearInterval(queuePollTimer); queuePollTimer = null }
  }

  // ── SSE ──
  function connectSSE(taskId: number) {
    disconnectSSE()
    const token = localStorage.getItem('token')
    sseSource = new EventSource(`/ws/logs/${taskId}/stream?token=${token}`)
    sseSource.onmessage = (e) => {
      sseRetryCount = 0
      try {
        const data = JSON.parse(e.data)
        if (data.type === 'log') {
          // 收到日志说明任务已真正开始，关闭排队等待弹窗
          if (d.queueModal.show) d.queueModal.show = false
          d.lastLogAt.value = Date.now()
          const msg = data.message as string
          d.logs.value.push(msg)
          if (d.logs.value.length > 500) d.logs.value.splice(0, d.logs.value.length - 500)
          nextTick(() => { if (d.logPanel.value) d.logPanel.value.scrollTop = d.logPanel.value.scrollHeight })
        } else if (data.type === 'status') {
          const prevSuccess = d.taskStatus.success_count
          const prevFail = d.taskStatus.fail_count
          d.taskStatus.success_count = data.success
          d.taskStatus.fail_count = data.fail
          if (data.success > prevSuccess) {
            const delta = data.success - prevSuccess
            d.userStats.total_success += delta
            const p = d.taskForm.platform
            if (!d.byPlatformStats[p]) d.byPlatformStats[p] = { success: 0, fail: 0 }
            d.byPlatformStats[p].success += delta
          }
          if (data.fail > prevFail) {
            const delta = data.fail - prevFail
            d.userStats.total_fail += delta
            const p = d.taskForm.platform
            if (!d.byPlatformStats[p]) d.byPlatformStats[p] = { success: 0, fail: 0 }
            d.byPlatformStats[p].fail += delta
          }
          if (data.success > prevSuccess) debouncedRefresh()
        } else if (data.type === 'complete') {
          if (d.queueModal.show) d.queueModal.show = false
          d.taskStatus.success_count = data.success
          d.taskStatus.fail_count = data.fail
          d.taskStatus.is_done = true
          d.isRunning.value = false
          d.isQueued.value = false
          stopElapsedTimer()
          disconnectSSE()
          d.loadBalance(); d.loadResults(); d.loadTxHistory(); d.loadGlobalStats()
          const elapsedStr = data.elapsed ? ` 耗时 ${data.elapsed}` : ''
          const msg = `任务完成！成功 ${data.success} 个，失败 ${data.fail} 个${elapsedStr}`
          d.toast(msg, 'success')
          d.setTaskNotice(msg, 'success')
          d.playNotificationSound()
          d.showBrowserNotification('任务完成', msg)
          if (data.success > 0) d.sprayTaskCompleteRibbons()
          setTimeout(() => { d.loadBalance(); d.loadTxHistory() }, 3000)
          setTimeout(() => { if (d.taskStatus.is_done && !d.isRunning.value) d.clearTaskNotice() }, 5000)
        } else if (data.type === 'error' || data.type === 'info') {
          d.logs.value.push(data.message)
          if (data.type === 'error') {
            if (d.queueModal.show) d.queueModal.show = false
            d.taskStatus.is_done = true
            d.isRunning.value = false
            d.isQueued.value = false
            stopElapsedTimer()
            disconnectSSE()
            d.loadBalance(); d.loadResults(); d.loadTxHistory(); d.loadGlobalStats()
          }
        }
      } catch (err) { d.logs.value.push('[!] 日志解析异常: ' + (err as Error).message) }
    }
    sseSource.onerror = () => {
      // 防止已切换到新任务后，旧 SSE 的 onerror 仍触发重连
      if (d.taskStatus.task_id !== taskId) { disconnectSSE(); return }
      sseRetryCount++
      if (sseRetryCount >= SSE_MAX_RETRIES) {
        disconnectSSE()
        checkTaskAndRecover(taskId)
        return
      }
      // 清除已有定时器防重复连接
      if (sseRetryTimer) { clearTimeout(sseRetryTimer); sseRetryTimer = null }
      if (d.isRunning.value) {
        sseRetryTimer = setTimeout(() => { sseRetryTimer = null; if (d.isRunning.value && d.taskStatus.task_id === taskId) connectSSE(taskId) }, 3000)
      }
    }
  }

  function disconnectSSE() {
    if (sseRetryTimer) { clearTimeout(sseRetryTimer); sseRetryTimer = null }
    sseSource?.close()
    sseSource = null
  }

  async function checkTaskAndRecover(taskId: number) {
    try {
      const { data } = await taskApi.current(d.taskForm.platform)
      if (data && data.task_id === taskId && data.status === 'running') {
        sseRetryCount = 0; connectSSE(taskId); return
      }
      d.isRunning.value = false; d.isQueued.value = false; d.taskStatus.is_done = true
      if (data) {
        d.taskStatus.success_count = data.success_count || d.taskStatus.success_count
        d.taskStatus.fail_count = data.fail_count || d.taskStatus.fail_count
      }
      d.loadBalance(); d.loadResults(); d.loadTxHistory(); d.loadGlobalStats()
      d.toast('任务已结束', 'info')
      setTimeout(() => { d.loadBalance(); d.loadTxHistory() }, 3000)
    } catch {
      d.isRunning.value = false; d.isQueued.value = false; d.taskStatus.is_done = true
    }
  }

  // ── 任务操作 ──
  async function startTask(proxyId?: number) {
    if (d.processing.value) return
    d.processing.value = true
    try {
      const { data: task } = await taskApi.create({
        platform: d.taskForm.platform,
        target: d.taskForm.target_count,
        threads: 0,
        mode: d.currentPlatformFree.value ? d.taskMode.value : '',
        proxy_id: proxyId || undefined,
      })
      const { data: started } = await taskApi.start(task.task_id)
      const isQueued = started.message && (started.message.includes('队列') || started.message.includes('排队'))

      Object.assign(d.taskStatus, {
        task_id: task.task_id, platform: d.taskForm.platform,
        target: d.taskForm.target_count, threads: task.threads || d.taskForm.thread_count,
        credits_reserved: d.taskForm.target_count,
        success_count: 0, fail_count: 0, is_done: false,
      })
      d.logs.value = []
      sseRetryCount = 0

      if (isQueued) {
        // 排队中：不启动计时器/SSE，弹排队弹窗，后台轮询等待调度
        d.isQueued.value = true
        const posMatch = started.message.match(/前方\s*(\d+)/)
        const waitMatch = started.message.match(/预计等待\s*(.+)/)
        d.queueModal.position = posMatch ? parseInt(posMatch[1]) : 0
        d.queueModal.waitTime = waitMatch ? waitMatch[1] : ''
        d.queueModal.message = started.message
        d.queueModal.show = true
        d.setTaskNotice(started.message, 'info')
        startQueuePolling(task.task_id)
      } else {
        // 直接启动：正常流程
        d.isRunning.value = true
        startElapsedTimer()
        connectSSE(task.task_id)
        const msg = `${d.platformLabel.value} 任务 #${task.task_id} 已启动 (${d.taskForm.target_count}个 × ${d.taskForm.thread_count}线程)`
        d.toast(msg, 'success'); d.setTaskNotice(msg, 'success')
      }
      d.loadBalance(); d.loadTxHistory(); d.loadGlobalStats()
    } catch (e: any) {
      const errMsg = e.response?.data?.detail || '启动失败'
      d.toast(errMsg, 'error'); d.setTaskNotice(errMsg, 'error')
    } finally { d.processing.value = false }
  }

  async function stopTask() {
    if (!d.taskStatus.task_id || d.processing.value) return
    d.processing.value = true
    try {
      await taskApi.stop(d.taskStatus.task_id)
      disconnectSSE(); stopElapsedTimer(); stopQueuePolling()
      d.queueModal.show = false
      d.isQueued.value = false
      d.isRunning.value = false; d.taskStatus.is_done = true
      await d.loadBalance(); await d.loadResults(); await d.loadTxHistory()
      d.loadGlobalStats()
      d.toast('任务已停止', 'success')
      d.setTaskNotice(`任务 #${d.taskStatus.task_id} 已停止 — 成功 ${d.taskStatus.success_count} 个`, 'info')
      setTimeout(() => { d.loadBalance(); d.loadTxHistory() }, 3000)
    } catch (e: any) { d.toast(e.response?.data?.detail || '停止失败', 'error') }
    finally { d.processing.value = false }
  }

  async function loadCurrentTask() {
    try {
      const { data } = await taskApi.current()
      if (data && data.task_id) {
        Object.assign(d.taskStatus, data)
        if (data.platform) d.taskForm.platform = data.platform
        if (data.status === 'running' || data.status === 'stopping') {
          d.isRunning.value = true; sseRetryCount = 0
          startElapsedTimer(); connectSSE(data.task_id)
        } else if (data.status === 'queued' || data.status === 'pending') {
          d.isQueued.value = true
          // 页面恢复时不弹模态框，只静默轮询；任务卡片已显示"排队中"
          d.setTaskNotice('任务排队中，等待调度...', 'info')
          startQueuePolling(data.task_id)
        }
      }
    } catch { /* 页面加载时静默，resumeFromInit 是主恢复路径 */ }
  }

  /** 从 initLoad 返回的 current_task 恢复 SSE */
  function resumeFromInit(currentTask: any) {
    if (!currentTask?.task_id) return
    Object.assign(d.taskStatus, currentTask)
    if (currentTask.platform) d.taskForm.platform = currentTask.platform
    if (currentTask.status === 'running') {
      d.isRunning.value = true; sseRetryCount = 0
      startElapsedTimer(); connectSSE(currentTask.task_id)
    } else if (currentTask.status === 'stopping') {
      // stopping 视为运行中，等待引擎完成清理
      d.isRunning.value = true; sseRetryCount = 0
      startElapsedTimer(); connectSSE(currentTask.task_id)
    } else if (currentTask.status === 'queued' || currentTask.status === 'pending') {
      d.isQueued.value = true
      // 页面恢复时不弹模态框，只静默轮询；任务卡片已显示"排队中"
      d.setTaskNotice('任务排队中，等待调度...', 'info')
      startQueuePolling(currentTask.task_id)
    }
  }

  // ── Watchers ──
  /** 自动计算线程数 */
  function setupWatchers() {
    watch(() => d.taskForm.target_count, (target) => {
      if (d.isRunning.value || d.isQueued.value) return
      let threads: number | undefined
      if (d.limits.thread_tiers) {
        try {
          const tiers = typeof d.limits.thread_tiers === 'string' ? JSON.parse(d.limits.thread_tiers) : d.limits.thread_tiers
          if (Array.isArray(tiers) && tiers.length > 0) {
            for (const tier of tiers) { if (target <= tier.max) { threads = tier.threads; break } }
            if (threads === undefined) threads = tiers[tiers.length - 1].threads
          }
        } catch { /* 降级 */ }
      }
      if (threads === undefined) {
        if (target <= 5) threads = 1
        else if (target <= 20) threads = 2
        else if (target <= 100) threads = 3
        else if (target <= 300) threads = 5
        else if (target <= 500) threads = 8
        else threads = 12
      }
      if (d.limits.max_threads > 0 && threads > d.limits.max_threads) threads = d.limits.max_threads
      d.taskForm.thread_count = threads
    }, { immediate: true })

    watch(d.sliderMax, (max) => {
      if (d.taskForm.target_count > max) d.taskForm.target_count = max
    })
  }

  // ── 清理 ──
  function cleanup() {
    disconnectSSE(); stopElapsedTimer(); stopQueuePolling()
    d.isQueued.value = false
    if (refreshDebounceTimer) { clearTimeout(refreshDebounceTimer); refreshDebounceTimer = null }
  }

  return {
    startTask, stopTask, loadCurrentTask, resumeFromInit,
    connectSSE, disconnectSSE, startElapsedTimer, stopElapsedTimer,
    setupWatchers, cleanup,
  }
}
