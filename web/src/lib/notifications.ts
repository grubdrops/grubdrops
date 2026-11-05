import type { AppNotification, NotificationKind, NotificationsEvent, NotificationsResp } from '$lib/api/types';
import { httpsOnly } from '$lib/format';
import { t } from '$lib/i18n/index.svelte';

/** Rows the bell keeps in memory; the server sends this many at most by default. */
export const MAX_BELL = 100;

const DAY = 24 * 3600_000;

/** Badge text for n unread: empty at 0, capped at 99+. */
export function badgeText(n: number): string {
	if (n <= 0) return '';
	return n > 99 ? '99+' : String(n);
}

/** The feed after one SSE event: a new row goes on top once, read marks every row read. */
export function applyBellEvent(s: NotificationsResp | null, ev: NotificationsEvent): NotificationsResp | null {
	if (!s) return s;
	if (ev.read) return { items: s.items.map((n) => (n.read ? n : { ...n, read: true })), unread: 0 };
	const it = ev.item;
	if (!it || s.items.some((n) => n.id === it.id)) return s;
	return { items: [it, ...s.items].slice(0, MAX_BELL), unread: s.unread + (it.read ? 0 : 1) };
}

/** Rows from the last 24 hours, then the older ones, each newest first as given. */
export function bellGroups(items: AppNotification[], now: number): { recent: AppNotification[]; older: AppNotification[] } {
	const recent: AppNotification[] = [];
	const older: AppNotification[] = [];
	for (const n of items) {
		const at = n.created_at ? Date.parse(n.created_at) : NaN;
		(Number.isFinite(at) && now - at < DAY ? recent : older).push(n);
	}
	return { recent, older };
}

/** Where a row leads: the page that fixes or shows it. External opens in a new tab. */
export function bellTarget(n: AppNotification): { href: string; external: boolean } {
	const account = n.account_id ? `/settings?account=${encodeURIComponent(n.account_id)}#accounts` : '/settings#accounts';
	switch (n.kind) {
		case 'needs_link': {
			const link = httpsOnly(n.link);
			return link ? { href: link, external: true } : { href: '/inventory#needs-link', external: false };
		}
		case 'auth':
		case 'auth_ok':
			return { href: account, external: false };
		case 'canary':
		case 'error':
			return { href: '/settings#health', external: false };
		default:
			return { href: '/inventory', external: false };
	}
}

const ICONS = {
	claim: 'gift',
	needs_link: 'link',
	error: 'alert',
	auth: 'lock',
	auth_ok: 'check-circle',
	canary: 'pulse',
	reward_code: 'key',
	reward_reminder: 'key',
	daily_reward: 'gift'
} as const satisfies Record<NotificationKind, string>;

/** The icon shown when a row has no image. */
export function bellIcon(kind: NotificationKind): (typeof ICONS)[NotificationKind] {
	return ICONS[kind] ?? 'bell';
}

/** Problem rows get the warning tone on their icon. */
export function bellAlert(kind: NotificationKind): boolean {
	return kind === 'needs_link' || kind === 'error' || kind === 'auth' || kind === 'canary';
}

const PLATFORMS: Record<string, string> = { twitch: 'Twitch', kick: 'Kick' };

/** The one line a row reads. */
export function bellLine(n: AppNotification): string {
	const title = n.title;
	switch (n.kind) {
		case 'claim':
			return title ? t('bell.claim', { drop: title }) : t('bell.claim_any');
		case 'needs_link':
			return title ? t('bell.needs_link', { drop: title }) : t('bell.needs_link_any');
		case 'auth':
			return t('bell.auth');
		case 'auth_ok':
			return t('bell.auth_ok');
		case 'canary':
			return n.platform ? t('bell.canary', { platform: PLATFORMS[n.platform] ?? n.platform }) : t('bell.canary_any');
		case 'reward_code':
			return t('bell.reward_code', { reward: title });
		case 'reward_reminder':
			return t('bell.reward_reminder', { reward: title });
		case 'daily_reward':
			return title ? t('bell.daily_reward', { reward: title }) : t('bell.daily_reward_any');
		default:
			return n.body ? t('bell.error', { msg: n.body }) : t('bell.error_any');
	}
}
