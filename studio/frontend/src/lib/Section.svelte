<script>
  // A titled group of settings with an icon and an optional explanation.
  let { icon: Icon = null, title, hint = '', children, actions = null } = $props()
</script>

<section class="section">
  <header>
    {#if Icon}<span class="icon"><Icon size={16} /></span>{/if}
    <h3>{title}</h3>
    {#if actions}<span class="actions">{@render actions()}</span>{/if}
  </header>
  {#if hint}<p class="hint">{hint}</p>{/if}
  {@render children()}
</section>

<style>
  .section { padding: 12px 0 14px; border-bottom: 1px solid var(--line); }
  .section:last-child { border-bottom: none; }
  header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
  .icon { display: inline-flex; width: 26px; height: 26px; border-radius: 7px; align-items: center; justify-content: center; background: var(--soft); color: var(--accent); }
  h3 { margin: 0; font-size: 13px; text-transform: none; letter-spacing: 0; color: var(--text); }
  .actions { margin-left: auto; }
</style>
