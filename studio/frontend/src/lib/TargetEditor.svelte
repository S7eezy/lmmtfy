<script>
  // The target editor: the columns of a target, in order, and how each one
  // is checked and written. The file format and name are in Settings.
  //
  // A target file is shared by mappings, which refer to its columns by name:
  // renaming or removing a column of a saved file says so. renames() tells
  // the caller which columns were renamed since the editor opened, so that a
  // mapping it was opened from can follow.
  import { Columns3, Plus, Search, GripVertical, Trash2, TriangleAlert, CircleAlert, Sparkles, FileOutput, Check, ListChecks, Info, Users } from '@lucide/svelte'
  import ConfidenceBadge from './ConfidenceBadge.svelte'
  import TypeIcon from './TypeIcon.svelte'
  import Section from './Section.svelte'
  import TargetColumnForm from './TargetColumnForm.svelte'
  import * as M from './mapping.js'
  import { Reorder } from './reorder.svelte.js'
  import { toast } from './toast.svelte.js'

  // detection: when the target was drafted from an example file, what was
  // detected for each column, in the order of target.columns.
  let { target = $bindable(), path = '', detection = null, issues = [] } = $props()

  let selected = $state(0)
  let filter = $state('')
  const cols = $derived(target.columns)
  const current = $derived(cols[Math.min(selected, cols.length - 1)])

  // Columns are tracked by identity, which survives renames and moves.
  const original = new WeakMap() // column → its name when the editor opened
  const detected = new WeakMap()
  target.columns.forEach((c, i) => {
    original.set(c, c.name)
    if (detection?.columns?.[i]) detected.set(c, detection.columns[i])
  })
  const removed = $state([])

  export function renames() {
    const out = {}
    for (const c of target.columns) {
      const was = original.get(c)
      if (was && was !== c.name) out[was] = c.name
    }
    return out
  }

  const visible = $derived.by(() => {
    const all = cols.map((col, i) => ({ col, i }))
    const q = filter.trim().toLowerCase()
    return q ? all.filter(({ col }) => col.name.toLowerCase().includes(q)) : all
  })

  function issuesOf(col, i) {
    return issues.filter((x) => x.path === `target.columns.${col.name}` || x.path.startsWith(`target.columns.${col.name}.`) ||
      x.path.startsWith(`target.columns[${i}]`))
  }
  const general = $derived(issues.filter((x) => !cols.some((c, i) => issuesOf(c, i).includes(x))))
  const errors = $derived(issues.filter((x) => x.severity === 'error'))

  function uniqueName(base) {
    const names = new Set(cols.map((c) => c.name))
    let name = base
    for (let n = 2; names.has(name); n++) name = `${base}_${n}`
    return name
  }
  function add() {
    target.columns.push({ name: uniqueName(`column_${cols.length + 1}`), type: 'string' })
    selected = cols.length - 1
    filter = ''
    requestAnimationFrame(() => listEl?.querySelector('tr.selected')?.scrollIntoView({ block: 'nearest' }))
  }
  function remove(i) {
    const [col] = target.columns.splice(i, 1)
    selected = Math.max(0, Math.min(selected, cols.length - 1))
    const was = original.get(col)
    if (path && was) {
      removed.push(was)
      toast('info', `Removed ${was}. Mappings with a rule for it will list it in their Review.`)
    }
  }
  function rename(col, name) {
    if (!name || cols.some((c) => c !== col && c.name === name)) return false
    col.name = name
    return true
  }

  let listEl = $state(null)
  let bodyEl = $state(null)
  const reorder = new Reorder({
    list: () => listEl, body: () => bodyEl, count: () => cols.length, off: () => !!filter,
    move: (from, to) => target.columns.splice(to, 0, ...target.columns.splice(from, 1)), select: (i) => (selected = i),
  })
  function focusColumn(i) {
    filter = ''
    selected = i
    requestAnimationFrame(() => listEl?.querySelector('tr.selected')?.scrollIntoView({ block: 'center', behavior: 'smooth' }))
  }
