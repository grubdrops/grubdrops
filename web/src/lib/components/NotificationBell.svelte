<script lang="ts">
	import { api } from '$lib/app';
	import type { AppNotification } from '$lib/api/types';
	import { relTime } from '$lib/format';
	import { t, tn } from '$lib/i18n/index.svelte';
	import { getLive } from '$lib/live-context';
	import {
		BELL_TABS,
		applyBellEvent,
		badgeText,
		bellAlert,
		bellEntryUnread,
		bellIcon,
		bellLine,
		bellSections,
		bellTabCount,
		bellTabUrgent,
		bellTarget,
		bellUrgent,
		type BellEntry,
		type BellTab
	} from '$lib/notifications';
	import Icon from './Icon.svelte';
	import NewPill from './NewPill.svelte';

	const live = getLive();

	let open = $state(false);
	let failed = $state(false);
	let now = $state(Date.now());
	// Rows that were unread when the panel opened keep their tint until it closes.
	let fresh = $state(new Set<number>());
	let trigger: HTMLButtonElement | undefined = $state();
	let pop: HTMLDivElement | undefined = $state();
	let tablist: HTMLDivElement | undefined = $state();
	let tab = $state<BellTab>('all');
	// Folded needs-link rows the user opened, by the id of their newest row.
	let expanded = $state(new Set<number>());

	const TAB_LABEL: Record<BellTab, () => string> = {
		all: () => t('bell.tab_all'),
		claims: () => t('bell.tab_claims'),
		alerts: () => t('bell.tab_alerts')
	};

	const items = $derived(live.bell?.items ?? []);
	const unread = $derived(live.bell?.unread ?? 0);
	const badge = $derived(badgeText(unread));
	// The badge turns red while an unread reward code waits.
	const urgentUnread = $derived(items.some((n) => !n.read && bellUrgent(n.kind)));
	const groups = $derived(bellSections(items, tab, now));
	const label = $derived(unread > 0 ? tn('bell.open', unread) : t('bell.title'));

	// Mark the feed read on the server; the badge clears now, the SSE echo is a no-op.
	function markRead(): void {
		if (!live.bell || live.bell.unread === 0) return;
		live.bell = applyBellEvent(live.bell, { read: true });
		void api.readNotifications().catch(() => {});
	}

	async function show(): Promise<void> {
		open = true;
		tab = 'all';
		expanded = new Set();
		now = Date.now();
		failed = false;
		if (!live.bell) {
			try {
				live.bell = await api.notifications();
			} catch {
				failed = true;
				return;
			}
		}
		fresh = new Set(live.bell.items.filter((n) => !n.read).map((n) => n.id));
		markRead();
	}

	function close(refocus: boolean): void {
		if (!open) return;
		open = false;
		fresh = new Set();
		// Rows that arrived while the panel was open were seen too.
		markRead();
		if (refocus) trigger?.focus();
	}

	function toggle(): void {
		if (open) close(false);
		else void show();
	}

	function onWindowPointer(e: PointerEvent): void {
		if (!open) return;
		const n = e.target as Node;
		if (pop?.contains(n) || trigger?.contains(n)) return;
		close(false);
	}

	function onWindowKey(e: KeyboardEvent): void {
		if (open && e.key === 'Escape') close(true);
	}

	// A row that lands while the panel is open stays tinted until it closes.
	$effect(() => {
		const items = live.bell?.items;
		if (!open || !items) return;
		const add = items.filter((n) => !n.read && !fresh.has(n.id));
		if (add.length > 0) fresh = new Set([...fresh, ...add.map((n) => n.id)]);
	});

	// Keep "5 min ago" honest while the panel is open.
	$effect(() => {
		if (!open) return;
		const id = setInterval(() => (now = Date.now()), 30_000);
		return () => clearInterval(id);
	});

	const isUnread = (n: AppNotification) => !n.read || fresh.has(n.id);

	function flip(id: number): void {
		const s = new Set(expanded);
		if (s.has(id)) s.delete(id);
		else s.add(id);
		expanded = s;
	}

	// Arrow keys move between tabs, as a tablist does.
	function onTabKey(e: KeyboardEvent): void {
		const step = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
		if (!step) return;
		e.preventDefault();
		const i = BELL_TABS.indexOf(tab);
		tab = BELL_TABS[(i + step + BELL_TABS.length) % BELL_TABS.length];
		tablist?.querySelector<HTMLElement>(`[data-tab="${tab}"]`)?.focus();
	}
