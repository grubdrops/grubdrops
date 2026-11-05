<script lang="ts">
	import { api } from '$lib/app';
	import type { AppNotification } from '$lib/api/types';
	import { relTime } from '$lib/format';
	import { t, tn } from '$lib/i18n/index.svelte';
	import { getLive } from '$lib/live-context';
	import { applyBellEvent, badgeText, bellAlert, bellGroups, bellIcon, bellLine, bellTarget } from '$lib/notifications';
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

	const unread = $derived(live.bell?.unread ?? 0);
	const badge = $derived(badgeText(unread));
	const groups = $derived(bellGroups(live.bell?.items ?? [], now));
	const label = $derived(unread > 0 ? tn('bell.open', unread) : t('bell.title'));

	// Mark the feed read on the server; the badge clears now, the SSE echo is a no-op.
	function markRead(): void {
		if (!live.bell || live.bell.unread === 0) return;
		live.bell = applyBellEvent(live.bell, { read: true });
		void api.readNotifications().catch(() => {});
	}

	async function show(): Promise<void> {
		open = true;
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
</script>

<svelte:window onpointerdown={onWindowPointer} onkeydown={onWindowKey} />

{#snippet row(n: AppNotification)}
	{@const to = bellTarget(n)}
	<li>
		<a
			class="row"
			class:unread={isUnread(n)}
			href={to.href}
			target={to.external ? '_blank' : undefined}
			rel={to.external ? 'noopener noreferrer' : undefined}
			onclick={() => close(false)}
		>
			<span class="pic" class:alert={bellAlert(n.kind)}>
				{#if n.image_url}
					<img src={n.image_url} alt="" loading="lazy" />
				{:else}
					<Icon name={bellIcon(n.kind)} size={16} />
				{/if}
			</span>
			<span class="txt">
				<span class="line">{bellLine(n)}</span>
				<span class="meta"
					>{#if n.account_name}<span class="who">{n.account_name}</span>{' · '}{/if}{relTime(n.created_at, now)}</span
				>
			</span>
			{#if isUnread(n)}<span class="sr">{t('bell.unread')}</span>{/if}
		</a>
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
		{#if badge}<span class="badge" aria-hidden="true">{badge}</span>{/if}
	</button>
	<div class="pop" id="bellpop" role="dialog" aria-label={t('bell.title')} hidden={!open} bind:this={pop}>
		<header>
			<h3>{t('bell.title')}<NewPill since="2.4" /></h3>
			<a class="gear" href="/settings#notifications" aria-label={t('bell.settings')} title={t('bell.settings')} onclick={() => close(false)}
				><Icon name="gear" size={15} /></a
			>
		</header>
		<div class="body">
			{#if failed}
				<p class="empty">{t('bell.load_failed')}</p>
			{:else if !live.bell}
				<p class="empty">{t('bell.loading')}</p>
			{:else if live.bell.items.length === 0}
				<p class="empty">{t('bell.empty')}</p>
			{:else}
				{#if groups.recent.length > 0}
					<h4>{t('bell.recent')}</h4>
					<ul>
						{#each groups.recent as n (n.id)}{@render row(n)}{/each}
					</ul>
				{/if}
				{#if groups.older.length > 0}
					<h4>{t('bell.older')}</h4>
					<ul>
						{#each groups.older as n (n.id)}{@render row(n)}{/each}
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
		border-bottom: 1px solid var(--border-soft);
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
	.row:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.body {
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 4px 6px 6px;
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
	.pic {
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
