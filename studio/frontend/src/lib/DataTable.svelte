<script>
  // A scrolling table of strings. rowClass(i) names a CSS class per row.
  // action(i), when given, renders a cell of controls at the start of row i.
  let { header = null, rows = [], rowClass = () => '', lineNumbers = null, empty = 'No rows', highlight = -1, action = null } = $props()

  // Bring the highlighted column into view when it changes.
  let wrap = $state(null)
  $effect(() => {
    if (highlight < 0 || !wrap) return
    wrap.querySelector('th.hl')?.scrollIntoView({ block: 'nearest', inline: 'center', behavior: 'smooth' })
  })
</script>

<div class="wrap" bind:this={wrap}>
  {#if rows.length === 0 && !header}
    <p class="muted empty">{empty}</p>
  {:else}
    <table>
      {#if header}
        <thead>
          <tr>
            {#if action}<th class="act"></th>{/if}
            {#if lineNumbers}<th class="n">line</th>{/if}
            {#each header as h, c}<th class:hl={c === highlight}>{h}</th>{/each}
          </tr>
        </thead>
      {/if}
      <tbody>
        {#each rows as row, i}
          <tr class={rowClass(i)}>
            {#if action}<td class="act">{@render action(i)}</td>{/if}
            {#if lineNumbers}<td class="n">{lineNumbers[i]}</td>{/if}
            {#each row as cell, c}<td class:hl={c === highlight} title={cell}>{cell}</td>{/each}
          </tr>
        {/each}
      </tbody>
    </table>
    {#if rows.length === 0}<p class="muted empty">{empty}</p>{/if}
  {/if}
</div>

<style>
  .wrap { overflow: auto; height: 100%; }
  table { border-collapse: collapse; font-family: var(--mono); font-size: 12px; white-space: pre; }
  th, td { border-bottom: 1px solid var(--line); padding: 3px 9px; text-align: left; max-width: 280px; overflow: hidden; text-overflow: ellipsis; }
  th { position: sticky; top: 0; background: var(--panel); font-weight: 600; z-index: 1; }
  td.n, th.n { color: var(--muted); text-align: right; }
  tr.skipped td, tr.footer td { color: var(--muted); text-decoration: line-through; }
  tr.header td { font-weight: 700; background: var(--soft); }
  tr.rejected td { color: var(--error); }
  .empty { padding: 12px; }
  th.act, td.act { width: 1px; padding: 0 2px 0 6px; position: sticky; left: 0; background: var(--panel); z-index: 1; }
  th.act { z-index: 2; }
  th.hl { background: var(--accent); color: var(--accent-text); }
  td.hl { background: var(--soft); }
</style>