</script>

<svelte:window onpointerdown={onWindowPointer} onkeydown={onWindowKey} />

{#snippet pic(n: AppNotification, count: number)}
	<span class="pic" class:alert={bellAlert(n.kind)} class:urgent={bellUrgent(n.kind)}>
		{#if n.image_url}
			<img src={n.image_url} alt="" loading="lazy" />
		{:else}
			<Icon name={bellIcon(n.kind)} size={16} />
		{/if}
		{#if count > 1}<span class="stack" aria-hidden="true">{count}</span>{/if}
	</span>
{/snippet}

{#snippet meta(n: AppNotification)}
	<span class="meta"
		>{#if n.account_name}<span class="who">{n.account_name}</span>{' · '}{/if}{relTime(n.created_at, now)}</span
	>
{/snippet}

{#snippet row(e: BellEntry)}
	{@const n = e.n}
	{@const to = bellTarget(n)}
	{@const unreadRow = bellEntryUnread(e, isUnread)}
	{@const kids = e.group}
	<li class:grp={!!kids}>
		<a
			class="row"
			class:unread={unreadRow}
			class:urgent={bellUrgent(n.kind)}
			class:hasmore={!!kids}
			href={to.href}
			target={to.external ? '_blank' : undefined}
			rel={to.external ? 'noopener noreferrer' : undefined}
			onclick={() => close(false)}
		>
			{@render pic(n, kids?.length ?? 0)}
			<span class="txt">
				{#if kids}
					<span class="line">{tn('bell.needs_link_group', kids.length)}</span>
					{#if n.campaign}<span class="meta camp">{n.campaign}</span>{/if}
				{:else}
					<span class="line">{bellLine(n)}</span>
				{/if}
				{@render meta(n)}
			</span>
			{#if unreadRow}<span class="sr">{t('bell.unread')}</span>{/if}
		</a>
		{#if kids}
			{@const shown = expanded.has(n.id)}
			<button
				type="button"
				class="more"
				aria-expanded={shown}
				aria-controls={`bellgrp-${n.id}`}
				aria-label={shown ? t('bell.hide_drops') : tn('bell.show_drops', kids.length)}
				title={shown ? t('bell.hide_drops') : tn('bell.show_drops', kids.length)}
				onclick={() => flip(n.id)}><Icon name={shown ? 'chevron-up' : 'chevron-down'} size={15} /></button
			>
			{#if shown}
				<ul class="kids" id={`bellgrp-${n.id}`}>
					{#each kids as k (k.id)}
						{@const kto = bellTarget(k)}
						<li>
							<a
								class="row kid"
								href={kto.href}
								target={kto.external ? '_blank' : undefined}
								rel={kto.external ? 'noopener noreferrer' : undefined}
								onclick={() => close(false)}
							>
								<span class="line">{k.title || bellLine(k)}</span>
								<span class="ago">{relTime(k.created_at, now)}</span>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		{/if}
	</li>
{/snippet}

<span class="bellwrap">
	<button
		type="button"
		class="bell"
		aria-label={label}
		aria-expanded={open}
		aria-controls="bellpop"
		bind:this={trigger}
		onclick={toggle}
	>
		<Icon name="bell" size={17} />
		{#if badge}<span class="badge" class:urgent={urgentUnread} aria-hidden="true">{badge}</span>{/if}
	</button>
	<div class="pop" id="bellpop" role="dialog" aria-label={t('bell.title')} hidden={!open} bind:this={pop}>
		<header>
			<h3>{t('bell.title')}<NewPill since="2.3" /></h3>
			<a class="gear" href="/settings#notifications" aria-label={t('bell.settings')} title={t('bell.settings')} onclick={() => close(false)}
				><Icon name="gear" size={15} /></a
			>
		</header>
		{#if live.bell && items.length > 0}
			<div class="tabs" role="tablist" tabindex="-1" aria-label={t('bell.tabs')} bind:this={tablist} onkeydown={onTabKey}>
				{#each BELL_TABS as x (x)}
					{@const k = bellTabCount(items, x, isUnread)}
					<button
						type="button"
						role="tab"
						data-tab={x}
						id={`belltab-${x}`}
						aria-selected={tab === x}
						aria-controls="bellbody"
						tabindex={tab === x ? 0 : -1}
						onclick={() => (tab = x)}
					>
						{TAB_LABEL[x]()}{#if k > 0}<span class="n" class:urgent={bellTabUrgent(items, x, isUnread)} aria-hidden="true"
								>{badgeText(k)}</span
							><span class="sr">{tn('bell.tab_unread', k)}</span>{/if}
					</button>
				{/each}
			</div>
		{/if}
		<div
			class="body"
			id="bellbody"
			role={live.bell && items.length > 0 ? 'tabpanel' : undefined}
			aria-labelledby={live.bell && items.length > 0 ? `belltab-${tab}` : undefined}
		>
			{#if failed}
				<p class="empty">{t('bell.load_failed')}</p>
			{:else if !live.bell}
				<p class="empty">{t('bell.loading')}</p>
			{:else if items.length === 0}
				<p class="empty">{t('bell.empty')}</p>
			{:else if groups.recent.length === 0 && groups.older.length === 0}
				<p class="empty">{t('bell.tab_empty')}</p>
			{:else}
				{#if groups.recent.length > 0}
					<h4>{t('bell.recent')}</h4>
					<ul>
						{#each groups.recent as e (e.n.id)}{@render row(e)}{/each}
					</ul>
				{/if}
				{#if groups.older.length > 0}
					<h4>{t('bell.older')}</h4>
					<ul>
						{#each groups.older as e (e.n.id)}{@render row(e)}{/each}
					</ul>
				{/if}
			{/if}
		</div>
	</div>
</span>

<style>
	.bellwrap {
		position: relative;
		display: inline-flex;
	}
	.bell {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		padding: 0;
		border: 0;
		border-radius: 8px;
		background: none;
		color: var(--t2);
		cursor: pointer;
	}
	.bell:hover,
	.bell[aria-expanded='true'] {
		color: var(--text);
		background: var(--surface);
	}
	.bell:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
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
	.pop {
		position: absolute;
		right: 0;
		top: calc(100% + 8px);
		width: 360px;
		max-width: calc(100vw - 32px);
		max-height: min(520px, calc(100vh - 80px));
		display: flex;
		flex-direction: column;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 10px;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
		font-size: 13px;
		z-index: 20;
		overflow: hidden;
	}
	.pop[hidden] {
		display: none;
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding: 10px 10px 8px 14px;
	}
	h3 {
		margin: 0;
		font-size: 13px;
		font-weight: 600;
	}
	.gear {
		display: inline-flex;
		padding: 6px;
		border-radius: 6px;
		color: var(--t2);
	}
	.gear:hover {
		color: var(--text);
		background: var(--surface-3);
	}
	.gear:focus-visible,
	.row:focus-visible,
	.more:focus-visible,
	.tabs button:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.body {
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 4px 6px 6px;
		border-top: 1px solid var(--border-soft);
	}
	h4 {
		margin: 8px 8px 4px;
		font-size: 11.5px;
		font-weight: 600;
		color: var(--t3);
	}
	ul {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 2px;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 8px;
		border-radius: 8px;
		color: var(--text);
		text-decoration: none;
	}
	.row:hover {
		background: var(--surface-3);
	}
	.row.unread {
		background: var(--accent-soft);
	}
	.row.unread:hover {
		background: color-mix(in srgb, var(--accent) 22%, transparent);
	}
	/* Reward codes: red, read or not, so a code waiting to be redeemed stands out. */
	.row.urgent {
		background: var(--red-soft);
	}
	.row.urgent:hover {
		background: color-mix(in srgb, var(--red) 24%, transparent);
	}
	.badge.urgent {
		background: var(--red);
	}
	.pic {
		position: relative;
		flex: none;
		width: 32px;
		height: 32px;
		border-radius: 6px;
		overflow: hidden;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: var(--art-bg);
		color: var(--t2);
	}
	.pic.alert {
		color: var(--amber);
		background: var(--amber-soft);
	}
	.pic.urgent {
		color: var(--red);
		background: var(--red-soft);
	}
	.pic img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.txt {
		min-width: 0;
		display: grid;
		gap: 2px;
	}
	.line {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.meta {
		font-size: 12px;
		color: var(--t3);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.who {
		color: var(--t2);
	}
	/* Segmented tabs under the title, three equal parts so they fit 360px and a phone. */
	.tabs {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 2px;
		margin: 0 10px 8px;
		padding: 2px;
		border-radius: 8px;
		background: var(--surface);
	}
	.tabs button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		min-width: 0;
		height: 28px;
		padding: 0 6px;
		border: 0;
		border-radius: 6px;
		background: none;
		color: var(--t2);
		font: inherit;
		font-size: 12.5px;
		font-weight: 500;
		white-space: nowrap;
		cursor: pointer;
	}
	.tabs button:hover {
		color: var(--text);
	}
	.tabs button[aria-selected='true'] {
		background: var(--surface-3);
		color: var(--text);
	}
	/* Unread count in a tab; red while an unread reward code waits in it. */
	.n {
		min-width: 16px;
		height: 16px;
		padding: 0 4px;
		box-sizing: border-box;
		border-radius: 999px;
		background: var(--accent-soft);
		color: var(--accent);
		font-size: 10.5px;
		font-weight: 650;
		line-height: 16px;
		text-align: center;
		font-variant-numeric: tabular-nums;
	}
	.n.urgent {
		background: var(--red-soft);
		color: var(--red);
	}
	/* A folded needs-link row: count on the image, chevron over the right edge. */
	.grp {
		position: relative;
	}
	.row.hasmore {
		padding-right: 44px;
	}
	.stack {
		position: absolute;
		right: 0;
		bottom: 0;
		min-width: 14px;
		padding: 0 3px;
		box-sizing: border-box;
		border-top-left-radius: 5px;
		background: rgba(0, 0, 0, 0.72);
		color: #fff;
		font-size: 9.5px;
		font-weight: 650;
		line-height: 13px;
		text-align: center;
		font-variant-numeric: tabular-nums;
	}
	.meta.camp {
		color: var(--t2);
	}
	.more {
		position: absolute;
		top: 9px;
		right: 6px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 30px;
		height: 30px;
		padding: 0;
		border: 0;
		border-radius: 6px;
		background: none;
		color: var(--t2);
		cursor: pointer;
	}
	.more:hover,
	.more[aria-expanded='true'] {
		color: var(--text);
		background: var(--surface-3);
	}
	.kids {
		margin: 2px 0 4px 50px;
		gap: 0;
	}
	.row.kid {
		justify-content: space-between;
		padding: 5px 8px;
		font-size: 12.5px;
	}
	.ago {
		flex: none;
		font-size: 12px;
		color: var(--t3);
	}
	.empty {
		margin: 0;
		padding: 24px 8px;
		text-align: center;
		color: var(--t3);
	}
	.sr {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
	@media (max-width: 720px) {
		/* Full width under the bar, 16px from each edge. */
		.bellwrap {
			position: static;
		}
		.pop {
			position: fixed;
			left: 16px;
			right: 16px;
			top: 56px;
			width: auto;
			max-width: none;
		}
	}
</style>
