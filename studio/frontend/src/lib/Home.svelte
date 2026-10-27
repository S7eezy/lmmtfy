<script>
  // The home screen: what lmmtfy does in one line and one picture, then the
  // two kinds of files side by side, each with its main action first and its
  // recent files. It fits the window: recent files scroll inside their
  // panel, and the hero shrinks on short windows.
  import { FilePlus2, FolderOpen, FileSpreadsheet, FileCog, Target, Workflow, Upload, ArrowRight, FileCheck2, Clock } from '@lucide/svelte'

  let { recent = [], version = '', onnewmapping, onopenmapping, onopencsv, onnewtarget, onopentarget } = $props()

  const fileName = (p) => p.split(/[\\/]/).pop()
  const dirName = (p) => p.slice(0, -fileName(p).length).replace(/[\\/]$/, '')
  const mappings = $derived(recent.filter((r) => r.kind !== 'target').slice(0, 10))
  const targets = $derived(recent.filter((r) => r.kind === 'target').slice(0, 10))

  function ago(iso) {
    const s = (Date.now() - new Date(iso).getTime()) / 1000
    if (!(s >= 0)) return ''
    if (s < 60) return 'just now'
    if (s < 3600) return `${Math.floor(s / 60)} min ago`
    if (s < 86400) return `${Math.floor(s / 3600)} h ago`
    if (s < 2 * 86400) return 'yesterday'
    if (s < 7 * 86400) return `${Math.floor(s / 86400)} days ago`
    return new Date(iso).toLocaleDateString()
  }
  const isMac = typeof navigator !== 'undefined' && /Mac/.test(navigator.platform)
</script>

