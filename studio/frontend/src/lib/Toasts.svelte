<script>
  import { CircleCheck, CircleAlert, Info, X } from '@lucide/svelte'
  import { toasts, dismiss } from './toast.svelte.js'
  const icons = { ok: CircleCheck, error: CircleAlert, info: Info }
</script>

<div class="toasts" aria-live="polite">
  {#each toasts as t (t.id)}
    {@const Icon = icons[t.kind] ?? Info}
    <div class="toast {t.kind}" role={t.kind === 'error' ? 'alert' : 'status'}>
      <Icon size={17} />
      <span>{t.text}</span>
      <button class="link" onclick={() => dismiss(t.id)} aria-label="Dismiss"><X size={14} /></button>
    </div>
  {/each}
</div>

<style>
  .toasts { position: fixed; right: 16px; bottom: 16px; display: grid; gap: 8px; z-index: 30; max-width: min(460px, 90vw); }
  .toast { display: flex; gap: 9px; align-items: flex-start; padding: 10px 12px; border-radius: 9px; background: var(--panel); border: 1px solid var(--line); box-shadow: 0 6px 22px rgb(0 0 0 / 0.18); animation: in 0.18s ease-out; }
  .toast span { flex: 1; word-break: break-word; }
  .toast.ok { border-left: 4px solid var(--high); }
  .toast.ok :global(svg:first-child) { color: var(--high); }
  .toast.error { border-left: 4px solid var(--error); }
  .toast.error :global(svg:first-child) { color: var(--error); }
  .toast.info :global(svg:first-child) { color: var(--accent); }
  @keyframes in { from { transform: translateY(8px); opacity: 0; } }
</style>
