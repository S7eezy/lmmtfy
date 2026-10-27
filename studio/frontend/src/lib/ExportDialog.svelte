<script>
  // Export standalone converters for production machines.
  import { Package, Monitor, Apple, Server, CircleCheck, CircleX } from '@lucide/svelte'
  import Modal from './Modal.svelte'
  import * as api from './api.js'

  let { mapping, onclose } = $props()

  let platforms = $state([])
  let chosen = $state({})
  let dir = $state('')
  let results = $state([])
  let running = $state(false)

  $effect(() => {
    api.platforms().then((list) => {
      platforms = list
      for (const p of list) if (p.available && p.os === 'windows') chosen[`${p.os}/${p.arch}`] = true
    })
  })

  async function pickDir() { dir = (await api.chooseDirectory('Write converters to')) || dir }
  async function run() {
    running = true
    results = []
    for (const p of platforms.filter((p) => chosen[`${p.os}/${p.arch}`])) {
      try { results.push({ label: p.label, path: await api.exportConverter(mapping, p.os, p.arch, dir) }) }
      catch (e) { results.push({ label: p.label, error: String(e) }) }
    }
    running = false
  }
</script>

<Modal title="Export converters" icon={Package} {onclose}>
  <p class="muted">A converter is a single program with this mapping built in: copy it to the production machine and run <span class="mono">converter input.csv output.csv</span>. Nothing else is needed, not even lmmtfy.</p>
  <div class="list">
    {#each platforms as p}
      <label class="inline">
        <input type="checkbox" disabled={!p.available} bind:checked={chosen[`${p.os}/${p.arch}`]} />
        {#if p.os === 'darwin'}<Apple size={15} />{:else if p.os === 'windows'}<Monitor size={15} />{:else}<Server size={15} />{/if}
        {p.label} <span class="muted mono">{p.os}/{p.arch}</span>
        {#if !p.available}<span class="badge warn">not included in this Studio build</span>{/if}
      </label>
    {/each}
  </div>
  <label>Folder
    <div class="row"><input class="mono grow" bind:value={dir} /><button onclick={pickDir}>Choose…</button></div>
  </label>
  {#each results as r}
    <div class="issue {r.error ? 'error' : 'ok'} result">
      {#if r.error}<CircleX size={15} />{:else}<CircleCheck size={15} />{/if}
      <div><strong>{r.label}</strong><div class="mono path">{r.error ?? r.path}</div></div>
    </div>
  {/each}
  {#snippet footer()}
    <button onclick={onclose}>Close</button>
    <button class="primary" onclick={run} disabled={running || !dir || !Object.values(chosen).some(Boolean)}>{running ? 'Exporting…' : 'Export'}</button>
  {/snippet}
</Modal>

<style>
  .list { display: grid; gap: 6px; margin: 10px 0 14px; }
  .grow { flex: 1; }
  .result { display: flex; gap: 8px; align-items: flex-start; }
  .path { word-break: break-all; margin-top: 2px; }
  .issue.ok { background: var(--soft); color: var(--ok); }
</style>
