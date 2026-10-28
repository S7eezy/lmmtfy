<script>
  // Combine rows: several input rows (a batch, sharing a key) make one
  // output row. Numbered target columns (price_0 … price_16) take the 1st,
  // 2nd, … row of the batch; other columns read the key, or aggregate the
  // batch with lowest(…), total(…), …
  import { untrack } from 'svelte'
  import { Combine, Check, TriangleAlert, Info } from '@lucide/svelte'
  import Modal from './Modal.svelte'
  import * as M from './mapping.js'
  import * as api from './api.js'

  let { mapping, sources = [], samplePath = '', detectedFor = () => null, onclose } = $props()

  // The dialog starts from the mapping as it is when opened.
  const g = untrack(() => mapping.group)
  const fam = untrack(() => M.families(M.columns(mapping)))
  const existing = untrack(() => Object.keys(mapping.columns ?? {}).filter(M.isPattern))
  let patterns = $state(existing.length ? existing : fam.patterns)
  const guess = () => untrack(() => sources).find((s) => /batch|order|group|lot/i.test(s.label)) ?? sources[0]
  let by = $state(g?.by ? [...g.by] : guess() ? [guess().name] : [])
  let orderBy = $state(g?.order_by?.[0] ?? '')
  let sorted = $state(g?.sorted ?? true)
  let overflow = $state(g?.overflow ?? 'error')
  let stats = $state(null)
  let statsError = $state('')

  const helpers = $derived(Object.keys(mapping.helpers ?? {}))
  const slots = $derived(fam.numbers.length && patterns.length ? fam.numbers.length : 0)
  const named = $derived(sources.filter((s) => !s.index))

  $effect(() => {
    const key = [...by]
    stats = null
    statsError = ''
    if (!samplePath || !key.length) return
    const snapshot = JSON.parse(JSON.stringify(mapping))
    api.batches(samplePath, snapshot, key).then((s) => {
      stats = s
      if (!s.sorted) sorted = false
    }).catch((e) => (statsError = String(e)))
  })

  function toggle(list, v) { return list.includes(v) ? list.filter((x) => x !== v) : [...list, v] }
  function apply() {
    const group = { by: [...by], sorted, overflow }
    if (orderBy) group.order_by = [orderBy]
    if (mapping.group) {
      mapping.group = group
      M.combine(mapping, group, patterns.filter((p) => !mapping.columns[p]), sources, detectedFor)
    } else {
      M.combine(mapping, group, patterns, sources, detectedFor)
    }
    onclose()
  }
  function stop() {
    M.uncombine(mapping)
    onclose()
  }
</script>

<Modal title="Combine rows" icon={Combine} {onclose} wide>
  <p class="lead">Several input rows sharing a key (a batch) become <strong>one output row</strong>. Numbered output columns take the 1st, 2nd, … row of the batch; other columns read the key, or sum up the batch with <span class="mono">lowest(…)</span>, <span class="mono">total(…)</span>, <span class="mono">count()</span>…</p>

  <h3>Key: rows with the same values form a batch</h3>
  <div class="chips">
    {#each named as s}
      <button class="chip" class:on={by.includes(s.name)} onclick={() => (by = toggle(by, s.name))}>{#if by.includes(s.name)}<Check size={12} />{/if} {s.label}</button>
    {/each}
  </div>
  {#if !samplePath}<p class="hint">Open a sample file to see the batches it makes.</p>
  {:else if statsError}<div class="issue error">{statsError}</div>
  {:else if stats}
    <div class="stats" class:warn={!stats.sorted || (slots && stats.max > slots)}>
      <strong>{stats.batches} {stats.batches === 1 ? 'batch' : 'batches'}</strong> in {stats.rows}{stats.partial ? '+' : ''} rows,
      {stats.min === stats.max ? `${stats.min} rows each` : `${stats.min} to ${stats.max} rows each`}.
      {#if stats.sorted}<span class="ok"><Check size={13} /> The rows of each batch are next to each other.</span>
      {:else}<span><TriangleAlert size={13} /> The rows of a batch are spread over the file: batches are then held in memory until the end of the file.</span>{/if}
      {#if slots && stats.max > slots}<span><TriangleAlert size={13} /> The largest batch has {stats.max} rows, for {slots} slots.</span>{/if}
    </div>
  {/if}

  <h3>Repeated output columns</h3>
  {#if fam.patterns.length}
    <div class="chips">
      {#each fam.patterns as p}
        <button class="chip mono" class:on={patterns.includes(p)} onclick={() => (patterns = toggle(patterns, p))}>{#if patterns.includes(p)}<Check size={12} />{/if} {p}</button>
      {/each}
    </div>
    <p class="hint">{fam.numbers.length} slots, numbered {fam.numbers[0]} to {fam.numbers[fam.numbers.length - 1]}: each pattern is mapped once, for every slot.</p>
  {:else}
    <p class="hint with-icon"><Info size={13} /> The target has no numbered columns: each batch gives one row of key columns and aggregates.</p>
  {/if}

  <div class="grid">
    <label>Order of the rows in a batch
      <select bind:value={orderBy}>
        <option value="">as in the file</option>
        {#each helpers as h}<option value={h}>by helper {h}</option>{/each}
      </select>
    </label>
    <label>More rows than slots
      <select bind:value={overflow}>
        <option value="error">the batch is an error</option>
        <option value="drop_extra">drop the extra rows</option>
      </select>
    </label>
    <label>Rows of a batch
      <select value={String(sorted)} onchange={(e) => (sorted = e.target.value === 'true')}>
        <option value="true">are next to each other (streams any size)</option>
        <option value="false">may be anywhere (held in memory)</option>
      </select>
    </label>
  </div>
  {#if !helpers.length}<p class="hint">To order the rows of a batch, add a helper (e.g. a sequence number read as an integer) from a formula.</p>{/if}

  {#snippet footer()}
    {#if mapping.group}<button onclick={stop}>Stop combining</button>{/if}
    <span class="spacer"></span>
    <button onclick={onclose}>Cancel</button>
    <button class="primary" disabled={!by.length} onclick={apply}>{mapping.group ? 'Apply' : 'Combine rows'}</button>
  {/snippet}
</Modal>

<style>
  .lead { margin: 0 0 6px; font-size: 13px; }
  .chips { display: flex; flex-wrap: wrap; gap: 5px; }
  .chip { display: inline-flex; gap: 4px; align-items: center; border-radius: 12px; padding: 2px 10px; font-size: 12px; }
  .chip.on { background: var(--soft); border-color: var(--accent); color: var(--accent); font-weight: 600; }
  .stats { display: grid; gap: 3px; margin-top: 8px; padding: 7px 10px; border-radius: 7px; background: var(--neutral-soft); font-size: 12px; }
  .stats span { display: inline-flex; gap: 5px; align-items: center; }
  .stats.warn { background: var(--warn-soft); color: var(--warn); }
  .ok { color: var(--ok); }
  .grid { margin-top: 14px; }
  .with-icon { display: flex; gap: 5px; align-items: center; }
</style>
