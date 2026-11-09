<script lang="ts">
	import BoxArt from '$lib/components/BoxArt.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import NewPill from '$lib/components/NewPill.svelte';
	import PlatformMark from '$lib/components/PlatformMark.svelte';
	import { t } from '$lib/i18n/index.svelte';
	import { groupNeedsLink, openCount, type NeedsLinkGroup } from '$lib/needs-link';
	import { groupID, sample, type VariantId } from './_data';

	let { id }: { id: VariantId } = $props();

	/** Dismissed account:campaign pairs, as dismissed_at would hold them. */
	let hidden = $state<string[]>([]);
	/** A: groups folded to an Undo line this load. */
	let folded = $state<string[]>([]);
	/** B: the last hidden group, while its Undo toast shows. */
	let toast = $state<{ gid: string; name: string } | null>(null);
	let toastTimer: ReturnType<typeof setTimeout> | undefined;
	/** C: the Hidden list is open. */
	let showHidden = $state(false);
	let checking = $state<string | null>(null);
	let checked = $state<string[]>([]);

	const all = groupNeedsLink(sample);
	const shown = $derived(
		all.filter((g) => !hidden.includes(groupID(g)) || (id === 'a' && folded.includes(groupID(g))))
	);
	const hiddenGroups = $derived(all.filter((g) => hidden.includes(groupID(g))));
	const open = $derived(openCount(shown.filter((g) => !hidden.includes(groupID(g)))));
	const hiddenDrops = $derived(hiddenGroups.reduce((n, g) => n + g.drops.length, 0));

	function title(g: NeedsLinkGroup): string {
		return g.campaign_name || g.game;
	}

	function dismiss(g: NeedsLinkGroup): void {
		const gid = groupID(g);
		hidden = [...hidden, gid];
		if (id === 'a') folded = [...folded, gid];
		if (id === 'b') {
			clearTimeout(toastTimer);
			toast = { gid, name: title(g) };
			toastTimer = setTimeout(() => (toast = null), 6000);
		}
	}

	function restore(gid: string): void {
		hidden = hidden.filter((x) => x !== gid);
		folded = folded.filter((x) => x !== gid);
		if (toast?.gid === gid) {
			clearTimeout(toastTimer);
			toast = null;
		}
		if (hidden.length === 0) showHidden = false;
	}

	function check(gid: string): void {
		checking = gid;
		setTimeout(() => {
			checking = null;
			checked = [...checked, gid];
		}, 700);
	}

	export function reset(): void {
		hidden = [];
		folded = [];
		toast = null;
		showHidden = false;
		checked = [];
	}
</script>

