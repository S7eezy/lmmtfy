<script>
  // Mapping-wide settings: name, the output format of the target, where the
  // target is stored, and what happens on errors.
  import Modal from './Modal.svelte'
  import * as api from './api.js'

  let { mapping = $bindable(), targetPath = $bindable(), onclose, onerror } = $props()

  const out = $derived(mapping.target.inline.output)
  const errs = $derived(mapping.on_error)

  function setNumber(obj, key, value) {
    if (value === '') delete obj[key]
    else obj[key] = Number(value)
  }

  async function useTarget() {
    const path = await api.chooseFile('Use an existing target', 'mapping')
    if (!path) return
    try {
      const t = await api.loadTarget(path)
      const old = mapping.columns ?? {}
      mapping.target.inline = t
      // Keep the rules of columns that exist in the new target.
      mapping.columns = Object.fromEntries(t.columns.filter((c) => old[c.name]).map((c) => [c.name, old[c.name]]))
      mapping.target.inline.output.header ??= true
      targetPath = path
    } catch (e) {
      onerror(e)
    }
  }
  async function saveTargetAs() {
    const path = await api.chooseSaveFile('Store the target in its own file', (mapping.target.inline.name || 'target') + '.target.yaml', 'mapping')
    if (path) targetPath = path
  }
</script>

<Modal title="Mapping settings" {onclose} wide>
  <div class="grid">
    <label>Mapping name <input bind:value={mapping.name} /></label>
    <label>Description <input value={mapping.description ?? ''} oninput={(e) => (mapping.description = e.target.value)} /></label>
  </div>

  <h3>Target</h3>
  <p class="muted">The target is the format the receiving software expects: output columns and file format. It can live inside this mapping, or in its own file shared by several mappings.</p>
  <div class="row">
    {#if targetPath}
      <span>Stored in <span class="mono">{targetPath}</span> — saving updates that file, for every mapping using it.</span>
      <button onclick={() => (targetPath = '')}>Keep it in this mapping instead</button>
    {:else}
      <span>Stored in this mapping.</span>
      <button onclick={saveTargetAs}>Store in its own file…</button>
    {/if}
    <button onclick={useTarget}>Use an existing target…</button>
  </div>
  <div class="grid">
    <label>Target name <input value={mapping.target.inline.name ?? ''} oninput={(e) => (mapping.target.inline.name = e.target.value)} placeholder="optional" /></label>
  </div>

  <h3>Output file format</h3>
  <div class="grid">
    <label>Encoding
      <select bind:value={out.encoding}>
        {#each ['utf-8', 'utf-16le', 'utf-16be', 'windows-1252', 'latin-1', 'iso-8859-15'] as e}<option value={e}>{e}</option>{/each}
      </select>
    </label>
    <label class="inline"><input type="checkbox" bind:checked={out.bom} /> Byte order mark (BOM)</label>
    <label>Delimiter
      <select bind:value={out.delimiter}>
        <option value=",">comma ,</option><option value=";">semicolon ;</option><option value="tab">tab</option><option value="|">pipe |</option>
      </select>
    </label>
    <label>Quoting
      <select bind:value={out.quote_style}>
        <option value="minimal">only when needed</option><option value="always">every value</option>
        <option value="non_numeric">every non-numeric value</option><option value="never">never</option>
      </select>
    </label>
    <label>Escape inside quotes
      <select bind:value={out.escape}><option value="double">doubled ""</option><option value="backslash">backslash \"</option></select>
    </label>
    <label>Line endings
      <select bind:value={out.line_ending}><option value="crlf">Windows (CRLF)</option><option value="lf">Unix (LF)</option></select>
    </label>
    <label class="inline"><input type="checkbox" bind:checked={out.header} /> Header row</label>
    <label>Empty values written as <input value={out.null_value ?? ''} oninput={(e) => (out.null_value = e.target.value)} placeholder="nothing" /></label>
  </div>

  <h3>Errors</h3>
  <div class="grid">
    <label>When a value is invalid
      <select bind:value={errs.strategy}>
        <option value="fail_file">fail the whole file</option>
        <option value="skip_row">reject the row</option>
        <option value="default">use the column default</option>
      </select>
    </label>
    <label class="inline"><input type="checkbox" bind:checked={errs.reject_file} /> Write rejected rows to a reject file</label>
    <label>Fail if more than this share of rows has errors
      <input type="number" min="0" max="1" step="0.01" value={errs.max_error_rate ?? ''} oninput={(e) => setNumber(errs, 'max_error_rate', e.target.value)} placeholder="no limit (0.01 = 1%)" />
    </label>
    <label>Fail after this many rows with errors
      <input type="number" min="0" value={errs.max_errors ?? ''} oninput={(e) => setNumber(errs, 'max_errors', e.target.value)} placeholder="no limit" />
    </label>
  </div>

  <h3>Logging</h3>
  <div class="grid">
    <label>Level
      <select bind:value={mapping.logging.level}>
        <option value="none">none</option><option value="errors">errors</option><option value="warnings">warnings</option>
        <option value="info">info</option><option value="trace">every row (slow)</option>
      </select>
    </label>
    <label>Format <select bind:value={mapping.logging.format}><option value="text">text</option><option value="json">JSON lines</option></select></label>
    <label>Log file <input value={mapping.logging.file ?? ''} oninput={(e) => (mapping.logging.file = e.target.value)} placeholder="standard error" /></label>
    <label class="inline"><input type="checkbox" bind:checked={mapping.logging.summary} /> Summary at the end</label>
  </div>
  {#snippet footer()}<button class="primary" onclick={onclose}>Done</button>{/snippet}
</Modal>
