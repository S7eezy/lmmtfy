<script>
  let { title, onclose, children, footer = null, wide = false } = $props()
  function key(e) { if (e.key === 'Escape') onclose?.() }
</script>

<svelte:window onkeydown={key} />
<div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && onclose?.()}>
  <div class="modal panel" class:wide role="dialog" aria-label={title}>
    <header><h2>{title}</h2><button class="link" onclick={onclose} aria-label="Close">✕</button></header>
    <div class="body">{@render children()}</div>
    {#if footer}<footer>{@render footer()}</footer>{/if}
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: rgb(0 0 0 / 0.35); display: grid; place-items: center; z-index: 10; }
  .modal { width: min(560px, 92vw); max-height: 88vh; display: flex; flex-direction: column; }
  .modal.wide { width: min(860px, 94vw); }
  header { display: flex; justify-content: space-between; align-items: center; padding: 14px 18px 0; }
  header h2 { margin: 0; }
  .body { padding: 12px 18px; overflow: auto; }
  footer { display: flex; justify-content: flex-end; gap: 8px; padding: 12px 18px; border-top: 1px solid var(--line); }
</style>