{#snippet recheck(g: NeedsLinkGroup)}
	{@const gid = groupID(g)}
	<button type="button" class="recheck" disabled={checking === gid} aria-busy={checking === gid} onclick={() => check(gid)}>
		<span class="ic" class:spin={checking === gid}><Icon name="refresh" size={12} /></span>{checking === gid
			? t('games.rechecking')
			: t('games.recheck')}
	</button>
	{#if checked.includes(gid)}<span class="res" role="status">{t('games.recheck_still')}</span>{/if}
{/snippet}

{#snippet head(g: NeedsLinkGroup, dim = false)}
	<div class="what">
		<div class="t">{title(g)}</div>
		<div class="c">
			{#if g.campaign_name && g.game}<span>{g.game}</span>{/if}
			<span class="acct"><PlatformMark platform={g.platform} size="sm" />{g.account_name}</span>
			{#if g.expired}<span class="tag">{t('needslink.expired')}</span>{/if}
			{#if dim}<span class="tag">{g.drops.length} drops</span>{/if}
		</div>
	</div>
{/snippet}

{#snippet list(g: NeedsLinkGroup)}
	<ul class="drops">
		{#each g.drops as d (d.benefit_id)}
			<li>
				<span class="img"><BoxArt src={d.image_url || null} name={d.benefit_name} size="sm" /></span>
				<span class="nm">{d.benefit_name}</span>
			</li>
		{/each}
	</ul>
{/snippet}

{#if shown.length > 0 || (id === 'c' && hiddenGroups.length > 0)}
	<section id="needs-link" class="sec" aria-labelledby="needs-link-h">
		<div class="sec-h">
			<h2 id="needs-link-h">
				<span class="ic amber"><Icon name="alert" size={14} /></span>{t('needslink.title')}
				{#if open > 0}<span class="n">{open}</span>{/if}<NewPill since="2.3" />
			</h2>
		</div>
		<p class="why">{t('needslink.why')}</p>
		<div class="panel">
			{#each shown as g (g.key)}
				{@const gid = groupID(g)}
				{#if id === 'a' && folded.includes(gid)}
					<div class="grp gone" role="status">
						<span>Dismissed <b>{title(g)}</b></span>
						<button type="button" class="link" onclick={() => restore(gid)}>Undo</button>
					</div>
				{:else}
					<div class="grp" class:closed={g.expired}>
						<div class="gh" class:with-x={id === 'b'}>
							{@render head(g)}
							{#if id === 'b'}
								<div class="acts">
									{#if !g.expired}
										{#if g.href}<a class="btn go" href={g.href} target="_blank" rel="noopener noreferrer">{t('needslink.connect')}</a>{/if}
										{@render recheck(g)}
									{/if}
								</div>
								<button type="button" class="x" aria-label="Dismiss {title(g)}" title="Dismiss" onclick={() => dismiss(g)}>
									<Icon name="x" size={14} />
								</button>
							{:else}
								<div class="acts">
									{#if !g.expired}
										{#if g.href}<a class="btn go" href={g.href} target="_blank" rel="noopener noreferrer">{t('needslink.connect')}</a>{/if}
										{@render recheck(g)}
									{/if}
									<button type="button" class="quiet" onclick={() => dismiss(g)}>{id === 'c' ? 'Hide' : 'Dismiss'}</button>
								</div>
							{/if}
						</div>
						{@render list(g)}
					</div>
				{/if}
			{/each}
			{#if id === 'c' && hiddenGroups.length > 0}
				<div class="hid">
					<button type="button" class="hid-t" aria-expanded={showHidden} onclick={() => (showHidden = !showHidden)}>
						<span>{hiddenGroups.length} hidden · {hiddenDrops} drops</span>
						<span class="chev" class:up={showHidden}><Icon name="chevron-down" size={14} /></span>
					</button>
					{#if showHidden}
						{#each hiddenGroups as g (g.key)}
							<div class="hrow">
								{@render head(g, true)}
								<button type="button" class="quiet" onclick={() => restore(groupID(g))}>Show again</button>
							</div>
						{/each}
					{/if}
				</div>
			{/if}
		</div>
	</section>
{:else}
	<p class="empty">The Needs link section is gone: nothing waits. Use Reset to bring the groups back.</p>
{/if}

{#if id === 'b' && toast}
	<div class="toast" role="status">
		<span>Dismissed <b>{toast.name}</b></span>
		<button type="button" class="link" onclick={() => toast && restore(toast.gid)}>Undo</button>
	</div>
{/if}

<style>
	.sec {
		min-width: 0;
	}
	.sec-h {
		display: flex;
		align-items: center;
		gap: 8px;
		min-height: 26px;
		flex-wrap: wrap;
	}
	.sec-h h2 {
		margin: 0;
		font-size: 13px;
		font-weight: 600;
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.ic {
		display: inline-flex;
	}
	.amber {
		color: var(--amber);
	}
	.n {
		font: 400 12px var(--mono);
		color: var(--t2);
		background: var(--surface-3);
		border-radius: 10px;
		padding: 0 6px;
	}
	.why {
		margin: 2px 0 8px;
		font-size: 12px;
		color: var(--t2);
	}
	.panel {
		background: var(--surface);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius);
		overflow: hidden;
	}
	.grp + .grp,
	.grp + .hid {
		border-top: 1px solid var(--border-soft);
	}
	.gh {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 10px 14px;
		background: color-mix(in srgb, var(--amber) 8%, var(--surface));
	}
	.closed .gh {
		background: var(--surface-2);
	}
	.what {
		min-width: 0;
	}
	.t {
		font-size: 13px;
		font-weight: 500;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.c {
		font-size: 12px;
		color: var(--t2);
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 3px;
		flex-wrap: wrap;
	}
	.acct {
		display: inline-flex;
		align-items: center;
		gap: 5px;
	}
	.tag {
		font-size: 11px;
		color: var(--t2);
		background: var(--surface-3);
		border-radius: 4px;
		padding: 0 6px;
	}
	.acts {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		justify-content: flex-end;
	}
	.btn {
		display: inline-flex;
		align-items: center;
		font-size: 12px;
		font-weight: 500;
		background: var(--surface-2);
		border: 1px solid var(--accent);
		border-radius: 6px;
		padding: 1px 9px;
		color: var(--accent);
		white-space: nowrap;
		text-decoration: none;
	}
	.btn:hover {
		background: var(--accent-soft);
	}
	button:focus-visible,
	.btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.recheck {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		background: none;
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 1px 7px;
		font: inherit;
		font-size: 12px;
		color: var(--t2);
		cursor: pointer;
		white-space: nowrap;
	}
	.recheck:hover:not(:disabled) {
		color: var(--text);
		border-color: var(--t2);
	}
	.recheck:disabled {
		cursor: default;
		opacity: 0.7;
	}
	.spin {
		animation: spin 0.9s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.spin {
			animation: none;
		}
	}
	.res {
		font-size: 12px;
		color: var(--t2);
	}
	/* Dismiss / Hide / Show again: text only, quieter than Check now. */
	.quiet {
		background: none;
		border: 0;
		padding: 1px 4px;
		font: inherit;
		font-size: 12px;
		color: var(--t2);
		cursor: pointer;
		border-radius: 6px;
		white-space: nowrap;
	}
	.quiet:hover {
		color: var(--text);
		background: var(--surface-3);
	}
	.link {
		background: none;
		border: 0;
		padding: 0 2px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		color: var(--accent);
		cursor: pointer;
		border-radius: 4px;
	}
	.link:hover {
		text-decoration: underline;
		text-underline-offset: 2px;
	}
	/* B: the x sits top right, in line with the title. */
	.gh.with-x {
		align-items: flex-start;
	}
	.gh.with-x .what {
		flex: 1;
		align-self: center;
	}
	.gh.with-x .acts {
		align-self: center;
	}
	.x {
		flex: none;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		margin: -3px -6px 0 -8px;
		background: none;
		border: 0;
		border-radius: 6px;
		color: var(--t2);
		cursor: pointer;
	}
	.x:hover {
		color: var(--text);
		background: color-mix(in srgb, var(--text) 8%, transparent);
	}
	/* A: a dismissed group folds to one line until the next load. */
	.gone {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 9px 14px;
		font-size: 12px;
		color: var(--t2);
		background: var(--surface-2);
		min-width: 0;
	}
	.gone > span {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.gone b,
	.toast b {
		font-weight: 500;
		color: var(--text);
	}
	/* C: the hidden list at the foot of the panel. */
	.hid {
		background: var(--surface-2);
	}
	.hid-t {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: 8px 14px;
		background: none;
		border: 0;
		font: inherit;
		font-size: 12px;
		color: var(--t2);
		cursor: pointer;
		text-align: left;
	}
	.hid-t:hover {
		color: var(--text);
	}
	.chev {
		display: inline-flex;
		transition: transform 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.chev.up {
		transform: rotate(180deg);
	}
	.hrow {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 8px 14px;
		border-top: 1px solid var(--border-soft);
	}
	.hrow .what {
		opacity: 0.7;
	}
	.drops {
		list-style: none;
		margin: 0;
		padding: 4px 0;
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
	}
	.drops li {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 6px 14px;
		min-width: 0;
	}
	.closed .drops li {
		opacity: 0.6;
	}
	.img {
		flex: none;
		width: 32px;
		height: 32px;
		border-radius: 4px;
		overflow: hidden;
		display: block;
	}
	.img :global(.art) {
		min-height: 0;
	}
	.nm {
		font-size: 13px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		min-width: 0;
	}
	.toast {
		position: fixed;
		left: 50%;
		bottom: 24px;
		transform: translateX(-50%);
		z-index: 50;
		display: flex;
		align-items: center;
		gap: 14px;
		max-width: calc(100vw - 32px);
		padding: 10px 14px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--surface-2);
		color: var(--t2);
		font-size: 13px;
		box-shadow: 0 6px 20px rgb(0 0 0 / 0.25);
		animation: rise 220ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.toast > span {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	@keyframes rise {
		from {
			opacity: 0;
			transform: translate(-50%, 8px);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.toast {
			animation: none;
		}
	}
	.empty {
		font-size: 12px;
		color: var(--t2);
		margin: 0;
	}
	/* Phone layout, also forced by the sandbox 390 px frame. */
	@media (max-width: 720px) {
		.gh:not(.with-x) {
			flex-direction: column;
			align-items: flex-start;
			gap: 8px;
		}
		.gh.with-x {
			flex-wrap: wrap;
			gap: 8px;
		}
		.gh.with-x .acts {
			order: 3;
			flex-basis: 100%;
			justify-content: flex-start;
		}
		.acts {
			justify-content: flex-start;
		}
		.drops {
			grid-template-columns: minmax(0, 1fr);
		}
	}
	:global(.frame.phone) .gh:not(.with-x) {
		flex-direction: column;
		align-items: flex-start;
		gap: 8px;
	}
	:global(.frame.phone) .gh.with-x {
		flex-wrap: wrap;
		gap: 8px;
	}
	:global(.frame.phone) .gh.with-x .acts {
		order: 3;
		flex-basis: 100%;
		justify-content: flex-start;
	}
	:global(.frame.phone) .acts {
		justify-content: flex-start;
	}
	:global(.frame.phone) .drops {
		grid-template-columns: minmax(0, 1fr);
	}
</style>
