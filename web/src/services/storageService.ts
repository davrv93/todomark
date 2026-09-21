export function saveState<T>(key: string, data: T): void {
  try {
    localStorage.setItem(key, JSON.stringify(data));
  } catch (e) {
    console.error('saveState error', e);
  }
}

export function loadState<T>(key: string): T | null {
  try {
    const raw = localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch (e) {
    console.error('loadState error', e);
    return null;
  }
}

export function clearState(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch (e) {
    console.error('clearState error', e);
  }
}

export function syncUrlWithState(params: Record<string, string>): void {
  const search = new URLSearchParams(params).toString();
  const url = `${window.location.pathname}?${search}`;
  window.history.replaceState(null, '', url);
}
