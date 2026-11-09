<script lang="ts">
	import type { AppNotification } from '$lib/api/types';
	import Icon from '$lib/components/Icon.svelte';
	import NewPill from '$lib/components/NewPill.svelte';
	import { relTime } from '$lib/format';
	import { bellAlert, bellGroups, bellIcon, bellLine, bellUrgent } from '$lib/notifications';
	import { CHIP_OF, CHIPS, collapse, inTab, type Chip, type Entry, type Tab } from './_data';

	/**
	 * The open bell panel, markup and styles copied from NotificationBell.svelte,
	 * plus one filter style per variant. Rows never navigate in the mock.
	 */
	let {
		mode,
		items,
		now,
		uid
	}: { mode: 'tabs' | 'chips' | 'grouped'; items: AppNotification[]; now: number; uid: string } = $props();

	const TABS: { id: Tab; label: string }[] = [
		{ id: 'all', label: 'All' },
		{ id: 'claims', label: 'Claims' },
		{ id: 'alerts', label: 'Alerts' }
	];

	let tab = $state<Tab>('all');
	let picked = $state(new Set<Chip>());
	let opened = $state(new Set<number>());

	const shown = $derived(
		mode === 'chips'
			? items.filter((n) => picked.size === 0 || picked.has(CHIP_OF[n.kind]))
			: items.filter((n) => inTab(n, tab))
	);
	const groups = $derived(bellGroups(shown, now));
	const entries = (list: AppNotification[]): Entry[] => (mode === 'grouped' ? collapse(list) : list.map((n) => ({ n })));

	const unreadOf = (e: Entry): boolean => (e.group ?? [e.n]).some((n) => !n.read);
	/** v3 counts a folded group as one row; v1 and v2 count every row. */
	function tabCount(t: Tab): number {
		const list = items.filter((n) => inTab(n, t));
		const rows = mode === 'grouped' ? collapse(list) : list.map((n) => ({ n }));
		return rows.filter(unreadOf).length;
	}
	const tabUrgent = (t: Tab): boolean => items.some((n) => !n.read && bellUrgent(n.kind) && inTab(n, t));
	const chipCount = (c: Chip): number => items.filter((n) => !n.read && CHIP_OF[n.kind] === c).length;

	function pick(c: Chip): void {
		const s = new Set(picked);
		if (s.has(c)) s.delete(c);
		else s.add(c);
		picked = s;
	}
	function flip(id: number): void {
		const s = new Set(opened);
		if (s.has(id)) s.delete(id);
		else s.add(id);
		opened = s;
	}
	function onTabKey(e: KeyboardEvent): void {
		const i = TABS.findIndex((x) => x.id === tab);
		const step = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
		if (!step) return;
		e.preventDefault();
		tab = TABS[(i + step + TABS.length) % TABS.length].id;
		(e.currentTarget as HTMLElement).querySelector<HTMLElement>(`[data-tab="${tab}"]`)?.focus();
	}
	const stay = (e: MouseEvent) => e.preventDefault();
</script>

