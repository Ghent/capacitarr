export interface Toast {
  id: number;
  message: string;
  type: 'error' | 'success' | 'info';
  duration: number;
}

let nextId = 0;

export function useToast() {
  const toasts = useState<Toast[]>('toasts', () => []);

  function addToast(
    message: string,
    type: 'error' | 'success' | 'info' = 'info',
    duration?: number,
  ) {
    const defaultDuration = type === 'error' ? 8000 : type === 'success' ? 4000 : 5000;
    const toast: Toast = {
      id: nextId++,
      message,
      type,
      duration: duration ?? defaultDuration,
    };
    toasts.value.push(toast);

    // Auto-remove after duration
    setTimeout(() => {
      removeToast(toast.id);
    }, toast.duration);
  }

  function removeToast(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id);
  }

  return { toasts, addToast, removeToast };
}
