// Toast notifications: short messages in a corner (ok, info, warn, error).
// They disappear by themselves, warnings more slowly; errors stay until
// dismissed.
export const toasts = $state([])
let next = 0

export function toast(kind, text) {
  const t = { id: ++next, kind, text }
  toasts.push(t)
  if (kind !== 'error') setTimeout(() => dismiss(t.id), kind === 'warn' ? 8000 : 4000)
}

export function dismiss(id) {
  const i = toasts.findIndex((t) => t.id === id)
  if (i >= 0) toasts.splice(i, 1)
}
