<script>
  // Step 2: target columns, their sources and types, with a live preview of
  // the converted output.
  //
  // Built for wide files (hundreds of columns): rows are reordered by
  // dragging their grip, a filter finds columns, and only the selected row
  // carries the heavy source picker (one <select> of every source column
  // per row would mean tens of thousands of DOM nodes).
  //
  // With a linked target file (locked), the output columns are the file's:
  // names, types and order are shown, not edited, and Edit target opens the
  // target editor. Only how each column is filled belongs to the mapping.
  import { Columns3, Plus, Search, GripVertical, ArrowRight, Trash2, TriangleAlert, CircleAlert, Table, Ban, MessageSquareWarning, FileSpreadsheet, ListChecks, CircleHelp, Info, Check, Link, Lock, PencilRuler, Eraser } from '@lucide/svelte'
  import DataTable from './DataTable.svelte'
  import ColumnDetails from './ColumnDetails.svelte'
  import ConfidenceBadge from './ConfidenceBadge.svelte'
  import TypeIcon from './TypeIcon.svelte'
  import * as M from './mapping.js'
  import { Reorder } from './reorder.svelte.js'

  let { mapping = $bindable(), sources = [], detection = null, preview = null, issues = [], autoMatched = [], reviewKey = 0, samplePath = '', onsample,
    locked = false, targetFile = '', onedittarget = () => {} } = $props()
  const hasInput = $derived(!!samplePath)

  let selected = $state(0)
  let chosenTab = $state(null) // null: Review when something needs attention, else the output
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
  // Issues of no column; rules for a column the target lacks are in Review.
  const general = $derived(issues.filter((i) => !cols.some((c) => byColumn.get(c.name)?.includes(i)) &&
    !M.orphans(mapping).some((n) => i.path === `columns.${n}` || i.path.startsWith(`columns.${n}.`))))
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
  function pickSource(name, key) {
    const s = sourceOf(key)
    M.setSource(mapping, name, s, s && detectedFor(s.index ? { source_index: s.index } : { source: s.name }))
  }
  // A type that does not match what the data looks like deserves a warning.
  function mismatch(col, d) {
    if (!d || col.type === 'string') return ''
    if (d.type === 'string') {
      return d.near_type === col.type ? `Only ${Math.round(d.near_confidence * 100)}% of sampled values read as ${col.type}` : `The sampled values look like text, not ${col.type}`
    }
    if (d.type === col.type || (d.type === 'integer' && col.type === 'decimal') || (d.type === 'date' && col.type === 'datetime')) return ''
    return `The sampled values look like ${d.type}, not ${col.type}`
  }
  // Review: everything that needs a decision, in one list. Each item points
  // to its column. "check" items (columns matched by name to a target) go
  // away once confirmed.
  let confirmed = $state(new Set())
  const review = $derived.by(() => {
    const items = []
    cols.forEach((col, i) => {
      const r = mapping.columns?.[col.name]
      const key = M.sourceKey(r)
      if (!r) {
        if (col.default !== undefined) {
          items.push({ level: 'info', i, name: col.name, action: 'map', text: `No source column: every row gets the target default “${col.default}”.` })
        } else {
          items.push({ level: col.required ? 'error' : 'warn', i, name: col.name, action: 'map',
            text: col.required ? 'Required by the target, but no source column is mapped.' : 'In the target, but no source column is mapped: it will be empty.' })
        }
        return
      }
      if (r.default === '' && col.default !== undefined) {
        items.push({ level: 'warn', i, name: col.name, action: 'targetdefault',
          text: `The default is empty, which overrides the target default “${col.default}”.` })
      }
      for (const p of (byColumn.get(col.name) ?? []).filter((p) => p.severity === 'error')) {
        items.push({ level: 'error', i, name: col.name, text: p.message })
      }
      if (key && sources.length && !sourceOf(key)) {
        const optional = r.if_missing && r.if_missing !== 'error'
        items.push({ level: optional ? 'info' : 'error', i, name: col.name, action: 'map',
          text: `Source column “${key}” is not in this file${optional ? ' (optional: handled)' : ''}.` })
      }
      if (autoMatched.includes(col.name) && !confirmed.has(col.name)) {
        items.push({ level: 'check', i, name: col.name, action: 'confirm', text: `Matched by name to source “${key}”. Is it the right column?` })
      }
      const warn = mismatch(col, detectedFor(r))
      if (warn) items.push({ level: 'warn', i, name: col.name, text: warn + '.' })
    })
    for (const name of M.orphans(mapping)) {
      items.push({ level: 'error', i: -1, name, action: 'drop',
        text: `A rule fills “${name}”, but the target has no such column (renamed or removed there).` })
    }
    if (unused.length) {
      items.push({ level: 'info', i: -1, name: '', action: 'unused',
        text: `${unused.length} source ${unused.length === 1 ? 'column is' : 'columns are'} not in the output: ${unused.slice(0, 6).map((s) => s.label).join(', ')}${unused.length > 6 ? '…' : ''}` })
    }
    const order = { error: 0, warn: 1, check: 2, info: 3 }
    return items.sort((a, b) => order[a.level] - order[b.level] || a.i - b.i)
  })
  const toReview = $derived(review.filter((x) => x.level !== 'info').length)
  const tab = $derived(chosenTab ?? (toReview ? 'review' : 'output'))
  // A newly linked target brings the list to the front.
  $effect(() => { if (reviewKey) chosenTab = 'review' })

  function focusColumn(i) {
    filter = ''
    selected = i
    requestAnimationFrame(() => listEl?.querySelector('tr.selected')?.scrollIntoView({ block: 'center', behavior: 'smooth' }))
  }
  function confirm(name) { confirmed = new Set([...confirmed, name]) }
  function dropRule(name) { delete mapping.columns[name] }
  const checks = $derived(review.filter((x) => x.level === 'check'))
  function confirmAll() { confirmed = new Set([...confirmed, ...checks.map((x) => x.name)]) }

  // The output preview highlights the selected column.
  const highlight = $derived(current && preview?.header ? preview.header.indexOf(current.name) : -1)
  function sourceLabel(r) {
    if (!r) return '— not mapped —'
    const key = M.sourceKey(r)
    if (!key) return '— none: default value —'
    return sourceOf(key) || !sources.length ? key : `${key} (not in this file)`
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

  let listEl = $state(null)
  let bodyEl = $state(null)
  const reorder = new Reorder({
    list: () => listEl, body: () => bodyEl, count: () => cols.length, off: () => !!filter || locked,
    move: (from, to) => M.moveTo(mapping, from, to), select: (i) => (selected = i),
  })
</script>

<svelte:window onkeydown={reorder.cancel} />

<div class="layout" class:dragging={!!reorder.drag}>
  <section class="panel columns">
    <div class="head row">
      <span class="title"><Columns3 size={16} /> Columns</span>
      <span class="badge">{filter ? `${visible.length} of ${cols.length}` : cols.length}</span>
      {#if errors.length}<span class="badge error">{errors.length} {errors.length === 1 ? 'error' : 'errors'}</span>{/if}
      {#if toReview}<button class="to-review" onclick={() => (chosenTab = 'review')}><ListChecks size={13} /> {toReview} to review</button>{/if}
      <label class="search"><Search size={14} /><input type="search" bind:value={filter} placeholder="Filter columns…" /></label>
      <span class="spacer"></span>
      {#if !locked}<button onclick={() => addFrom(null)}><Plus size={14} /> Add column</button>{/if}
    </div>
    {#if locked}
      <div class="locked row">
        <Lock size={14} />
        <span>Target <strong class="mono" title={targetFile}>{targetFile.split(/[\\/]/).pop()}</strong>: output names, types and order come from it.</span>
        <span class="spacer"></span>
        <button onclick={onedittarget}><PencilRuler size={13} /> Edit target…</button>
      </div>
    {/if}
    <div class="scroll list" bind:this={listEl}>
      <table>
        <thead>
          <tr>
            <th class="grip-col"></th><th class="num">#</th>
            <th class="side-in">Source column <span class="muted">· detected</span></th>
            <th class="arrow"></th>
            <th class="name-col">Output name</th><th class="type-col">Output type</th><th></th>
          </tr>
        </thead>
        <tbody bind:this={bodyEl}>
          {#each visible as { col, i } (col)}
            {@const r = mapping.columns?.[col.name]}
            {@const problems = byColumn.get(col.name) ?? []}
            {@const d = detectedFor(r)}
            {@const warn = mismatch(col, d)}
            <tr class="{reorder.dropClass(i)}" class:selected={i === selected} class:moving={reorder.drag?.from === i} onclick={() => (selected = i)}>
              <td class="grip-col">
                {#if !locked}
                  <button class="grip" class:disabled={!!filter} tabindex={filter ? -1 : 0}
                    title={filter ? 'Clear the filter to reorder' : 'Drag to reorder (or arrow keys)'} aria-label="Reorder {col.name}"
                    onpointerdown={(e) => reorder.start(e, i)} onpointermove={reorder.move} onpointerup={reorder.end} onpointercancel={reorder.end}
                    onkeydown={(e) => reorder.key(e, i)}><GripVertical size={15} /></button>
                {/if}
              </td>
              <td class="num muted">{i + 1}</td>
              <td class="side-in">
                <div class="source-cell">
                  {#if i === selected}
                    <select value={r ? M.sourceKey(r) : '-'} onchange={(e) => pickSource(col.name, e.target.value)}>
                      {#if !r}<option value="-" disabled>— not mapped —</option>{/if}
                      <option value="">— none: default value —</option>
                      {#each sources as s}<option value={s.key}>{s.label}</option>{/each}
                      {#if M.sourceKey(r) && !sourceOf(M.sourceKey(r))}<option value={M.sourceKey(r)}>{M.sourceKey(r)}{sources.length ? ' (not in this file)' : ''}</option>{/if}
                    </select>
                  {:else}
                    <span class="source" class:none={!M.sourceKey(r)}>{sourceLabel(r)}</span>
                  {/if}
                  {#if d}<ConfidenceBadge detected={d} compact />{/if}
                </div>
              </td>
              <td class="arrow"><ArrowRight size={14} /></td>
              <td>
                {#if locked}<span class="fixed" title={col.name}>{col.name}{#if col.required}<span class="required-mark" title="Required"> *</span>{/if}</span>
                {:else}<input value={col.name} onchange={(e) => { if (!M.rename(mapping, i, e.target.value.trim())) e.target.value = col.name }} />{/if}
              </td>
              <td>
                <div class="type-cell">
                  <TypeIcon type={col.type} />
                  {#if locked}<span class="fixed">{col.type}</span>
                  {:else}
                    <select value={col.type} onchange={(e) => M.setType(mapping, i, e.target.value, d)}>
                      {#each M.TYPES as t}<option value={t}>{t}</option>{/each}
                    </select>
                  {/if}
                  {#if warn}<span class="mismatch" title={warn}><TriangleAlert size={14} /></span>{/if}
                </div>
              </td>
              <td class="status">
                {#if problems.some((p) => p.severity === 'error')}<span class="state error" title={problems.map((p) => p.message).join('\n')}><CircleAlert size={15} /></span>
                {:else if problems.length}<span class="state warn" title={problems.map((p) => p.message).join('\n')}><TriangleAlert size={15} /></span>{/if}
                {#if !locked}<button class="icon delete" onclick={(e) => { e.stopPropagation(); remove(i) }} aria-label="Delete column" title="Delete column"><Trash2 size={14} /></button>{/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
      {#if filter && visible.length === 0}<p class="muted pad">No column matches “{filter}”.</p>{/if}
      {#if unused.length && !filter}
        <div class="unused">
          {#if locked}
            <span class="muted"><Ban size={13} /> {unused.length} source {unused.length === 1 ? 'column is' : 'columns are'} not used (the target has no column for {unused.length === 1 ? 'it' : 'them'}):</span>
            {#each unused as s}<span class="chip static">{s.label}</span>{/each}
          {:else}
            <span class="muted"><Ban size={13} /> {unused.length} source {unused.length === 1 ? 'column is' : 'columns are'} not in the output — click to add:</span>
            {#each unused as s}<button class="chip" onclick={() => addFrom(s)}><Plus size={12} /> {s.label}</button>{/each}
          {/if}
        </div>
      {/if}
      {#each general as i}<div class="issue {i.severity}">{i.path ? i.path + ': ' : ''}{i.message}</div>{/each}
    </div>
  </section>

  {#if reorder.drag}
    {@const drag = reorder.drag}
    <div class="ghost" style="left: {drag.x + 14}px; top: {drag.y - 12}px">⠿ {cols[drag.from]?.name} → position {drag.to + 1}</div>
  {/if}

  <aside class="panel side scroll">
    {#if current && mapping.columns?.[current.name]}
      <ColumnDetails col={current} r={mapping.columns[current.name]} {issues} detected={detectedFor(mapping.columns[current.name])} {locked} />
    {:else if current}
      <div class="details unmapped">
        <h2>{current.name} <TypeIcon type={current.type} size={15} /></h2>
        {#if current.default !== undefined}
          <div class="issue info">No source column: every row gets the target default “{current.default}”.</div>
        {:else}
          <div class="issue {current.required ? 'error' : 'warning'}">
            {current.required ? 'Required by the target, but nothing fills it yet.' : 'In the target, but nothing fills it yet: it will be empty.'}
          </div>
        {/if}
        <label>Take its values from a source column
          <select value="" onchange={(e) => e.target.value && pickSource(current.name, e.target.value)}>
            <option value="">Choose a source column…</option>
            {#each unused as s}<option value={s.key}>{s.label}</option>{/each}
            {#if unused.length && unused.length < sources.length}<option disabled>── already used ──</option>{/if}
            {#each sources.filter((s) => !unused.includes(s)) as s}<option value={s.key}>{s.label}</option>{/each}
          </select>
        </label>
        <p class="muted or">or</p>
        <button onclick={() => (mapping.columns[current.name] = current.default !== undefined ? {} : { default: '' })}>
          {current.default !== undefined ? 'Set a default of this mapping' : 'Fill it with a default value'}</button>
      </div>
    {:else}
      <p class="muted pad">Add a column to start.</p>
    {/if}
  </aside>

  <section class="panel bottom">
    <div class="tabs row">
      <button class:active={tab === 'review'} onclick={() => (chosenTab = 'review')}><ListChecks size={14} /> Review {#if toReview}<span class="badge warn">{toReview}</span>{:else}<Check size={13} />{/if}</button>
      <button class:active={tab === 'output'} onclick={() => (chosenTab = 'output')}><Table size={14} /> Output {#if preview?.rows}<span class="badge">{preview.rows.length}</span>{/if}</button>
      <button class:active={tab === 'rejected'} onclick={() => (chosenTab = 'rejected')}><Ban size={14} /> Rejected {#if preview?.rejected?.length}<span class="badge error">{preview.rejected.length - 1}</span>{/if}</button>
      <button class:active={tab === 'events'} onclick={() => (chosenTab = 'events')}><MessageSquareWarning size={14} /> Messages {#if preview?.events?.length}<span class="badge warn">{preview.events.length}</span>{/if}</button>
      <span class="spacer"></span>
      {#if hasInput}
        <button class="sample" onclick={onsample} title="Sample file: {samplePath} — click to change"><FileSpreadsheet size={14} /> <span class="mono">{samplePath.split(/[\\/]/).pop()}</span></button>
      {:else}
        <button class="primary" onclick={onsample}><FileSpreadsheet size={14} /> Open a sample CSV to preview</button>
      {/if}
    </div>
    {#if preview?.failure && !errors.length}
      <div class="failure"><CircleAlert size={15} /> <span><strong>The preview conversion fails:</strong> {preview.failure}</span></div>
    {/if}
    <div class="tabbody">
      {#if errors.length && tab !== 'review'}
        <div class="pad">{#each errors as i}<div class="issue error">{i.path}: {i.message}</div>{/each}</div>
      {:else if tab === 'review'}
        <div class="review">
          {#if checks.length > 1}
            <div class="row bulk">
              <span class="muted">{checks.length} columns were matched by name: check a few, then confirm the rest.</span>
              <button onclick={confirmAll}><Check size={13} /> Confirm all name matches</button>
            </div>
          {/if}
          {#each review as item}
            <div class="item {item.level}">
              <span class="icon">
                {#if item.level === 'error'}<CircleAlert size={15} />{:else if item.level === 'warn'}<TriangleAlert size={15} />{:else if item.level === 'check'}<CircleHelp size={15} />{:else}<Info size={15} />{/if}
              </span>
              {#if item.i >= 0}<button class="link col" onclick={() => focusColumn(item.i)}>#{item.i + 1} {item.name}</button>
              {:else if item.action === 'drop'}<span class="col mono">{item.name}</span>{/if}
              <span class="text">{item.text}</span>
              {#if item.action === 'map'}<button onclick={() => focusColumn(item.i)}><Link size={13} /> Choose a source</button>{/if}
              {#if item.action === 'confirm'}
                <button onclick={() => focusColumn(item.i)}>Show</button>
                <button onclick={() => confirm(item.name)}><Check size={13} /> Looks right</button>
              {/if}
              {#if item.action === 'targetdefault'}<button onclick={() => delete mapping.columns[item.name].default}>Use the target default</button>{/if}
              {#if item.action === 'drop'}<button onclick={() => dropRule(item.name)}><Eraser size={13} /> Remove the rule</button>{/if}
              {#if item.action === 'unused'}<button onclick={() => listEl?.querySelector('.unused')?.scrollIntoView({ behavior: 'smooth' })}>Show them</button>{/if}
            </div>
          {:else}
            <p class="all-good"><Check size={16} /> Nothing to review: every output column has a source, types match the data, and the mapping is valid.</p>
          {/each}
        </div>
      {:else if tab === 'output'}
        <DataTable header={preview?.header} rows={preview?.rows ?? []} empty="No output rows" {highlight} />
      {:else if tab === 'rejected'}
        <DataTable header={preview?.rejected?.[0]} rows={preview?.rejected?.slice(1) ?? []} rowClass={() => 'rejected'} empty="No rejected rows" />
      {:else}
        <div class="pad">
          {#each preview?.events ?? [] as e}
            <div class="issue {e.level === 'error' ? 'error' : 'warning'}">{[e.line && `line ${e.line}`, e.column].filter(Boolean).join(' · ')}{e.line || e.column ? ': ' : ''}{e.message}</div>
          {:else}<p class="muted">No messages.</p>{/each}
        </div>
      {/if}
    </div>
  </section>
</div>

<style>
  .layout { display: grid; grid-template-columns: 1fr 360px; grid-template-rows: 1fr 260px; gap: 12px; height: 100%; min-height: 0; }
  .columns { display: flex; flex-direction: column; min-height: 0; min-width: 0; }
  .head { padding: 10px 12px; border-bottom: 1px solid var(--line); }
  .list { flex: 1; min-height: 0; padding-bottom: 8px; }
  /* Fixed layout: declared widths win over content, the source column
     takes what is left and truncates long names. */
  table { width: 100%; border-collapse: collapse; table-layout: fixed; }
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
  .title { display: inline-flex; gap: 6px; align-items: center; font-weight: 600; }
  .head button { display: inline-flex; gap: 5px; align-items: center; }
  .search { flex-direction: row; align-items: center; gap: 0; margin-left: 8px; position: relative; color: var(--muted); }
  .search :global(svg) { position: absolute; left: 8px; }
  .search input { padding-left: 27px; width: 190px; }
  /* Header cells are opaque and above the rows scrolling under them. */
  th { z-index: 2; }
  th.side-in { background: var(--panel); }
  td.side-in { background: color-mix(in srgb, var(--soft) 45%, transparent); }
  tr.selected td.side-in { background: var(--soft); }
  .source-cell { display: flex; gap: 6px; align-items: center; min-width: 0; }
  .source-cell select { flex: 1; min-width: 0; }
  .source-cell .source { flex: 1; min-width: 0; }
  .source-cell :global(.conf) { flex-shrink: 0; }
  .arrow { width: 22px; color: var(--muted); text-align: center; padding: 0; }
  .type-cell { display: flex; align-items: center; gap: 5px; }
  .name-col { width: 26%; }
  .type-col { width: 150px; }
  th:last-child { width: 74px; }
  .type-cell select { flex: 1; }
  .mismatch { color: var(--fair); display: inline-flex; }
  .state { display: inline-flex; vertical-align: middle; margin-right: 4px; }
  .state.error { color: var(--error); }
  .state.warn { color: var(--warn); }
  .delete { color: var(--muted); }
  .delete:hover { color: var(--error); }
  .unused .muted { display: inline-flex; gap: 5px; align-items: center; }
  .chip.static { border: 1px solid var(--line); color: var(--muted); }
  .locked { gap: 7px; padding: 6px 12px; font-size: 12px; background: var(--neutral-soft); border-bottom: 1px solid var(--line); color: var(--muted); }
  .locked strong { color: var(--text); }
  .locked button { display: inline-flex; gap: 4px; align-items: center; font-size: 12px; padding: 2px 10px; }
  .fixed { display: block; padding: 2px 6px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-weight: 500; }
  .chip { display: inline-flex; gap: 3px; align-items: center; }
  .tabs button { display: inline-flex; gap: 5px; align-items: center; }
  .to-review { display: inline-flex; gap: 4px; align-items: center; border-radius: 12px; padding: 1px 9px; font-size: 12px; background: var(--warn-soft); color: var(--warn); border-color: transparent; }
  .review { padding: 8px 12px; display: grid; gap: 5px; }
  .item { display: flex; gap: 8px; align-items: center; padding: 6px 10px; border-radius: 7px; background: var(--neutral-soft); }
  .item .text { flex: 1; }
  .item button { display: inline-flex; gap: 4px; align-items: center; padding: 2px 9px; font-size: 12px; white-space: nowrap; }
  .item .col { font-family: var(--mono); font-size: 12px; font-weight: 600; padding: 0; white-space: nowrap; }
  .item .icon { display: inline-flex; }
  .item.error { background: var(--error-soft); }
  .item.error .icon { color: var(--error); }
  .item.warn { background: var(--warn-soft); }
  .item.warn .icon { color: var(--warn); }
  .item.check .icon { color: var(--accent); }
  .item.check { background: var(--soft); }
  .item.info .icon { color: var(--muted); }
  .bulk { justify-content: space-between; padding: 2px 2px 4px; font-size: 12px; }
  .bulk button { display: inline-flex; gap: 4px; align-items: center; font-size: 12px; padding: 2px 10px; }
  .all-good { display: flex; gap: 8px; align-items: center; color: var(--high); padding: 8px 4px; }
  .unmapped { display: grid; gap: 10px; }
  .unmapped h2 { display: flex; gap: 6px; align-items: center; }
  .or { margin: 0; text-align: center; }
  .failure { display: flex; gap: 7px; align-items: flex-start; padding: 7px 12px; background: var(--error-soft); color: var(--error); font-size: 12px; border-bottom: 1px solid var(--line); }
  .failure strong { color: inherit; }
  .tabs .sample { display: inline-flex; gap: 5px; align-items: center; color: var(--muted); max-width: 210px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
  .tabs .sample:hover { color: var(--accent); }
  .tabs { flex-wrap: nowrap; overflow: hidden; }
  .tabs button.primary { background: var(--accent); color: var(--accent-text); }
  .source { display: block; padding: 2px 8px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 100%; font-weight: 500; }
  .source.none { color: var(--muted); font-style: italic; }
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
