import type { PreferenceSet } from '~/types/api';

export function useAutoSave() {
  const api = useApi();
  const { addToast } = useToast();

  const saveStatus = reactive<Record<string, 'idle' | 'saving' | 'saved' | 'error'>>({});
  const saveTimers: Record<string, ReturnType<typeof setTimeout>> = {};

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
    showSaveStatus(field, 'saving');
    try {
      const currentPrefs = (await api('/api/v1/preferences')) as PreferenceSet;
      await api('/api/v1/preferences', {
        method: 'PUT',
        body: { ...currentPrefs, [key]: value },
      });
      showSaveStatus(field, 'saved');
    } catch {
      showSaveStatus(field, 'error');
      addToast(`Failed to save ${field} setting`, 'error');
    }
  }

  return { saveStatus, initFields, showSaveStatus, autoSavePreference };
}
