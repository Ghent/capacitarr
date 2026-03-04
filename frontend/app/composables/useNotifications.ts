import type { InAppNotification } from '~/types/api'

/**
 * Composable for in-app notification management.
 * Provides unread count polling, notification list, and read/mark-all/clear helpers.
 */
export function useNotifications() {
  const api = useApi()
  const unreadCount = useState<number>('notif-unread', () => 0)
  const notifications = useState<InAppNotification[]>('notif-list', () => [])
  const loading = useState<boolean>('notif-loading', () => false)

  let pollTimer: ReturnType<typeof setInterval> | null = null

  /** Fetch unread count from the API */
  async function fetchUnreadCount() {
    try {
      const res = await api('/api/v1/notifications/unread-count') as { count: number }
      unreadCount.value = res.count
    } catch (e) {
      // Silently fail — badge just stays at last known value
      console.warn('[useNotifications] fetchUnreadCount failed:', e)
    }
  }

  /** Fetch recent in-app notifications (newest first, max 20) */
  async function fetchNotifications() {
    loading.value = true
    try {
      notifications.value = await api('/api/v1/notifications') as InAppNotification[]
    } catch (e) {
      // Silently fail — list stays at last known state
      console.warn('[useNotifications] fetchNotifications failed:', e)
    } finally {
      loading.value = false
    }
  }

  /** Mark a single notification as read */
  async function markAsRead(id: number) {
    try {
      await api(`/api/v1/notifications/${id}/read`, { method: 'PUT' })
      // Update local state
      const notif = notifications.value.find(n => n.id === id)
      if (notif) notif.read = true
      unreadCount.value = Math.max(0, unreadCount.value - 1)
    } catch (e) {
      // Silently fail — mark-as-read is non-critical
      console.warn('[useNotifications] markAsRead failed:', e)
    }
  }

  /** Mark all notifications as read */
  async function markAllAsRead() {
    try {
      await api('/api/v1/notifications/read-all', { method: 'PUT' })
      notifications.value.forEach((n) => {
        n.read = true
      })
      unreadCount.value = 0
    } catch (e) {
      // Silently fail — mark-all-as-read is non-critical
      console.warn('[useNotifications] markAllAsRead failed:', e)
    }
  }

  /** Delete all in-app notifications */
  async function clearAll() {
    try {
      await api('/api/v1/notifications', { method: 'DELETE' })
      notifications.value = []
      unreadCount.value = 0
    } catch (e) {
      // Silently fail — clearAll is non-critical
      console.warn('[useNotifications] clearAll failed:', e)
    }
  }

  /** Start polling unread count every 30 seconds */
  function startPolling() {
    stopPolling()
    fetchUnreadCount()
    pollTimer = setInterval(fetchUnreadCount, 30_000)
  }

  /** Stop polling */
  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  return {
    unreadCount,
    notifications,
    loading,
    fetchUnreadCount,
    fetchNotifications,
    markAsRead,
    markAllAsRead,
    clearAll,
    startPolling,
    stopPolling
  }
}
