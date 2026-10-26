<script>
  // A scrolling table of strings. rowClass(i) names a CSS class per row.
  let { header = null, rows = [], rowClass = () => '', lineNumbers = null, empty = 'No rows' } = $props()
</script>

<div class="wrap">
  {#if rows.length === 0 && !header}
    <p class="muted empty">{empty}</p>
  {:else}
    <table>
      {#if header}
        <thead>
          <tr>
            {#if lineNumbers}<th class="n">line</th>{/if}
            {#each header as h}<th>{h}</th>{/each}
          </tr>
        </thead>
      {/if}
      <tbody>
        {#each rows as row, i}
          <tr class={rowClass(i)}>
            {#if lineNumbers}<td class="n">{lineNumbers[i]}</td>{/if}
            {#each row as cell}<td title={cell}>{cell}</td>{/each}
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
</style>