{#snippet item(r, open, Icon)}
  <button class="item" title={r.path} onclick={() => open(r.path)}>
    <span class="item-icon"><Icon size={15} /></span>
    <span class="item-text"><span class="name">{fileName(r.path)}</span><span class="dir"><bdi>{dirName(r.path)}</bdi></span></span>
    <span class="when">{ago(r.opened)}</span>
  </button>
{/snippet}

<div class="home">
  <header class="hero">
    <img class="logo" src="/icon.png" alt="" width="68" height="68" />
    <h1>Let Me Map That For You</h1>
    <p class="tagline">Messy CSV in. The exact format your software expects out.</p>

    <div class="flow" aria-label="How it works">
      <span class="node">
        <span class="node-icon source"><FileSpreadsheet size={16} /></span>
        <span><strong>Any CSV</strong><small>partner exports, as they come</small></span>
      </span>
      <span class="link"><span class="line"></span><span class="label"><Workflow size={12} /> mapping</span><ArrowRight size={14} /></span>
      <span class="node">
        <span class="node-icon target"><Target size={16} /></span>
        <span><strong>Your target</strong><small>columns, types, file format</small></span>
      </span>
      <span class="link"><span class="line"></span><span class="label">convert</span><ArrowRight size={14} /></span>
      <span class="node">
        <span class="node-icon done"><FileCheck2 size={16} /></span>
        <span><strong>Clean file</strong><small>here, CLI or standalone converter</small></span>
      </span>
    </div>
  </header>

  <div class="groups">
    <section class="group">
      <div class="group-head">
        <span class="group-icon"><Workflow size={18} /></span>
        <div><h2>Mappings</h2><p>How one source becomes your target.</p></div>
      </div>
      <button class="action primary" onclick={() => onnewmapping()}>
        <FilePlus2 size={20} />
        <span><strong>New mapping from a CSV file</strong><small>Read it, choose its target, map its columns.</small></span>
        <ArrowRight size={16} class="go" />
      </button>
      <button class="action" onclick={() => onopenmapping()}>
        <FolderOpen size={18} />
        <span><strong>Open a mapping</strong><small>Edit it, convert files, export converters.</small></span>
      </button>
      <div class="recent">
        <h3><Clock size={12} /> Recent</h3>
        <div class="recent-list">
          {#each mappings as r}
            {@render item(r, r.kind === 'mapping' ? onopenmapping : onopencsv, r.kind === 'mapping' ? FileCog : FileSpreadsheet)}
          {:else}
            <p class="empty">Mappings and CSV files you open show up here.</p>
          {/each}
        </div>
      </div>
    </section>

    <section class="group">
      <div class="group-head">
        <span class="group-icon"><Target size={18} /></span>
        <div><h2>Targets</h2><p>The format your software expects, shared by mappings.</p></div>
      </div>
      <button class="action primary soft" onclick={() => onnewtarget()}>
        <FilePlus2 size={20} />
        <span><strong>New target</strong><small>From an example of the expected file, or blank.</small></span>
        <ArrowRight size={16} class="go" />
      </button>
      <button class="action" onclick={() => onopentarget()}>
        <FolderOpen size={18} />
        <span><strong>Open a target</strong><small>Change its columns or file format.</small></span>
      </button>
      <div class="recent">
        <h3><Clock size={12} /> Recent</h3>
        <div class="recent-list">
          {#each targets as r}
            {@render item(r, onopentarget, Target)}
          {:else}
            <p class="empty">Targets you open or save show up here.</p>
          {/each}
        </div>
      </div>
    </section>
  </div>

  <footer class="foot">
    <span><Upload size={13} /> Drop a CSV file, a mapping or a target on the window</span>
    <span class="dot">·</span>
    <span><kbd>{isMac ? '⌘' : 'Ctrl'}</kbd> <kbd>S</kbd> saves</span>
    {#if version}<span class="dot">·</span><span class="version">{version === 'dev' ? 'development build' : `v${version.replace(/^v/, '')}`}</span>{/if}
  </footer>
</div>

<style>
  .home { height: calc(100% + 24px); overflow: auto; display: flex; flex-direction: column; align-items: center; padding: 3.5vh 16px 12px;
    background: radial-gradient(ellipse 60% 38% at 50% 0%, color-mix(in srgb, var(--accent) 13%, transparent), transparent 70%); margin: -12px; }

  .hero { text-align: center; max-width: 900px; flex-shrink: 0; }
  .logo { border-radius: 17px; box-shadow: 0 10px 30px color-mix(in srgb, var(--accent) 35%, transparent); }
  h1 { font-size: 30px; letter-spacing: -0.02em; margin: 14px 0 4px; }
  .tagline { font-size: 15px; color: var(--muted); margin: 0; }

  .flow { display: flex; align-items: center; justify-content: center; gap: 6px; margin: 24px 0 0; flex-wrap: wrap; }
  .node { display: inline-flex; gap: 9px; align-items: center; text-align: left; padding: 8px 14px 8px 9px; border-radius: 12px;
    background: var(--panel); border: 1px solid var(--line); box-shadow: 0 1px 2px rgb(0 0 0 / 0.04); }
  .node strong { display: block; font-size: 13px; }
  .node small { display: block; font-size: 11px; color: var(--muted); }
  .node-icon { display: inline-flex; width: 30px; height: 30px; border-radius: 8px; align-items: center; justify-content: center; }
  .node-icon.source { background: var(--fair-soft); color: var(--fair); }
  .node-icon.target { background: var(--soft); color: var(--accent); }
  .node-icon.done { background: var(--high-soft); color: var(--high); }
  .link { display: inline-flex; align-items: center; position: relative; width: 92px; color: var(--muted); }
  .link .line { flex: 1; border-top: 1.5px dashed color-mix(in srgb, var(--muted) 55%, transparent); }
  .link .label { position: absolute; left: 0; right: 14px; top: -17px; display: inline-flex; gap: 3px; align-items: center; justify-content: center; font-size: 10.5px; font-weight: 600; letter-spacing: 0.02em; }

  .groups { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); grid-template-rows: minmax(0, 1fr); gap: 18px; width: 100%; max-width: 940px; margin: 3vh 0 12px; flex: 1; min-height: 300px; max-height: 620px; }
  .group { display: flex; flex-direction: column; gap: 8px; min-width: 0; min-height: 0; padding: 14px 16px; border-radius: 16px; background: var(--panel); border: 1px solid var(--line);
    box-shadow: 0 1px 3px rgb(0 0 0 / 0.04), 0 10px 30px rgb(0 0 0 / 0.04); }
  .group-head { display: flex; gap: 11px; align-items: center; margin-bottom: 6px; }
  .group-icon { display: inline-flex; width: 36px; height: 36px; border-radius: 10px; align-items: center; justify-content: center; background: var(--soft); color: var(--accent); flex-shrink: 0; }
  .group h2 { font-size: 16px; margin: 0; }
  .group-head p { margin: 1px 0 0; font-size: 12px; color: var(--muted); }

  .action { display: flex; gap: 12px; align-items: center; text-align: left; padding: 11px 14px; border-radius: 11px; width: 100%;
    transition: transform 0.12s, box-shadow 0.12s, border-color 0.12s; color: var(--text); }
  .action :global(svg) { flex-shrink: 0; color: var(--accent); }
  .action strong { display: block; font-size: 13.5px; }
  .action small { display: block; font-size: 12px; color: var(--muted); }
  .action > span { flex: 1; min-width: 0; }
  .action:hover { transform: translateY(-1px); box-shadow: 0 6px 18px rgb(0 0 0 / 0.08); }
  .action.primary { padding: 13px 16px; background: var(--accent); border-color: var(--accent); color: var(--accent-text); }
  .action.primary :global(svg), .action.primary small { color: inherit; }
  .action.primary small { opacity: 0.85; }
  .action.primary.soft { background: var(--soft); border-color: color-mix(in srgb, var(--accent) 30%, transparent); color: var(--text); }
  .action.primary.soft :global(svg) { color: var(--accent); }
  .action.primary.soft small { color: var(--muted); opacity: 1; }
  .action :global(.go) { opacity: 0.6; transition: transform 0.12s, opacity 0.12s; }
  .action:hover :global(.go) { opacity: 1; transform: translateX(2px); }

  .recent { margin-top: 6px; border-top: 1px solid var(--line); padding-top: 4px; flex: 1; min-height: 0; display: flex; flex-direction: column; }
  .recent-list { flex: 1; min-height: 0; overflow: auto; }
  .recent h3 { display: flex; gap: 5px; align-items: center; margin: 8px 2px 4px; }
  .item { display: flex; gap: 10px; align-items: center; width: 100%; padding: 5px 8px; border: none; background: none; border-radius: 8px; text-align: left; color: var(--text); }
  .item:hover { background: var(--soft); }
  .item-icon { display: inline-flex; color: var(--accent); flex-shrink: 0; }
  .item-text { display: flex; flex-direction: column; min-width: 0; flex: 1; }
  .item .name { font-weight: 600; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .item .dir { font-family: var(--mono); font-size: 11px; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; direction: rtl; text-align: left; }
  .item .when { font-size: 11px; color: var(--muted); white-space: nowrap; flex-shrink: 0; }
  .empty { margin: 4px 2px 6px; font-size: 12px; color: var(--muted); }

  .foot { flex-shrink: 0; display: flex; gap: 8px; align-items: center; flex-wrap: wrap; justify-content: center; font-size: 12px; color: var(--muted); padding-top: 4px; }
  .foot span { display: inline-flex; gap: 5px; align-items: center; }
  .dot { opacity: 0.5; }
  .version { font-family: var(--mono); font-size: 11px; }

  /* Short windows: a smaller hero, then no picture at all. */
  @media (max-height: 800px) {
    .home { padding-top: 2.5vh; }
    .logo { width: 52px; height: 52px; border-radius: 13px; }
    h1 { font-size: 25px; margin-top: 10px; }
    .flow { margin-top: 18px; }
    .node { padding: 6px 12px 6px 7px; }
    .node small { display: none; }
  }
  @media (max-height: 660px) {
    .flow { display: none; }
    .logo { width: 40px; height: 40px; border-radius: 10px; }
  }
  @media (max-width: 860px) {
    .groups { grid-template-columns: minmax(0, 1fr); grid-template-rows: none; max-width: 520px; flex: none; }
    .link { width: 40px; }
    .link .label { display: none; }
  }
</style>
