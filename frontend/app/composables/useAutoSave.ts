import type { PreferenceSet } from '~/types/api';
import { toast } from 'vue-sonner';

export function useAutoSave() {
  const api = useApi();

  const saveStatus = reactive<Record<string, 'idle' | 'saving' | 'saved' | 'error'>>({});
  const saveTimers: Record<string, ReturnType<typeof setTimeout>> = {};

  // Pending field overrides queued while a save is in-flight. When the current
  // save finishes, all pending overrides are merged into a single follow-up
  // save. This prevents concurrent GET→PUT cycles from clobbering each other's
  // values — the classic last-writer-wins race condition.
  let saving = false;
  let pendingOverrides = new Map<string, { field: string; value: string | number | boolean }>();

  function initFields(fields: string[]) {
    for (const f of fields) {
      saveStatus[f] = 'idle';
    }
  }

  function showSaveStatus(field: string, status: 'saving' | 'saved' | 'error') {
    saveStatus[field] = status;
    if (status === 'saved') {
      if (saveTimers[field]) clearTimeout(saveTimers[field]);
      saveTimers[field] = setTimeout(() => {
        saveStatus[field] = 'idle';
      }, 2000);
    }
  }

  async function autoSavePreference(field: string, key: string, value: string | number | boolean) {
    // If a save is already in-flight, queue this override and return.
    // It will be merged into the next save when the current one finishes.
    if (saving) {
      pendingOverrides.set(key, { field, value });
      showSaveStatus(field, 'saving');
      return;
    }

    saving = true;
    showSaveStatus(field, 'saving');
    try {
      const currentPrefs = await api.GET('/preferences');
      // Merge any overrides that were queued while we were fetching
      const body = { ...currentPrefs, [key]: value } as PreferenceSet;
      const snapshot = new Map(pendingOverrides);
      for (const [k, v] of snapshot) {
        (body as Record<string, string | number | boolean>)[k] = v.value;
      }

      await api.PUT('/preferences', { body });
      showSaveStatus(field, 'saved');
      for (const v of snapshot.values()) showSaveStatus(v.field, 'saved');
    } catch {
      showSaveStatus(field, 'error');
      for (const v of pendingOverrides.values()) showSaveStatus(v.field, 'error');
      toast.error(`Failed to save ${field} setting`);
    } finally {
      pendingOverrides = new Map();
      saving = false;
    }
  }

  async function patchPreference(
    field: string,
    group: 'engine' | 'sunset' | 'content' | 'advanced',
    key: string,
    value: string | number | boolean,
  ) {
    showSaveStatus(field, 'saving');
    try {
      const path = `/preferences/${group}` as
        | '/preferences/engine'
        | '/preferences/sunset'
        | '/preferences/content'
        | '/preferences/advanced';
      await api.PATCH(path, { body: { [key]: value } as never });
      showSaveStatus(field, 'saved');
    } catch {
      showSaveStatus(field, 'error');
      toast.error(`Failed to save ${field} setting`);
    }
  }

  return { saveStatus, initFields, autoSavePreference, patchPreference };
}