{#snippet pic(n: AppNotification, count = 0)}
	<span class="pic" class:alert={bellAlert(n.kind)} class:urgent={bellUrgent(n.kind)}>
		{#if n.image_url}
			<img src={n.image_url} alt="" loading="lazy" />
		{:else}
			<Icon name={bellIcon(n.kind)} size={16} />
		{/if}
		{#if count > 1}<span class="stack" aria-hidden="true">{count}</span>{/if}
	</span>
{/snippet}

{#snippet row(e: Entry)}
	{@const n = e.n}
	{#if e.group}
		{@const open = opened.has(n.id)}
		<li class="grp">
			<a class="row hasmore" class:unread={unreadOf(e)} href={n.link} onclick={stay}>
				{@render pic(n, e.group.length)}
				<span class="txt">
					<span class="line">Link your account to claim {e.group.length} drops</span>
					<span class="meta camp">{n.campaign}</span>
					<span class="meta"><span class="who">{n.account_name}</span>{' · '}{relTime(n.created_at, now)}</span>
				</span>
				{#if unreadOf(e)}<span class="sr">Unread</span>{/if}
			</a>
			<button
				type="button"
				class="more"
				aria-expanded={open}
				aria-controls={`${uid}-g${n.id}`}
				aria-label={open ? 'Hide drops' : `Show ${e.group.length} drops`}
				onclick={() => flip(n.id)}><Icon name={open ? 'chevron-up' : 'chevron-down'} size={15} /></button
			>
			{#if open}
				<ul class="kids" id={`${uid}-g${n.id}`}>
					{#each e.group as k (k.id)}
						<li>
							<a class="row kid" class:unread={!k.read} href={k.link} onclick={stay}>
								<span class="txt">
									<span class="line">{k.title}</span>
								</span>
								<span class="ago">{relTime(k.created_at, now)}</span>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</li>
	{:else}
		<li>
			<a class="row" class:unread={!n.read} class:urgent={bellUrgent(n.kind)} href="/inventory" onclick={stay}>
				{@render pic(n)}
				<span class="txt">
					<span class="line">{bellLine(n)}</span>
					<span class="meta"
						>{#if n.account_name}<span class="who">{n.account_name}</span>{' · '}{/if}{relTime(n.created_at, now)}</span
					>
				</span>
				{#if !n.read}<span class="sr">Unread</span>{/if}
			</a>
		</li>
	{/if}
{/snippet}

<div class="pop" role="dialog" aria-label="Notifications">
	<header>
		<h3>Notifications<NewPill since="2.3" /></h3>
		<a class="gear" href="/settings#notifications" aria-label="Notification settings" title="Notification settings" onclick={stay}
			><Icon name="gear" size={15} /></a
		>
	</header>

	{#if mode === 'chips'}
		<div class="chips" role="group" aria-label="Show">
			{#each CHIPS as c (c.id)}
				{@const k = chipCount(c.id)}
				<button
					type="button"
					class="chip"
					class:urgent={c.id === 'codes' && k > 0}
					aria-pressed={picked.has(c.id)}
					onclick={() => pick(c.id)}
				>
					{c.label}{#if k > 0}<span class="n">{k}</span>{/if}
				</button>
			{/each}
		</div>
	{:else}
		<div class="tabs" role="tablist" aria-label="Show" tabindex="-1" onkeydown={onTabKey}>
			{#each TABS as x (x.id)}
				{@const k = tabCount(x.id)}
				<button
					type="button"
					role="tab"
					data-tab={x.id}
					id={`${uid}-t-${x.id}`}
					aria-selected={tab === x.id}
					aria-controls={`${uid}-body`}
					tabindex={tab === x.id ? 0 : -1}
					onclick={() => (tab = x.id)}
				>
					{x.label}{#if k > 0}<span class="n" class:urgent={x.id !== 'all' && tabUrgent(x.id)}>{k}</span>{/if}
				</button>
			{/each}
		</div>
	{/if}

	<div
		class="body"
		id={`${uid}-body`}
		role={mode === 'chips' ? undefined : 'tabpanel'}
		aria-labelledby={mode === 'chips' ? undefined : `${uid}-t-${tab}`}
	>
		{#if shown.length === 0}
			<p class="empty">Nothing here</p>
		{:else}
			{#if groups.recent.length > 0}
				<h4>Last 24 hours</h4>
				<ul>
					{#each entries(groups.recent) as e (e.n.id)}{@render row(e)}{/each}
				</ul>
			{/if}
			{#if groups.older.length > 0}
				<h4>Older</h4>
				<ul>
					{#each entries(groups.older) as e (e.n.id)}{@render row(e)}{/each}
				</ul>
			{/if}
		{/if}
	</div>
</div>

<style>
	.pop {
		width: 360px;
		max-width: 100%;
		max-height: 520px;
		display: flex;
		flex-direction: column;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 10px;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
		font-size: 13px;
		overflow: hidden;
		text-align: left;
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
	.tabs button:focus-visible,
	.chip:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}

	/* v1, v3: segmented tabs under the title. */
	.tabs {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
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
		height: 28px;
		padding: 0 8px;
		border: 0;
		border-radius: 6px;
		background: none;
		color: var(--t2);
		font: inherit;
		font-size: 12.5px;
		font-weight: 500;
		cursor: pointer;
	}
	.tabs button:hover {
		color: var(--text);
	}
	.tabs button[aria-selected='true'] {
		background: var(--surface-3);
		color: var(--text);
	}

	/* v2: multi-select chips. None picked shows everything. */
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		padding: 0 10px 8px;
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		height: 26px;
		padding: 0 8px;
		border: 1px solid var(--border);
		border-radius: 999px;
		background: none;
		color: var(--t2);
		font: inherit;
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
	}
	.chip:hover {
		color: var(--text);
		border-color: var(--t3);
	}
	.chip[aria-pressed='true'] {
		border-color: transparent;
		background: var(--accent-soft);
		color: var(--accent);
	}

	/* Unread count inside a tab or chip; red while a code waits, like the badge. */
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
	/* Chips carry the count as plain text so all five fit one row at 390px. */
	.chip .n {
		min-width: 0;
		padding: 0;
		background: none;
	}
	.chip.urgent .n {
		color: var(--red);
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
	.row.urgent {
		background: var(--red-soft);
	}
	.row.urgent:hover {
		background: color-mix(in srgb, var(--red) 24%, transparent);
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
	.meta.camp {
		color: var(--t2);
	}

	/* v3: a folded group. The chevron sits over the row's right edge. */
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
	.row.kid.unread {
		background: none;
	}
	.row.kid.unread:hover {
		background: var(--surface-3);
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
</style>
