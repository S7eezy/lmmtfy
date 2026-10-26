<script>
  // Step 2: target columns, their sources and types, with a live preview of
  // the converted output.
  //
  // Built for wide files (hundreds of columns): rows are reordered by
  // dragging their grip, the list scrolls by itself near its edges while
  // dragging, a filter finds columns, and only the selected row carries the
  // heavy source picker (one <select> of every source column per row would
  // mean tens of thousands of DOM nodes).
  import DataTable from './DataTable.svelte'
  import ColumnDetails from './ColumnDetails.svelte'
  import * as M from './mapping.js'

  let { mapping = $bindable(), sources = [], detection = null, preview = null, issues = [], hasInput = true } = $props()

  let selected = $state(0)
  let tab = $state('output')
  let filter = $state('')

  const cols = $derived(M.columns(mapping))
  const used = $derived(new Set(cols.map((c) => M.sourceKey(mapping.columns?.[c.name]))))
  const unused = $derived(sources.filter((s) => !used.has(s.key)))
  const errors = $derived(issues.filter((i) => i.severity === 'error'))
  const byColumn = $derived.by(() => {
    const map = new Map()
    for (const c of cols) map.set(c.name, M.issuesFor(issues, c.name))
    return map
  })
  const general = $derived(issues.filter((i) => !cols.some((c) => byColumn.get(c.name)?.includes(i))))
  const current = $derived(cols[Math.min(selected, cols.length - 1)])
  const visible = $derived.by(() => {
    const all = cols.map((col, i) => ({ col, i }))
    const q = filter.trim().toLowerCase()
    if (!q) return all
    return all.filter(({ col }) => col.name.toLowerCase().includes(q) ||
      M.sourceKey(mapping.columns?.[col.name]).toLowerCase().includes(q))
  })

  function detectedFor(r) {
    if (!detection || !r) return null
    return detection.columns.find((c) => (r.source_index ? c.index === r.source_index : c.name === r.source)) ?? null
  }
  function sourceOf(key) { return sources.find((s) => s.key === key) ?? null }
  function sourceLabel(r) {
    const key = M.sourceKey(r)
    if (!key) return '— none (default value) —'
    return sourceOf(key) ? key : `${key} (not in this file)`
  }

  function addFrom(source) {
    selected = M.add(mapping, source)
    filter = ''
    requestAnimationFrame(() => listEl?.querySelector('tr.selected')?.scrollIntoView({ block: 'nearest' }))
  }
  function remove(i) {
    M.remove(mapping, i)
    selected = Math.max(0, Math.min(selected, cols.length - 1))
  }

  // Drag and drop, with pointer events: they behave the same in every web
  // view, and do not interfere with dropping files on the window.
  let listEl = $state(null)
  let bodyEl = $state(null)
  let drag = $state(null) // { from, to, x, y }
  let frame = 0

  function startDrag(e, i) {
    if (filter || e.button !== 0) return
    e.preventDefault()
    try {
      e.currentTarget.setPointerCapture(e.pointerId) // keep receiving moves outside the grip
    } catch {}
    drag = { from: i, to: i, x: e.clientX, y: e.clientY }
    selected = i
    lastTime = 0
    frame = requestAnimationFrame(autoScroll)
  }
  function moveDrag(e) {
    if (!drag) return
    drag.x = e.clientX
    drag.y = e.clientY
    updateTarget()
  }
  function endDrag() {
    if (!drag) return
    const { from, to } = drag
    drag = null
    cancelAnimationFrame(frame)
    if (from !== to) {
      M.moveTo(mapping, from, to)
      selected = to
    }
  }
  function cancelDrag(e) {
    if (e.key === 'Escape' && drag) {
      drag = null
      cancelAnimationFrame(frame)
    }
  }
  // Rows all have the same height: the row under the pointer is found by
  // arithmetic, not by measuring hundreds of rows.
  function updateTarget() {
    const first = bodyEl?.rows[0]
    if (!first) return
    const top = first.getBoundingClientRect().top
    const h = first.offsetHeight || 1
    drag.to = Math.max(0, Math.min(cols.length - 1, Math.floor((drag.y - top) / h)))
  }
  // Scrolls while the pointer is near the top or bottom edge, faster the
  // closer it gets (up to 1500 px/s). Speed depends on elapsed time, not on
  // the frame rate, so slow machines scroll as fast as quick ones.
  let lastTime = 0
  function autoScroll(now = performance.now()) {
    if (!drag || !listEl) return
    const dt = lastTime ? Math.min(now - lastTime, 100) / 1000 : 0
    lastTime = now
    const r = listEl.getBoundingClientRect()
    const edge = 56
    const depth = drag.y < r.top + edge ? -(r.top + edge - drag.y) : drag.y > r.bottom - edge ? drag.y - (r.bottom - edge) : 0
    if (depth) {
      listEl.scrollTop += Math.sign(depth) * Math.min(1, Math.abs(depth) / edge) * 1500 * dt
      updateTarget()
    }
    frame = requestAnimationFrame(autoScroll)
  }
  // Keyboard: arrow keys on a focused grip move its row.
  function gripKey(e, i) {
    const d = e.key === 'ArrowUp' ? -1 : e.key === 'ArrowDown' ? 1 : 0
    if (!d || filter) return
    e.preventDefault()
    const to = Math.max(0, Math.min(cols.length - 1, i + d))
    M.moveTo(mapping, i, to)
    selected = to
    requestAnimationFrame(() => bodyEl?.rows[to]?.querySelector('.grip')?.focus())
  }
  function dropClass(i) {
    if (!drag || drag.from === drag.to) return ''
    if (i === drag.to) return drag.to > drag.from ? 'drop-below' : 'drop-above'
    return ''
  }
