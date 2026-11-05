<script lang="ts">
	import { page } from '$app/state';
	import logo from '../../../../assets/img/logo.png';
	import type { UpdateInfo } from '$lib/api/types';
	import { t } from '$lib/i18n/index.svelte';
	import VersionPopover from './VersionPopover.svelte';
	import NewPill from './NewPill.svelte';
	import NotificationBell from './NotificationBell.svelte';

	let { version, update }: { version: string; update: UpdateInfo | null } = $props();

	const items: { href: string; key: string; since?: string }[] = [
		{ href: '/', key: 'nav.home' },
		{ href: '/discover', key: 'nav.discover' },
		{ href: '/games', key: 'nav.games' },
		{ href: '/points', key: 'nav.points', since: '2.3' },
		{ href: '/inventory', key: 'nav.inventory' },
		{ href: '/settings', key: 'nav.settings' }
	];
	const on = (href: string) =>
		href === '/' ? page.url.pathname === '/' : page.url.pathname === href || page.url.pathname.startsWith(`${href}/`);

	// On narrow screens the links scroll sideways; keep the current one in view.
	let tabs: HTMLElement | undefined = $state();
	$effect(() => {
		void page.url.pathname;
		const nav = tabs;
		if (!nav) return;
		const reveal = () => {
			const a = nav.querySelector<HTMLElement>('a.on');
			if (!a) return;
			const t = nav.getBoundingClientRect();
			const r = a.getBoundingClientRect();
			if (r.right > t.right) nav.scrollLeft += r.right - t.right;
			else if (r.left < t.left) nav.scrollLeft -= t.left - r.left;
		};
		reveal();
		if (typeof ResizeObserver === 'undefined') return;
		const ro = new ResizeObserver(reveal);
		ro.observe(nav);
		return () => ro.disconnect();
	});
</script>

<header class="top">
	<div class="top-in">
		<a href="/" class="brand"><img src={logo} alt="" /><span>{t('app.name')}</span></a>
		<nav class="tabs" bind:this={tabs}>
			{#each items as it (it.href)}
				<a href={it.href} class:on={on(it.href)} aria-current={on(it.href) ? 'page' : undefined}
					>{t(it.key)}{#if it.since}<NewPill since={it.since} sup />{/if}</a
				>
			{/each}
		</nav>
		<div class="end">
			<VersionPopover current={version} {update} />
			<a
				class="gh"
				href="https://github.com/aalejandrofer/GrubDrops"
				target="_blank"
				rel="noreferrer"
				aria-label={t('topbar.github')}
				title={t('topbar.github')}
			>
				<svg width="17" height="17" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
					<path
						d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z"
					/>
				</svg>
			</a>
			<NotificationBell />
		</div>
	</div>
</header>

<style>
	.top {
		position: sticky;
		top: 0;
		z-index: 10;
		background: color-mix(in srgb, var(--bg) 92%, transparent);
		border-bottom: 1px solid var(--border-soft);
		backdrop-filter: blur(6px);
	}
	.top-in {
		max-width: 1160px;
		margin: 0 auto;
		height: var(--top-h);
		padding: 0 24px;
		display: flex;
		align-items: center;
		gap: 28px;
	}
	a {
		color: inherit;
		text-decoration: none;
	}
	.brand {
		display: flex;
		align-items: center;
		gap: 8px;
		font-weight: 600;
		letter-spacing: -0.01em;
	}
	.brand img {
		width: 24px;
		height: 24px;
		border-radius: 6px;
	}
	.tabs {
		display: flex;
		gap: 2px;
	}
	/* Version and bell sit together at the right edge. */
	.end {
		margin-left: auto;
		flex: none;
		display: flex;
		align-items: center;
		gap: 4px;
	}
	/* GitHub link: same 32px hit area and hover as the bell. */
	.gh {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 8px;
		color: var(--t2);
	}
	.gh:hover {
		color: var(--text);
		background: var(--surface);
	}
	.gh:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.tabs a {
		padding: 6px 10px;
		border-radius: 6px;
		color: var(--t2);
		font-weight: 500;
		font-size: 13.5px;
	}
	.tabs a:hover {
		color: var(--text);
	}
	/* Active link: purple underline under the word only; the inline-block New pill stays clear of it. */
	.tabs a.on {
		color: var(--text);
		text-decoration: underline 2px var(--accent);
		text-underline-offset: 7px;
	}
	.tabs a:focus-visible,
	.brand:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	@media (max-width: 720px) {
		.top-in {
			padding: 0 16px;
			gap: 14px;
		}
		.brand span {
			display: none;
		}
		/* Links scroll inside the bar when they don't fit, so the page never scrolls sideways. */
		.tabs {
			min-width: 0;
			overflow-x: auto;
			scrollbar-width: none;
		}
		.tabs::-webkit-scrollbar {
			display: none;
		}
		.tabs a {
			padding: 6px 8px;
			white-space: nowrap;
		}
		.tabs a:focus-visible {
			outline-offset: -2px;
		}
	}
	/* Phone: brand, version and bell on the first row; every link on a second
	   row of its own, spread across the width. It still scrolls if a longer
	   language does not fit. */
	@media (max-width: 560px) {
		.top-in {
			display: grid;
			grid-template-columns: auto minmax(0, 1fr);
			grid-template-rows: 48px 44px;
			column-gap: 8px;
		}
		.brand span {
			display: inline;
		}
		.end {
			grid-column: 2;
			justify-self: end;
		}
		.tabs {
			grid-row: 2;
			grid-column: 1 / -1;
			align-self: stretch;
			align-items: stretch;
			justify-content: space-between;
			gap: 0;
			margin: 0 -6px;
		}
		.tabs a {
			display: flex;
			align-items: center;
			padding: 0 6px;
			font-size: 13px;
			text-underline-offset: 6px;
		}
	}
</style>