</script>

<svelte:window onkeydown={reorder.cancel} />

<div class="layout" class:dragging={!!reorder.drag}>
  <section class="panel columns">
    <div class="head row">
      <span class="title"><Columns3 size={16} /> Target columns</span>
      <span class="badge">{filter ? `${visible.length} of ${cols.length}` : cols.length}</span>
      {#if errors.length}<span class="badge error">{errors.length} {errors.length === 1 ? 'error' : 'errors'}</span>{/if}
      <label class="search"><Search size={14} /><input type="search" bind:value={filter} placeholder="Filter columns…" /></label>
      <span class="spacer"></span>
      <button onclick={add}><Plus size={14} /> Add column</button>
    </div>
    {#if path}
      <div class="shared row"><Users size={14} /> <span>Mappings using <strong class="mono">{path.split(/[\\/]/).pop()}</strong> see every saved change. They refer to columns by name: renaming or removing one leaves their rule behind.</span></div>
    {/if}
    <div class="scroll list" bind:this={listEl}>
      <table>
        <thead>
          <tr>
            <th class="grip-col"></th><th class="num">#</th>
            <th>Name</th><th class="type-col">Type</th><th class="req-col">Required</th>
            {#if detection}<th class="det-col">In the example</th>{/if}
            <th class="status"></th>
          </tr>
        </thead>
        <tbody bind:this={bodyEl}>
          {#each visible as { col, i } (col)}
            {@const problems = issuesOf(col, i)}
            {@const d = detected.get(col)}
            <tr class="{reorder.dropClass(i)}" class:selected={i === selected} class:moving={reorder.drag?.from === i} onclick={() => (selected = i)}>
              <td class="grip-col">
                <button class="grip" class:disabled={!!filter} tabindex={filter ? -1 : 0}
                  title={filter ? 'Clear the filter to reorder' : 'Drag to reorder (or arrow keys)'} aria-label="Reorder {col.name}"
                  onpointerdown={(e) => reorder.start(e, i)} onpointermove={reorder.move} onpointerup={reorder.end} onpointercancel={reorder.end}
                  onkeydown={(e) => reorder.key(e, i)}><GripVertical size={15} /></button>
              </td>
              <td class="num muted">{i + 1}</td>
              <td><input value={col.name} onchange={(e) => { if (!rename(col, e.target.value.trim())) e.target.value = col.name }} /></td>
              <td>
                <div class="type-cell">
                  <TypeIcon type={col.type} />
                  <select value={col.type} onchange={(e) => M.setColumnType(col, e.target.value)}>
                    {#each M.TYPES as t}<option value={t}>{t}</option>{/each}
                  </select>
                </div>
              </td>
              <td class="req-col"><input type="checkbox" checked={!!col.required} onchange={(e) => (e.target.checked ? (col.required = true) : delete col.required)} aria-label="Required" /></td>
              {#if detection}<td class="det-col">{#if d}<ConfidenceBadge detected={d} compact />{/if}</td>{/if}
              <td class="status">
                {#if problems.some((p) => p.severity === 'error')}<span class="state error" title={problems.map((p) => p.message).join('\n')}><CircleAlert size={15} /></span>
                {:else if problems.length}<span class="state warn" title={problems.map((p) => p.message).join('\n')}><TriangleAlert size={15} /></span>{/if}
                <button class="icon delete" onclick={(e) => { e.stopPropagation(); remove(i) }} aria-label="Delete column" title="Delete column"><Trash2 size={14} /></button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
      {#if filter && visible.length === 0}<p class="muted pad">No column matches “{filter}”.</p>{/if}
      {#if !cols.length}<p class="muted pad">No columns yet: add the first one.</p>{/if}
    </div>
  </section>

  {#if reorder.drag}
    {@const drag = reorder.drag}
    <div class="ghost" style="left: {drag.x + 14}px; top: {drag.y - 12}px">⠿ {cols[drag.from]?.name} → position {drag.to + 1}</div>
  {/if}

  <aside class="panel side scroll">
    {#if current}
      {@const d = detected.get(current)}
      {@const was = original.get(current)}
      <div class="details">
        <h2><TypeIcon type={current.type} size={15} /> {current.name}</h2>
        {#each issuesOf(current, cols.indexOf(current)) as x}<div class="issue {x.severity}">{x.path.replace(`target.columns.${current.name}.`, '')}: {x.message}</div>{/each}
        {#if path && was && was !== current.name}
          <div class="issue warning">Renamed from <span class="mono">{was}</span>: other mappings using this target still fill “{was}”. Their Review will ask to map “{current.name}”.</div>
        {/if}
        {#if d}
          <Section icon={Sparkles} title="In the example file">
            <div class="row"><ConfidenceBadge detected={d} /><span class="muted small">{d.values} values{d.empty ? `, ${d.empty} empty` : ''}</span></div>
            <ConfidenceBadge detected={d} bar />
            {#if d.examples?.length}<div class="examples">{#each d.examples as e}<span class="mono example">{e}</span>{/each}</div>{/if}
            {#if d.failing?.length}<div class="issue warning">Not {d.type}: <span class="mono">{d.failing.join(' · ')}</span></div>{/if}
          </Section>
        {/if}
        <Section icon={FileOutput} title="Checking and writing" hint="How values of this column are checked and written in converted files.">
          <TargetColumnForm col={current} withDefault />
          {#if ['date', 'datetime', 'time'].includes(current.type)}<p class="hint">Tokens: <span class="mono">YYYY YY MM M DD D HH hh h mm ss .SSS A Z</span></p>{/if}
        </Section>
      </div>
    {:else}
      <p class="muted pad">Add a column to start.</p>
    {/if}
  </aside>

  <section class="panel bottom">
    <div class="tabs row"><span class="tab"><ListChecks size={14} /> Problems {#if issues.length}<span class="badge {errors.length ? 'error' : 'warn'}">{issues.length}</span>{:else}<Check size={13} />{/if}</span></div>
    <div class="tabbody">
      {#each issues as x}
        {@const i = cols.findIndex((c, k) => issuesOf(c, k).includes(x))}
        <div class="item {x.severity === 'error' ? 'error' : 'warn'}">
          <span class="icon">{#if x.severity === 'error'}<CircleAlert size={15} />{:else}<TriangleAlert size={15} />{/if}</span>
          {#if i >= 0}<button class="link col" onclick={() => focusColumn(i)}>#{i + 1} {cols[i].name}</button>{:else}<span class="col mono">{x.path}</span>{/if}
          <span class="text">{x.message}</span>
        </div>
      {:else}
        <p class="all-good"><Check size={16} /> No problems: the target can be saved.</p>
      {/each}
      {#if removed.length}<p class="muted note"><Info size={13} /> Removed since opening: {removed.join(', ')}.</p>{/if}
    </div>
  </section>
</div>

<style>
  .layout { display: grid; grid-template-columns: 1fr 380px; grid-template-rows: 1fr 170px; gap: 12px; height: 100%; min-height: 0; }
  .columns { display: flex; flex-direction: column; min-height: 0; min-width: 0; }
  .head { padding: 10px 12px; border-bottom: 1px solid var(--line); }
  .head button { display: inline-flex; gap: 5px; align-items: center; }
  .title { display: inline-flex; gap: 6px; align-items: center; font-weight: 600; }
  .search { flex-direction: row; align-items: center; gap: 0; margin-left: 8px; position: relative; color: var(--muted); }
  .search :global(svg) { position: absolute; left: 8px; }
  .search input { padding-left: 27px; width: 190px; }
  .shared { gap: 7px; padding: 6px 12px; font-size: 12px; background: var(--neutral-soft); border-bottom: 1px solid var(--line); color: var(--muted); align-items: flex-start; }
  .shared :global(svg) { flex-shrink: 0; margin-top: 1px; }
  .shared strong { color: var(--text); }
  .list { flex: 1; min-height: 0; padding-bottom: 8px; }
  table { width: 100%; border-collapse: collapse; table-layout: fixed; }
  th { text-align: left; font-size: 11px; color: var(--muted); font-weight: 600; padding: 6px 8px; position: sticky; top: 0; background: var(--panel); z-index: 2; }
  td { padding: 4px 6px; border-top: 1px solid var(--line); }
  td input:not([type="checkbox"]), td select { width: 100%; }
  tr { cursor: pointer; }
  tr.selected td { background: var(--soft); }
  .grip-col { width: 28px; padding-right: 0; }
  .grip { border: none; background: none; cursor: grab; color: var(--muted); padding: 2px 4px; line-height: 1; touch-action: none; }
  .grip:hover, .grip:focus-visible { color: var(--accent); }
  .grip.disabled { cursor: not-allowed; opacity: 0.3; }
  .dragging, .dragging * { cursor: grabbing !important; user-select: none; }
  tr.moving td { opacity: 0.35; }
  tr.drop-above td { box-shadow: inset 0 3px 0 var(--accent); }
  tr.drop-below td { box-shadow: inset 0 -3px 0 var(--accent); }
  .num { width: 36px; text-align: right; font-size: 11px; }
  .type-col { width: 160px; }
  .type-cell { display: flex; align-items: center; gap: 5px; }
  .type-cell select { flex: 1; }
  .req-col { width: 70px; text-align: center; }
  .det-col { width: 110px; }
  .status { width: 60px; white-space: nowrap; text-align: right; }
  .state { display: inline-flex; vertical-align: middle; margin-right: 4px; }
  .state.error { color: var(--error); }
  .state.warn { color: var(--warn); }
  .delete { color: var(--muted); }
  .delete:hover { color: var(--error); }
  .ghost { position: fixed; z-index: 20; pointer-events: none; background: var(--accent); color: var(--accent-text); padding: 3px 10px; border-radius: 12px; font-size: 12px; font-weight: 600; box-shadow: 0 4px 14px rgb(0 0 0 / 0.25); white-space: nowrap; }
  .side { grid-row: 1 / span 2; grid-column: 2; min-height: 0; }
  .details { padding: 12px 16px 20px; }
  .details h2 { display: flex; gap: 6px; align-items: center; word-break: break-all; padding-bottom: 8px; border-bottom: 1px solid var(--line); }
  .small { font-size: 11px; }
  .examples { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; }
  .example { background: var(--neutral-soft); padding: 1px 6px; border-radius: 5px; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .bottom { display: flex; flex-direction: column; min-height: 0; min-width: 0; }
  .tabs { padding: 6px 8px; border-bottom: 1px solid var(--line); }
  .tab { display: inline-flex; gap: 5px; align-items: center; padding: 4px 10px; border-radius: 6px; background: var(--soft); color: var(--accent); font-weight: 600; }
  .tabbody { flex: 1; min-height: 0; overflow: auto; padding: 8px 12px; display: grid; gap: 5px; align-content: start; }
  .item { display: flex; gap: 8px; align-items: center; padding: 6px 10px; border-radius: 7px; }
  .item .text { flex: 1; }
  .item .col { font-family: var(--mono); font-size: 12px; font-weight: 600; padding: 0; white-space: nowrap; }
  .item .icon { display: inline-flex; }
  .item.error { background: var(--error-soft); }
  .item.error .icon { color: var(--error); }
  .item.warn { background: var(--warn-soft); }
  .item.warn .icon { color: var(--warn); }
  .all-good { display: flex; gap: 8px; align-items: center; color: var(--high); margin: 4px; }
  .note { display: flex; gap: 5px; align-items: center; margin: 4px; font-size: 12px; }
  .pad { padding: 12px 16px; }
</style>
