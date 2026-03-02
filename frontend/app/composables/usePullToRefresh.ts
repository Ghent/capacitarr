/**
 * Pull-to-refresh composable for touch devices.
 *
 * Detects a pull-down gesture when the user is scrolled to the top
 * of the page, shows a visual indicator, and triggers a callback on release.
 *
 * Usage:
 *   const { isRefreshing, pullProgress } = usePullToRefresh(async () => {
 *     await fetchData()
 *   })
 */

const PULL_THRESHOLD = 80 // px of pull needed to trigger refresh
const MAX_PULL = 120 // max visual displacement

export function usePullToRefresh(onRefresh: () => Promise<void>) {
  const isRefreshing = ref(false)
  const pullDistance = ref(0)
  const pullProgress = computed(() => Math.min(pullDistance.value / PULL_THRESHOLD, 1))
  const isPulling = ref(false)

  let startY = 0
  let currentY = 0

  function onTouchStart(e: TouchEvent) {
    // Only activate when scrolled to the top
    if (window.scrollY > 5) return
    if (isRefreshing.value) return
    startY = e.touches[0]!.clientY
    isPulling.value = true
  }

  function onTouchMove(e: TouchEvent) {
    if (!isPulling.value) return
    currentY = e.touches[0]!.clientY
    const delta = currentY - startY

    // Only pull down (positive delta)
    if (delta <= 0) {
      pullDistance.value = 0
      return
    }

    // Apply rubber-band effect (diminishing returns)
    pullDistance.value = Math.min(delta * 0.5, MAX_PULL)

    // Prevent default scroll when pulling
    if (pullDistance.value > 10) {
      e.preventDefault()
    }
  }

  async function onTouchEnd() {
    if (!isPulling.value) return
    isPulling.value = false

    if (pullDistance.value >= PULL_THRESHOLD && !isRefreshing.value) {
      isRefreshing.value = true
      pullDistance.value = PULL_THRESHOLD * 0.5 // Hold at half-threshold during refresh

      try {
        await onRefresh()
      } finally {
        isRefreshing.value = false
        pullDistance.value = 0
      }
    } else {
      pullDistance.value = 0
    }
  }

  onMounted(() => {
    document.addEventListener('touchstart', onTouchStart, { passive: true })
    document.addEventListener('touchmove', onTouchMove, { passive: false })
    document.addEventListener('touchend', onTouchEnd)
  })

  onUnmounted(() => {
    document.removeEventListener('touchstart', onTouchStart)
    document.removeEventListener('touchmove', onTouchMove)
    document.removeEventListener('touchend', onTouchEnd)
  })

  return {
    isRefreshing: readonly(isRefreshing),
    pullProgress: readonly(pullProgress),
    pullDistance: readonly(pullDistance)
  }
}
