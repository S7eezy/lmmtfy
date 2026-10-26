<script>
  // Step 2: target columns, their sources and types, with a live preview of
  // the converted output.
  import DataTable from './DataTable.svelte'
  import ColumnDetails from './ColumnDetails.svelte'
  import * as M from './mapping.js'

  let { mapping = $bindable(), sources = [], detection = null, preview = null, issues = [], hasInput = true } = $props()

  let selected = $state(0)
  let tab = $state('output')

  const cols = $derived(M.columns(mapping))
  const used = $derived(new Set(cols.map((c) => M.sourceKey(mapping.columns?.[c.name]))))
  const unused = $derived(sources.filter((s) => !used.has(s.key)))
  const errors = $derived(issues.filter((i) => i.severity === 'error'))
  const general = $derived(issues.filter((i) => !cols.some((c) => M.issuesFor([i], c.name).length)))
  const current = $derived(cols[Math.min(selected, cols.length - 1)])

  function detectedFor(r) {
    if (!detection || !r) return null
    return detection.columns.find((c) => (r.source_index ? c.index === r.source_index : c.name === r.source)) ?? null
  }
  function sourceOf(key) { return sources.find((s) => s.key === key) ?? null }
  function rowIssues(name) { return M.issuesFor(issues, name) }

  function addFrom(source) { selected = M.add(mapping, source) }
  function remove(i) {
    M.remove(mapping, i)
    selected = Math.max(0, Math.min(selected, cols.length - 1))
  }
  function move(i, d) {
    M.move(mapping, i, d)
    if (selected === i) selected = i + d
  }
</script>

