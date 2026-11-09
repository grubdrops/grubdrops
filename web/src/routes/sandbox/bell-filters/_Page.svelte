<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import { badgeText, bellUrgent } from '$lib/notifications';
	import Theme from '../points-page/_Theme.svelte';
	import Panel from './_Panel.svelte';
	import { feed } from './_data';

	type Mode = 'tabs' | 'chips' | 'grouped';
	let { mode }: { mode: Mode } = $props();

	const VARIANTS: { mode: Mode; id: string; href: string; name: string; trade: string; note: string }[] = [
		{
			mode: 'tabs',
			id: 'v1',
			href: '/sandbox/bell-filters/',
			name: 'V1 · Tabs: All, Claims, Alerts',
			trade: 'One tap away from the claim, nothing new to learn. The flood still fills Alerts.',
			note: 'Claims: drops, Kick daily rewards and codes. Alerts: link, login, errors, watch checks. Codes go under Claims because a code is a reward the miner won, not a fault: Alerts stays about fixing the miner. Code rows stay red there, and the Claims count turns red while one waits, as the bell badge does today.'
		},
		{
			mode: 'chips',
			id: 'v2',
			href: '/sandbox/bell-filters/v2',
			name: 'V2 · Filter chips, multi-select',
			trade: 'Finer cuts (Claims plus Codes, Login plus Errors), but five controls to read.',
			note: 'No chip picked shows everything. Each chip shows its unread count; Codes goes red while a code waits. Picking Links alone gives the flood its own view.'
		},
		{
			mode: 'grouped',
			id: 'v3',
			href: '/sandbox/bell-filters/v3',
			name: 'V3 · Tabs plus folded link rows',
			trade: 'Fixes the flood itself: ten rows become one. Adds a chevron to learn.',
			note: 'Needs-link rows for the same account and campaign fold into one row in the slot of the newest. The campaign moves to the second line so it never gets cut. The chevron lists each drop; the row itself opens the link page. Tab counts count a folded row once. The bell badge still counts every row unless the server learns to fold too.'
		}
	];
	const v = $derived(VARIANTS.find((x) => x.mode === mode) ?? VARIANTS[0]);

	const loadedAt = Date.now();
	const items = feed(loadedAt);
	const unread = items.filter((n) => !n.read).length;
	const urgent = items.some((n) => !n.read && bellUrgent(n.kind));
</script>

{#snippet bar()}
	<div class="topbar">
		<span class="brand">GrubDrops</span>
		<span class="bell" aria-hidden="true">
			<Icon name="bell" size={17} />
			<span class="badge" class:urgent>{badgeText(unread)}</span>
		</span>
	</div>
{/snippet}

<main>
	<div class="nav">
		<p>
			<a href="/sandbox/">Sandbox</a> / bell-filters /
			{#each VARIANTS as x, i (x.id)}{i ? ' · ' : ''}{#if x.mode === mode}<b>{x.id}</b>{:else}<a href={x.href}>{x.id}</a>{/if}{/each}
		</p>
		<Theme />
	</div>
	<h1>{v.name}</h1>
	<p class="trade">{v.trade}</p>
	<p class="note">{v.note}</p>
	<p class="note dim">Fake data: ten "Link your account" rows from one Kick account bury the one claim, as on staging.</p>

	<div class="frames">
		<section>
			<p class="where">Desktop</p>
			<div class="frame desk">
				{@render bar()}
				<div class="anchor"><Panel {mode} {items} now={loadedAt} uid="d" /></div>
			</div>
		</section>
		<section>
			<p class="where">Phone, 390px</p>
			<div class="frame phone">
				{@render bar()}
				<div class="anchor"><Panel {mode} {items} now={loadedAt} uid="p" /></div>
			</div>
		</section>
	</div>
</main>

<style>
	main {
		padding: 24px 20px 48px;
		min-height: 100vh;
		max-width: 1200px;
		margin: 0 auto;
	}
	a {
		text-decoration: underline;
	}
	.nav {
		display: flex;
		align-items: center;
		justify-content: space-between;
		flex-wrap: wrap;
		gap: 12px;
	}
	.nav p {
		margin: 0;
	}
	h1 {
		margin: 8px 0 4px;
		font-size: 22px;
	}
	.trade {
		margin: 0 0 4px;
		font-weight: 500;
		color: var(--accent);
		max-width: 80ch;
	}
	.note {
		color: var(--t2);
		margin: 0 0 8px;
		max-width: 80ch;
	}
	.note.dim {
		color: var(--t3);
		margin-bottom: 16px;
	}
	.frames {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 390px;
		gap: 24px;
		align-items: start;
	}
	.where {
		margin: 0 0 6px;
		font-size: 12px;
		color: var(--t3);
	}
	.frame {
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--bg);
		color: var(--text);
		overflow: hidden;
		min-height: 600px;
	}
	.phone {
		width: 390px;
		max-width: 100%;
	}
	.topbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		height: 48px;
		padding: 0 16px;
		border-bottom: 1px solid var(--border-soft);
	}
	.brand {
		font-weight: 650;
	}
	.bell {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 8px;
		color: var(--text);
		background: var(--surface);
	}
	.badge {
		position: absolute;
		top: 2px;
		right: 1px;
		min-width: 16px;
		height: 16px;
		padding: 0 4px;
		box-sizing: border-box;
		border-radius: 999px;
		background: var(--accent-strong);
		color: #fff;
		font-size: 10px;
		font-weight: 650;
		line-height: 16px;
		text-align: center;
		font-variant-numeric: tabular-nums;
		box-shadow: 0 0 0 2px var(--bg);
	}
	.badge.urgent {
		background: var(--red);
	}
	/* Desktop: 360px under the bell, right-aligned. Phone: full width, 16px from each edge. */
	.anchor {
		display: flex;
		justify-content: flex-end;
		padding: 8px 8px 16px;
	}
	.phone .anchor {
		padding: 8px 16px 16px;
	}
	.phone .anchor :global(.pop) {
		width: 100%;
	}
	@media (max-width: 960px) {
		.frames {
			grid-template-columns: minmax(0, 1fr);
		}
	}
	@media (max-width: 720px) {
		main {
			padding: 16px 16px 48px;
		}
		/* On a phone the page is the phone frame: edge to edge, so the panel sits 16px from each side. */
		.frames section:first-child {
			display: none;
		}
		.phone {
			width: auto;
			max-width: none;
			margin: 0 -16px;
			border-width: 1px 0;
			border-radius: 0;
		}
	}
</style>
