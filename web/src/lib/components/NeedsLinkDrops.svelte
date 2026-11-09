<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { api } from '$lib/app';
	import { ApiError } from '$lib/api/client';
	import type { NeedsLinkDrop } from '$lib/api/types';
	import BoxArt from '$lib/components/BoxArt.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import LinkRecheck from '$lib/components/LinkRecheck.svelte';
	import NewPill from '$lib/components/NewPill.svelte';
	import PlatformMark from '$lib/components/PlatformMark.svelte';
	import { t } from '$lib/i18n/index.svelte';
	import { getLive } from '$lib/live-context';
	import { dismissKey, groupNeedsLink, needsLinkRows, openCount, type NeedsLinkGroup } from '$lib/needs-link';

	const live = getLive();

	let drops = $state<NeedsLinkDrop[]>([]);
	let loaded = false;
	let seq = 0;

	async function load(): Promise<void> {
		const mine = ++seq;
		let next: NeedsLinkDrop[];
		try {
			next = (await api.needsLink()).drops;
		} catch {
			return;
		}
		if (mine !== seq) return;
		drops = next;
		if (!loaded) {
			loaded = true;
			if (next.length > 0 && location.hash === '#needs-link') {
				await tick();
				document.getElementById('needs-link')?.scrollIntoView();
			}
		}
	}

	// Scans and claims arrive over SSE: reload so a claimed drop leaves the list.
	$effect(() => {
		void live?.games;
		void live?.recent;
		untrack(() => void load());
	});

	$effect(() => {
		const reload = setInterval(() => void load(), 15 * 60_000);
		return () => clearInterval(reload);
	});

	/** Groups dismissed on this visit, by dismissKey: they fold into an Undo line until the page is left. */
	let dismissed = $state(new Map<string, NeedsLinkGroup>());
	let busy = $state(new Set<string>());
	/** The failed action per dismissKey, shown like Check now's result. */
	let failed = $state(new Map<string, 'dismiss' | 'undo'>());

	function setBusy(key: string, on: boolean): void {
		const next = new Set(busy);
		if (on) next.add(key);
		else next.delete(key);
		busy = next;
	}

	function setFailed(key: string, what: 'dismiss' | 'undo' | null): void {
		const next = new Map(failed);
		if (what) next.set(key, what);
		else next.delete(key);
		failed = next;
	}

	async function dismiss(g: NeedsLinkGroup): Promise<void> {
		const key = dismissKey(g);
		setBusy(key, true);
		setFailed(key, null);
		try {
			await api.dismissNeedsLink(g);
		} catch (e) {
			// 404: nothing of it is left to hide, so fold it all the same.
			if (!(e instanceof ApiError && e.status === 404)) {
				setFailed(key, 'dismiss');
				return;
			}
		} finally {
			setBusy(key, false);
		}
		dismissed = new Map(dismissed).set(key, g);
	}

	async function undo(key: string, g: NeedsLinkGroup): Promise<void> {
		setBusy(key, true);
		setFailed(key, null);
		try {
			await api.restoreNeedsLink(g);
		} catch (e) {
			// 404: claimed or pruned meanwhile, so there is nothing to show again.
			if (!(e instanceof ApiError && e.status === 404)) {
				setFailed(key, 'undo');
				return;
			}
		} finally {
			setBusy(key, false);
		}
		const next = new Map(dismissed);
		next.delete(key);
		dismissed = next;
		await load();
	}

	const groups = $derived(groupNeedsLink(drops));
	const rows = $derived(needsLinkRows(groups, dismissed));
	const open = $derived(openCount(rows.flatMap((r) => (r.kind === 'group' ? [r.group] : []))));
</script>

{#snippet failure(key: string)}
	{@const what = failed.get(key)}
	{#if what}<span class="res" role="status">{t(what === 'undo' ? 'needslink.undo_failed' : 'needslink.dismiss_failed')}</span>{/if}
{/snippet}

{#if rows.length > 0}
	<section id="needs-link" class="sec" aria-labelledby="needs-link-h">
		<div class="sec-h">
			<h2 id="needs-link-h">
				<span class="ic"><Icon name="alert" size={14} /></span>{t('needslink.title')}
				{#if open > 0}<span class="n">{open}</span>{/if}<NewPill since="2.3" />
			</h2>
		</div>
		<p class="why">{t('needslink.why')}</p>
		<div class="panel">
			{#each rows as r (r.kind === 'group' ? r.group.key : `dismissed:${r.key}`)}
				{@const g = r.group}
				{@const key = dismissKey(g)}
				{#if r.kind === 'dismissed'}
					<div class="grp gone">
						<span class="gone-t">{t('needslink.dismissed')} <b>{g.campaign_name || g.game}</b></span>
						<button type="button" class="undo" disabled={busy.has(key)} onclick={() => undo(key, g)}>{t('needslink.undo')}</button>
						{@render failure(key)}
					</div>
				{:else}
					<div class="grp" class:closed={g.expired}>
						<div class="gh">
							<div class="what">
								<div class="t">{g.campaign_name || g.game}</div>
								<div class="c">
									{#if g.campaign_name && g.game}<span>{g.game}</span>{/if}
									<span class="acct"><PlatformMark platform={g.platform} size="sm" />{g.account_name}</span>
									{#if g.expired}<span class="tag">{t('needslink.expired')}</span>{/if}
								</div>
							</div>
							<div class="acts">
								{#if !g.expired}
									{#if g.href}<a class="btn go" href={g.href} target="_blank" rel="noopener noreferrer">{t('needslink.connect')}</a>{/if}
									<LinkRecheck accountIds={[g.account_id]} />
								{/if}
								<button type="button" class="quiet" disabled={busy.has(key)} onclick={() => dismiss(g)}>{t('needslink.dismiss')}</button>
								{@render failure(key)}
							</div>
						</div>
						<ul class="drops">
							{#each g.drops as d (d.benefit_id)}
								<li>
									<span class="img"><BoxArt src={d.image_url || null} name={d.benefit_name} size="sm" /></span>
									<span class="nm">{d.benefit_name}</span>
								</li>
							{/each}
						</ul>
					</div>
				{/if}
			{/each}
		</div>
	</section>
{/if}

<style>
	.sec {
		margin-top: 28px;
		min-width: 0;
		scroll-margin-top: calc(var(--top-h) + 20px);
	}
	.sec:first-child {
		margin-top: 0;
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
	.grp + .grp {
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
	.btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.quiet {
		background: none;
		border: 0;
		border-radius: 6px;
		padding: 1px 4px;
		font: inherit;
		font-size: 12px;
		color: var(--t2);
		cursor: pointer;
		white-space: nowrap;
	}
	.quiet:hover:not(:disabled) {
		color: var(--text);
		background: var(--surface-3);
	}
	.quiet:disabled,
	.undo:disabled {
		cursor: default;
		opacity: 0.7;
	}
	.quiet:focus-visible,
	.undo:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.res {
		font-size: 12px;
		color: var(--t2);
	}
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
	.gone-t {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.gone-t b {
		font-weight: 500;
		color: var(--text);
	}
	.undo {
		flex: none;
		background: none;
		border: 0;
		border-radius: 4px;
		padding: 0 2px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		color: var(--accent);
		cursor: pointer;
	}
	.undo:hover:not(:disabled) {
		text-decoration: underline;
		text-underline-offset: 2px;
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
	@media (max-width: 720px) {
		.gh {
			flex-direction: column;
			align-items: flex-start;
			gap: 8px;
		}
		.acts {
			justify-content: flex-start;
		}
		.drops {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