</script>

<svelte:window onkeydown={cancelDrag} />

<div class="layout" class:dragging={!!drag}>
  <section class="panel columns">
    <div class="head row">
      <strong>Output columns</strong>
      <span class="badge">{filter ? `${visible.length} of ${cols.length}` : cols.length}</span>
      {#if errors.length}<span class="badge error">{errors.length} errors</span>{/if}
      <input class="filter" type="search" bind:value={filter} placeholder="Filter columns…" />
      <span class="spacer"></span>
      <button onclick={() => addFrom(null)}>+ Add column</button>
    </div>
    <div class="scroll list" bind:this={listEl}>
      <table>
        <thead><tr><th class="grip-col"></th><th class="num">#</th><th>Output name</th><th>Type</th><th>From source column</th><th></th></tr></thead>
        <tbody bind:this={bodyEl}>
          {#each visible as { col, i } (col)}
            {@const r = mapping.columns?.[col.name]}
            {@const problems = byColumn.get(col.name) ?? []}
            <tr class="{dropClass(i)}" class:selected={i === selected} class:moving={drag?.from === i} onclick={() => (selected = i)}>
              <td class="grip-col">
                <button class="grip" class:disabled={!!filter} tabindex={filter ? -1 : 0}
                  title={filter ? 'Clear the filter to reorder' : 'Drag to reorder (or arrow keys)'} aria-label="Reorder {col.name}"
                  onpointerdown={(e) => startDrag(e, i)} onpointermove={moveDrag} onpointerup={endDrag} onpointercancel={endDrag}
                  onkeydown={(e) => gripKey(e, i)}>⠿</button>
              </td>
              <td class="num muted">{i + 1}</td>
              <td><input value={col.name} onchange={(e) => { if (!M.rename(mapping, i, e.target.value.trim())) e.target.value = col.name }} /></td>
              <td>
                <select value={col.type} onchange={(e) => M.setType(mapping, i, e.target.value)}>
                  {#each M.TYPES as t}<option value={t}>{t}</option>{/each}
                </select>
              </td>
              <td>
                {#if i === selected}
                  <select value={M.sourceKey(r)} onchange={(e) => M.setSource(mapping, col.name, sourceOf(e.target.value))}>
                    <option value="">— none (default value) —</option>
                    {#each sources as s}<option value={s.key}>{s.label}</option>{/each}
                    {#if M.sourceKey(r) && !sourceOf(M.sourceKey(r))}<option value={M.sourceKey(r)}>{M.sourceKey(r)} (not in this file)</option>{/if}
                  </select>
                {:else}
                  <span class="source" class:none={!M.sourceKey(r)}>{sourceLabel(r)}</span>
                {/if}
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
      {#if filter && visible.length === 0}<p class="muted pad">No column matches “{filter}”.</p>{/if}
      {#if unused.length && !filter}
        <div class="unused">
          <span class="muted">Not in the output (click to add):</span>
          {#each unused as s}<button class="chip" onclick={() => addFrom(s)}>+ {s.label}</button>{/each}
        </div>
      {/if}
      {#each general as i}<div class="issue {i.severity}">{i.path ? i.path + ': ' : ''}{i.message}</div>{/each}
    </div>
  </section>

  {#if drag}
    <div class="ghost" style="left: {drag.x + 14}px; top: {drag.y - 12}px">⠿ {cols[drag.from]?.name} → position {drag.to + 1}</div>
  {/if}

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
  .grip-col { width: 28px; padding-right: 0; }
  .grip { border: none; background: none; cursor: grab; color: var(--muted); padding: 2px 4px; font-size: 15px; line-height: 1; touch-action: none; }
  .grip:hover, .grip:focus-visible { color: var(--accent); }
  .grip.disabled { cursor: not-allowed; opacity: 0.3; }
  .dragging, .dragging * { cursor: grabbing !important; user-select: none; }
  tr.moving td { opacity: 0.35; }
  tr.drop-above td { box-shadow: inset 0 3px 0 var(--accent); }
  tr.drop-below td { box-shadow: inset 0 -3px 0 var(--accent); }
  .num { width: 36px; text-align: right; font-size: 11px; }
  .source { display: block; padding: 4px 8px; border: 1px solid transparent; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .source.none { color: var(--muted); font-style: italic; }
  .filter { width: 180px; margin-left: 8px; }
  .ghost { position: fixed; z-index: 20; pointer-events: none; background: var(--accent); color: var(--accent-text); padding: 3px 10px; border-radius: 12px; font-size: 12px; font-weight: 600; box-shadow: 0 4px 14px rgb(0 0 0 / 0.25); white-space: nowrap; }
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
