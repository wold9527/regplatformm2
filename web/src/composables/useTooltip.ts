/**
 * 全局 Tooltip 系统
 * position:fixed 挂在 body 上，不受 overflow 裁剪影响
 * 用法：在 onMounted 调用 setup()，在 onUnmounted 调用 destroy()
 */
export function useTooltip() {
  let tipEl: HTMLDivElement | null = null
  let tipArrow: HTMLDivElement | null = null
  let tipHideTimer: ReturnType<typeof setTimeout> | null = null

  function setup() {
    tipEl = document.createElement('div')
    tipEl.style.cssText = 'position:fixed;z-index:99999;pointer-events:none;opacity:0;transition:opacity 0.15s;padding:4px 8px;border-radius:6px;background:var(--tooltip-bg);border:1px solid var(--tooltip-border);color:var(--tooltip-text);font-size:10px;line-height:1.4;white-space:nowrap;max-width:280px;'
    tipArrow = document.createElement('div')
    tipArrow.style.cssText = 'position:fixed;z-index:99999;pointer-events:none;opacity:0;transition:opacity 0.15s;width:0;height:0;border:4px solid transparent;'
    document.body.appendChild(tipEl)
    document.body.appendChild(tipArrow)
    document.addEventListener('mouseover', onTipEnter, true)
    document.addEventListener('mouseout', onTipLeave, true)
  }

  function destroy() {
    if (tipHideTimer) { clearTimeout(tipHideTimer); tipHideTimer = null }
    document.removeEventListener('mouseover', onTipEnter, true)
    document.removeEventListener('mouseout', onTipLeave, true)
    tipEl?.remove()
    tipArrow?.remove()
    tipEl = null
    tipArrow = null
  }

  function onTipEnter(e: Event) {
    const target = (e.target as HTMLElement)?.closest?.('.tip[data-tip]') as HTMLElement | null
    if (!target || !tipEl || !tipArrow) return
    const text = target.getAttribute('data-tip')
    if (!text) return
    if (tipHideTimer) { clearTimeout(tipHideTimer); tipHideTimer = null }
    tipEl.textContent = text
    tipEl.style.opacity = '0'
    tipArrow.style.opacity = '0'
    requestAnimationFrame(() => {
      if (!tipEl || !tipArrow) return
      const rect = target.getBoundingClientRect()
      const tw = tipEl.offsetWidth
      const th = tipEl.offsetHeight
      const gap = 6
      const isBottom = target.classList.contains('tip-bottom')
      const isLeft = target.classList.contains('tip-left')
      const isRight = target.classList.contains('tip-right')
      let tx: number, ty: number, ax: number, ay: number, arrowBorder: string
      if (isBottom) {
        tx = rect.left + rect.width / 2 - tw / 2
        ty = rect.bottom + gap
        ax = rect.left + rect.width / 2 - 4
        ay = rect.bottom + gap - 8
        arrowBorder = 'border-color: transparent transparent var(--tooltip-bg) transparent'
      } else if (isLeft) {
        tx = rect.left - tw - gap
        ty = rect.top + rect.height / 2 - th / 2
        ax = rect.left - gap - 4
        ay = rect.top + rect.height / 2 - 4
        arrowBorder = 'border-color: transparent transparent transparent var(--tooltip-bg)'
      } else if (isRight) {
        tx = rect.right + gap
        ty = rect.top + rect.height / 2 - th / 2
        ax = rect.right + gap - 4
        ay = rect.top + rect.height / 2 - 4
        arrowBorder = 'border-color: transparent var(--tooltip-bg) transparent transparent'
      } else {
        tx = rect.left + rect.width / 2 - tw / 2
        ty = rect.top - th - gap
        ax = rect.left + rect.width / 2 - 4
        ay = rect.top - gap
        arrowBorder = 'border-color: var(--tooltip-bg) transparent transparent transparent'
      }
      const vw = window.innerWidth
      const vh = window.innerHeight
      if (tx < 4) tx = 4
      if (tx + tw > vw - 4) tx = vw - tw - 4
      if (ty < 4 && !isBottom) {
        ty = rect.bottom + gap
        ay = rect.bottom + gap - 8
        arrowBorder = 'border-color: transparent transparent var(--tooltip-bg) transparent'
      }
      if (ty + th > vh - 4) ty = vh - th - 4
      tipEl.style.left = tx + 'px'
      tipEl.style.top = ty + 'px'
      tipEl.style.opacity = '1'
      tipArrow.style.cssText = `position:fixed;width:0;height:0;border:4px solid transparent;pointer-events:none;z-index:100000;left:${ax}px;top:${ay}px;${arrowBorder};opacity:1`
    })
  }

  function onTipLeave(e: Event) {
    const target = (e.target as HTMLElement)?.closest?.('.tip[data-tip]') as HTMLElement | null
    if (!target) return
    tipHideTimer = setTimeout(() => {
      if (tipEl) tipEl.style.opacity = '0'
      if (tipArrow) tipArrow.style.opacity = '0'
    }, 50)
  }

  return { setup, destroy }
}
