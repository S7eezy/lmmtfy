<script>
  // Mapping-wide settings: identity, target, output file format, errors,
  // logging. Used as step 2 of a new mapping and in the Settings dialog.
  import { Tag, Target, FileOutput, ShieldAlert, ScrollText, FilePlus, FolderOpen, FileCog } from '@lucide/svelte'
  import Section from './Section.svelte'
  import * as api from './api.js'
  import * as M from './mapping.js'
  import { toast } from './toast.svelte.js'

  let { mapping = $bindable(), targetPath = $bindable(), sources = [] } = $props()

  const out = $derived(mapping.target.inline.output)
  const errs = $derived(mapping.on_error)
  const mode = $derived(targetPath ? 'file' : 'inline')

  function setNumber(obj, key, value) {
    if (value === '' || value === null) delete obj[key]
    else obj[key] = Number(value)
  }

  async function useTarget() {
    const path = await api.chooseFile('Use an existing target', 'mapping')
    if (!path) return
    try {
      const t = await api.loadTarget(path)
      const old = mapping.columns ?? {}
      t.output.header ??= true
      mapping.target.inline = t
      mapping.columns = {}
      // Keep rules of columns with the same name, and map the others to a
      // source column with a matching name ("Order date" → order_date).
      let kept = 0, matched = 0
      for (const c of t.columns) {
        if (old[c.name]) {
          mapping.columns[c.name] = old[c.name]
          kept++
          continue
        }
        const s = sources.find((s) => M.targetName(s.label) === c.name || s.label.toLowerCase() === c.name.toLowerCase())
        if (s) {
          mapping.columns[c.name] = s.index ? { source_index: s.index } : { source: s.name }
          matched++
        }
      }
      targetPath = path
      const left = t.columns.length - kept - matched
      toast(left ? 'info' : 'ok', `Target ${t.name || 'loaded'}: ${t.columns.length} columns, ${kept + matched} mapped by name${left ? `, ${left} still to map` : ''}.`)
    } catch (e) {
      toast('error', String(e))
    }
  }
  async function storeInFile() {
    const path = await api.chooseSaveFile('Store the target in its own file', (mapping.target.inline.name || mapping.name || 'target') + '.target.yaml', 'mapping')
    if (path) targetPath = path
  }
</script>

<Section icon={Tag} title="Mapping" hint="The name appears in logs, reports and on converters (--info).">
  <div class="grid two">
    <label><span>Name <span class="required-mark">*</span></span>
      <input bind:value={mapping.name} class:invalid={!mapping.name?.trim()} placeholder="e.g. acme-orders" />
    </label>
    <label>Description <input value={mapping.description ?? ''} oninput={(e) => (mapping.description = e.target.value)} placeholder="optional" /></label>
  </div>
</Section>

<Section icon={Target} title="Target" hint="The format the receiving software expects: its columns and its file format.">
  <div class="choices">
    <button class="choice" class:on={mode === 'inline'} onclick={() => (targetPath = '')}>
      <FilePlus size={18} />
      <span><span class="title">Define it in this mapping</span><span class="desc">Start from the columns of this file and shape them in the next step.</span></span>
    </button>
    <button class="choice" class:on={mode === 'file'} onclick={useTarget}>
      <FolderOpen size={18} />
      <span><span class="title">Use an existing target file…</span><span class="desc">The same output format for several sources. Columns with matching names are mapped automatically.</span></span>
    </button>
  </div>
  <div class="row target-file">
    {#if targetPath}
      <FileCog size={15} />
      <span class="mono" title={targetPath}>{targetPath.split(/[\\/]/).pop()}</span>
      <span class="muted">saving updates this file, for every mapping using it</span>
    {:else}
      <button class="link" onclick={storeInFile}>Store the target in its own file instead…</button>
    {/if}
  </div>
  <div class="grid two">
    <label>Target name <input value={mapping.target.inline.name ?? ''} oninput={(e) => (mapping.target.inline.name = e.target.value)} placeholder="optional, e.g. erp-orders" /></label>
  </div>
</Section>

<Section icon={FileOutput} title="Output file" hint="How converted files are written.">
  <div class="grid">
    <label>Encoding
      <select bind:value={out.encoding}>
        {#each ['utf-8', 'utf-16le', 'utf-16be', 'windows-1252', 'latin-1', 'iso-8859-15'] as e}<option value={e}>{e}</option>{/each}
      </select>
    </label>
    <label>Delimiter
      <span class="segmented">
        {#each [[',', ','], [';', ';'], ['tab', 'tab'], ['|', '|']] as [v, l]}
          <button class:on={out.delimiter === v || (v === 'tab' && out.delimiter === '\t')} onclick={() => (out.delimiter = v)}>{l}</button>
        {/each}
      </span>
    </label>
    <label>Line endings
      <span class="segmented">
        <button class:on={out.line_ending === 'crlf'} onclick={() => (out.line_ending = 'crlf')} title="Windows">CRLF</button>
        <button class:on={out.line_ending === 'lf'} onclick={() => (out.line_ending = 'lf')} title="Unix, macOS">LF</button>
      </span>
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
    <label>Empty values written as <input value={out.null_value ?? ''} oninput={(e) => (out.null_value = e.target.value)} placeholder="nothing" /></label>
    <label class="inline"><input type="checkbox" bind:checked={out.header} /> Header row</label>
    <label class="inline"><input type="checkbox" bind:checked={out.bom} /> Byte order mark (BOM)</label>
  </div>
</Section>

<Section icon={ShieldAlert} title="Invalid values" hint="What happens when a value cannot be converted. Columns can override it.">
  <div class="choices three">
    {#each [['fail_file', 'Fail the whole file', 'Safest: nothing is written until the data is fixed.'],
            ['skip_row', 'Reject the row', 'Other rows are converted; rejected ones go to a reject file.'],
            ['default', 'Use the default', 'The value is replaced by the column default (or left empty).']] as [v, t, d]}
      <button class="choice" class:on={errs.strategy === v} onclick={() => (errs.strategy = v)}>
        <span><span class="title">{t}</span><span class="desc">{d}</span></span>
      </button>
    {/each}
  </div>
  <div class="grid">
    <label class="inline"><input type="checkbox" bind:checked={errs.reject_file} /> Write rejected rows to a reject file</label>
    <label>Fail if this share of rows has errors
      <input type="number" min="0" max="1" step="0.01" value={errs.max_error_rate ?? ''} oninput={(e) => setNumber(errs, 'max_error_rate', e.target.value)} placeholder="no limit (0.01 = 1%)" />
    </label>
    <label>Fail after this many rows with errors
      <input type="number" min="0" value={errs.max_errors ?? ''} oninput={(e) => setNumber(errs, 'max_errors', e.target.value)} placeholder="no limit" />
    </label>
  </div>
</Section>

<Section icon={ScrollText} title="Logging" hint="What conversions report, on screen or in a log file.">
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
</Section>

<style>
  .two { grid-template-columns: 1fr 1fr; }
  .choices { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 8px; }
  .choices.three { grid-template-columns: 1fr 1fr 1fr; }
  .target-file { gap: 6px; margin-bottom: 8px; font-size: 12px; }
  input.invalid { border-color: var(--error); }
  label .segmented { align-self: flex-start; }
</style>
