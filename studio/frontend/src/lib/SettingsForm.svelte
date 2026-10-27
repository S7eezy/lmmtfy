<script>
  // Mapping-wide settings: identity, errors, logging. Used as the Settings
  // step of a new mapping and in the Settings dialog. The target and its
  // file format have their own step.
  import { Tag, ShieldAlert, ScrollText } from '@lucide/svelte'
  import Section from './Section.svelte'

  let { mapping = $bindable() } = $props()

  const errs = $derived(mapping.on_error)

  function setNumber(obj, key, value) {
    if (value === '' || value === null) delete obj[key]
    else obj[key] = Number(value)
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
  input.invalid { border-color: var(--error); }
</style>
