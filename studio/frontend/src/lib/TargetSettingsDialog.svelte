<script>
  // A target's name, description and file format, from the target editor
  // or for a target defined with its mapping.
  import { Target, FileOutput } from '@lucide/svelte'
  import Modal from './Modal.svelte'
  import Section from './Section.svelte'
  import OutputForm from './OutputForm.svelte'
  let { target = $bindable(), onclose } = $props()
</script>

<Modal title="Target settings" icon={Target} {onclose} wide>
  <Section icon={Target} title="Target" hint="The name appears in mappings using this target.">
    <div class="grid two">
      <label>Name <input value={target.name ?? ''} oninput={(e) => (target.name = e.target.value)} placeholder="e.g. erp-orders" /></label>
      <label>Description <input value={target.description ?? ''} oninput={(e) => (target.description = e.target.value)} placeholder="optional" /></label>
    </div>
  </Section>
  <Section icon={FileOutput} title="Output file format" hint="How converted files are written.">
    <OutputForm output={target.output} />
  </Section>
  {#snippet footer()}<button class="primary" onclick={onclose}>Done</button>{/snippet}
</Modal>

<style>
  .two { grid-template-columns: 1fr 1fr; }
</style>
