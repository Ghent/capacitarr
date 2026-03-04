import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref, computed, readonly, type Ref } from 'vue'

// Now import the composable under test (after all stubs are in place)
import { useEngineControl } from './useEngineControl'

// ---------------------------------------------------------------------------
// Mock Nuxt auto-imports before importing the composable under test
// ---------------------------------------------------------------------------

// useState mock — returns a ref that persists per key
const stateStore = new Map<string, Ref>()
function mockUseState<T>(key: string, init?: () => T): Ref<T> {
  if (!stateStore.has(key)) {
    stateStore.set(key, ref(init ? init() : undefined) as Ref)
  }
  return stateStore.get(key) as Ref<T>
}

// useApi mock — returns a mock fetch function
const mockApiFetch = vi.fn()
function mockUseApi() {
  return mockApiFetch
}

// useToast mock
const addToastSpy = vi.fn()
function mockUseToast() {
  return {
    toasts: ref([]),
    addToast: addToastSpy,
    removeToast: vi.fn()
  }
}

// Stub global Nuxt auto-imports
vi.stubGlobal('useState', mockUseState)
vi.stubGlobal('useApi', mockUseApi)
vi.stubGlobal('useToast', mockUseToast)

// Vue reactivity primitives are already available via import, but the composable
// uses them as auto-imports. Stub them globally so the module resolution works.
vi.stubGlobal('ref', ref)
vi.stubGlobal('computed', computed)
vi.stubGlobal('readonly', readonly)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useEngineControl', () => {
  beforeEach(() => {
    stateStore.clear()
    mockApiFetch.mockReset()
    addToastSpy.mockReset()
    vi.useFakeTimers()
    // Suppress expected console.error from error-handling code paths
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  // -------------------------------------------------------------------------
  // Initial state
  // -------------------------------------------------------------------------
  describe('initial state', () => {
    it('has null workerStats initially', () => {
      const ctrl = useEngineControl()
      expect(ctrl.workerStats.value).toBeNull()
    })

    it('has default computed values when workerStats is null', () => {
      const ctrl = useEngineControl()
      expect(ctrl.executionMode.value).toBe('dry-run')
      expect(ctrl.lastRunEpoch.value).toBe(0)
      expect(ctrl.lastRunEvaluated.value).toBe(0)
      expect(ctrl.lastRunFlagged.value).toBe(0)
      expect(ctrl.lastRunFreedBytes.value).toBe(0)
      expect(ctrl.queueDepth.value).toBe(0)
      expect(ctrl.isRunning.value).toBe(false)
      expect(ctrl.pollIntervalSeconds.value).toBe(300)
    })

    it('has loading flags set to false initially', () => {
      const ctrl = useEngineControl()
      expect(ctrl.runNowLoading.value).toBe(false)
      expect(ctrl.changingMode.value).toBe(false)
    })
  })

  // -------------------------------------------------------------------------
  // modeLabel
  // -------------------------------------------------------------------------
  describe('modeLabel', () => {
    it.each([
      ['auto', 'Auto'],
      ['approval', 'Approval'],
      ['dry-run', 'Dry-Run'],
      ['unknown', 'Dry-Run'],
      ['', 'Dry-Run']
    ])('modeLabel("%s") → "%s"', (mode, expected) => {
      const ctrl = useEngineControl()
      expect(ctrl.modeLabel(mode)).toBe(expected)
    })
  })

  // -------------------------------------------------------------------------
  // fetchStats
  // -------------------------------------------------------------------------
  describe('fetchStats', () => {
    it('populates worker stats from API response', async () => {
      const statsData = {
        executionMode: 'auto',
        lastRunEpoch: 1700000000,
        lastRunEvaluated: 150,
        lastRunFlagged: 5,
        lastRunFreedBytes: 1073741824,
        queueDepth: 3,
        isRunning: false,
        pollIntervalSeconds: 600
      }
      mockApiFetch.mockResolvedValueOnce(statsData)

      const ctrl = useEngineControl()
      await ctrl.fetchStats()

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/worker/stats')
      expect(ctrl.executionMode.value).toBe('auto')
      expect(ctrl.lastRunEpoch.value).toBe(1700000000)
      expect(ctrl.lastRunEvaluated.value).toBe(150)
      expect(ctrl.lastRunFlagged.value).toBe(5)
      expect(ctrl.lastRunFreedBytes.value).toBe(1073741824)
      expect(ctrl.queueDepth.value).toBe(3)
      expect(ctrl.isRunning.value).toBe(false)
      expect(ctrl.pollIntervalSeconds.value).toBe(600)
    })

    it('silently handles API errors', async () => {
      mockApiFetch.mockRejectedValueOnce(new Error('Network error'))

      const ctrl = useEngineControl()
      // Should not throw
      await expect(ctrl.fetchStats()).resolves.toBeUndefined()
    })

    it('detects run completion and shows toast', async () => {
      // First call: engine is running
      mockApiFetch.mockResolvedValueOnce({
        isRunning: true,
        executionMode: 'auto',
        lastRunEvaluated: 0,
        lastRunFlagged: 0
      })
      const ctrl = useEngineControl()
      await ctrl.fetchStats()
      expect(ctrl.isRunning.value).toBe(true)

      // Second call: engine has stopped
      mockApiFetch.mockResolvedValueOnce({
        isRunning: false,
        executionMode: 'auto',
        lastRunEvaluated: 200,
        lastRunFlagged: 10
      })
      await ctrl.fetchStats()
      expect(ctrl.isRunning.value).toBe(false)

      // Should have shown a completion toast
      expect(addToastSpy).toHaveBeenCalledWith(
        expect.stringContaining('Engine run complete'),
        'success'
      )
    })

    it('does not show toast when engine was already idle', async () => {
      // Both calls: engine is idle
      mockApiFetch.mockResolvedValueOnce({ isRunning: false, executionMode: 'dry-run' })
      const ctrl = useEngineControl()
      await ctrl.fetchStats()

      mockApiFetch.mockResolvedValueOnce({ isRunning: false, executionMode: 'dry-run' })
      await ctrl.fetchStats()

      expect(addToastSpy).not.toHaveBeenCalled()
    })
  })

  // -------------------------------------------------------------------------
  // setMode
  // -------------------------------------------------------------------------
  describe('setMode', () => {
    it('fetches preferences, PUTs new mode, refreshes stats, and toasts', async () => {
      const existingPrefs = { executionMode: 'dry-run', pollInterval: 300 }
      // 1st call: GET preferences
      mockApiFetch.mockResolvedValueOnce(existingPrefs)
      // 2nd call: PUT preferences
      mockApiFetch.mockResolvedValueOnce({})
      // 3rd call: fetchStats (inside setMode)
      mockApiFetch.mockResolvedValueOnce({
        executionMode: 'auto',
        isRunning: false
      })

      const ctrl = useEngineControl()
      await ctrl.setMode('auto')

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/preferences')
      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/preferences', {
        method: 'PUT',
        body: { ...existingPrefs, executionMode: 'auto' }
      })
      expect(addToastSpy).toHaveBeenCalledWith(
        'Execution mode set to Auto',
        'success'
      )
      expect(ctrl.changingMode.value).toBe(false)
    })

    it('sets changingMode to true during API call', async () => {
      let resolvePrefs: (value: unknown) => void
      const prefsPromise = new Promise((resolve) => {
        resolvePrefs = resolve
      })
      mockApiFetch.mockReturnValueOnce(prefsPromise)

      const ctrl = useEngineControl()
      const setModePromise = ctrl.setMode('approval')

      // changingMode should be true while waiting
      expect(ctrl.changingMode.value).toBe(true)

      // Resolve the chain
      resolvePrefs!({ executionMode: 'dry-run' })
      mockApiFetch.mockResolvedValueOnce({}) // PUT
      mockApiFetch.mockResolvedValueOnce({ executionMode: 'approval', isRunning: false }) // fetchStats
      await setModePromise

      expect(ctrl.changingMode.value).toBe(false)
    })

    it('shows error toast on failure and resets changingMode', async () => {
      mockApiFetch.mockRejectedValueOnce(new Error('Server error'))

      const ctrl = useEngineControl()
      await ctrl.setMode('auto')

      expect(addToastSpy).toHaveBeenCalledWith(
        'Failed to change execution mode',
        'error'
      )
      expect(ctrl.changingMode.value).toBe(false)
    })
  })

  // -------------------------------------------------------------------------
  // triggerRunNow
  // -------------------------------------------------------------------------
  describe('triggerRunNow', () => {
    it('POSTs to engine/run, waits, refreshes stats, and toasts', async () => {
      mockApiFetch.mockResolvedValueOnce({}) // POST engine/run
      mockApiFetch.mockResolvedValueOnce({ // fetchStats
        executionMode: 'auto',
        isRunning: true
      })

      const ctrl = useEngineControl()
      const promise = ctrl.triggerRunNow()

      // runNowLoading should be true immediately
      expect(ctrl.runNowLoading.value).toBe(true)

      // Advance past the 2000ms setTimeout
      await vi.advanceTimersByTimeAsync(2000)
      await promise

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/engine/run', { method: 'POST' })
      expect(addToastSpy).toHaveBeenCalledWith('Engine run triggered', 'info')
      expect(ctrl.runNowLoading.value).toBe(false)
    })

    it('shows error toast on failure and resets runNowLoading', async () => {
      mockApiFetch.mockRejectedValueOnce(new Error('Server error'))

      const ctrl = useEngineControl()
      await ctrl.triggerRunNow()

      expect(addToastSpy).toHaveBeenCalledWith(
        'Failed to trigger engine run',
        'error'
      )
      expect(ctrl.runNowLoading.value).toBe(false)
    })
  })
})
