<script>
  // The file format of a target: how converted files are written. Edits the
  // output object in place.
  let { output: out } = $props()
</script>

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

<style>
  label .segmented { align-self: flex-start; }
</style>