<div class="layout">
  <section class="panel columns">
    <div class="head row">
      <strong>Output columns</strong>
      <span class="badge">{cols.length}</span>
      {#if errors.length}<span class="badge error">{errors.length} errors</span>{/if}
      <span class="spacer"></span>
      <button onclick={() => addFrom(null)}>+ Add column</button>
    </div>
    <div class="scroll list">
      <table>
        <thead><tr><th></th><th>Output name</th><th>Type</th><th>From source column</th><th></th></tr></thead>
        <tbody>
          {#each cols as col, i (col)}
            {@const r = mapping.columns?.[col.name]}
            {@const problems = rowIssues(col.name)}
            <tr class:selected={i === selected} onclick={() => (selected = i)}>
              <td class="order">
                <button class="icon" disabled={i === 0} onclick={(e) => { e.stopPropagation(); move(i, -1) }} aria-label="Move up">↑</button>
                <button class="icon" disabled={i === cols.length - 1} onclick={(e) => { e.stopPropagation(); move(i, 1) }} aria-label="Move down">↓</button>
              </td>
              <td><input value={col.name} onchange={(e) => { if (!M.rename(mapping, i, e.target.value.trim())) e.target.value = col.name }} /></td>
              <td>
                <select value={col.type} onchange={(e) => M.setType(mapping, i, e.target.value)}>
                  {#each M.TYPES as t}<option value={t}>{t}</option>{/each}
                </select>
              </td>
              <td>
                <select value={M.sourceKey(r)} onchange={(e) => M.setSource(mapping, col.name, sourceOf(e.target.value))}>
                  <option value="">— none (default value) —</option>
                  {#each sources as s}<option value={s.key}>{s.label}</option>{/each}
                  {#if M.sourceKey(r) && !sourceOf(M.sourceKey(r))}<option value={M.sourceKey(r)}>{M.sourceKey(r)} (not in this file)</option>{/if}
                </select>
              </td>
              <td class="status">
                {#if problems.some((p) => p.severity === 'error')}<span class="badge error" title={problems.map((p) => p.message).join('\n')}>!</span>
                {:else if problems.length}<span class="badge warn" title={problems.map((p) => p.message).join('\n')}>?</span>{/if}
                <button class="icon" onclick={(e) => { e.stopPropagation(); remove(i) }} aria-label="Delete column" title="Delete column">🗑</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
      {#if unused.length}
        <div class="unused">
          <span class="muted">Not in the output (click to add):</span>
          {#each unused as s}<button class="chip" onclick={() => addFrom(s)}>+ {s.label}</button>{/each}
        </div>
      {/if}
      {#each general as i}<div class="issue {i.severity}">{i.path ? i.path + ': ' : ''}{i.message}</div>{/each}
    </div>
  </section>

  <aside class="panel side scroll">
    {#if current && mapping.columns?.[current.name]}
      <ColumnDetails col={current} r={mapping.columns[current.name]} {issues} detected={detectedFor(mapping.columns[current.name])} />
    {:else if current}
      <div class="details">
        <p>This column has no rule yet.</p>
        <button onclick={() => (mapping.columns[current.name] = { default: '' })}>Fill it with a default value</button>
      </div>
    {:else}
      <p class="muted pad">Add a column to start.</p>
    {/if}
  </aside>

  <section class="panel bottom">
    <div class="tabs row">
      <button class:active={tab === 'output'} onclick={() => (tab = 'output')}>Output preview {#if preview?.rows}<span class="badge">{preview.rows.length}</span>{/if}</button>
      <button class:active={tab === 'rejected'} onclick={() => (tab = 'rejected')}>Rejected rows {#if preview?.rejected?.length}<span class="badge error">{preview.rejected.length - 1}</span>{/if}</button>
      <button class:active={tab === 'events'} onclick={() => (tab = 'events')}>Messages {#if preview?.events?.length}<span class="badge warn">{preview.events.length}</span>{/if}</button>
      <span class="spacer"></span>
      {#if preview?.failure}<span class="issue error">Conversion fails: {preview.failure}</span>{/if}
      {#if !hasInput}<span class="muted">Open a sample CSV file to see a preview.</span>{/if}
    </div>
    <div class="tabbody">
      {#if errors.length}
        <div class="pad">{#each errors as i}<div class="issue error">{i.path}: {i.message}</div>{/each}</div>
      {:else if tab === 'output'}
        <DataTable header={preview?.header} rows={preview?.rows ?? []} empty="No output rows" />
      {:else if tab === 'rejected'}
        <DataTable header={preview?.rejected?.[0]} rows={preview?.rejected?.slice(1) ?? []} rowClass={() => 'rejected'} empty="No rejected rows" />
      {:else}
        <div class="pad">
          {#each preview?.events ?? [] as e}
            <div class="issue {e.level === 'error' ? 'error' : 'warning'}">{#if e.line}line {e.line}{/if}{#if e.column}, {e.column}{/if}{#if e.line || e.column}: {/if}{e.message}</div>
          {:else}<p class="muted">No messages.</p>{/each}
        </div>
      {/if}
    </div>
  </section>
</div>

<style>
  .layout { display: grid; grid-template-columns: 1fr 380px; grid-template-rows: 1fr 260px; gap: 12px; height: 100%; min-height: 0; }
  .columns { display: flex; flex-direction: column; min-height: 0; min-width: 0; }
  .head { padding: 10px 12px; border-bottom: 1px solid var(--line); }
  .list { flex: 1; min-height: 0; padding-bottom: 8px; }
  table { width: 100%; border-collapse: collapse; }
  th { text-align: left; font-size: 11px; color: var(--muted); font-weight: 600; padding: 6px 8px; position: sticky; top: 0; background: var(--panel); }
  td { padding: 4px 6px; border-top: 1px solid var(--line); }
  td input, td select { width: 100%; }
  tr { cursor: pointer; }
  tr.selected td { background: var(--soft); }
  .order { width: 64px; white-space: nowrap; }
  .status { width: 70px; white-space: nowrap; text-align: right; }
  .unused { padding: 10px 12px; display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
  .chip { border-radius: 12px; padding: 2px 10px; font-size: 12px; }
  .side { grid-row: 1 / span 2; grid-column: 2; min-height: 0; }
  .bottom { display: flex; flex-direction: column; min-height: 0; min-width: 0; }
  .tabs { padding: 6px 8px; border-bottom: 1px solid var(--line); }
  .tabs button { border: none; background: none; }
  .tabs button.active { background: var(--soft); color: var(--accent); font-weight: 600; }
  .tabbody { flex: 1; min-height: 0; overflow: auto; }
  .pad, .details { padding: 12px 16px; }
</style>
