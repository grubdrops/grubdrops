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

function isRecent(n: AppNotification, now: number): boolean {
	const at = n.created_at ? Date.parse(n.created_at) : NaN;
	return Number.isFinite(at) && now - at < DAY;
}

/** Rows from the last 24 hours, then the older ones, each newest first as given. */
export function bellGroups(items: AppNotification[], now: number): { recent: AppNotification[]; older: AppNotification[] } {
	const recent: AppNotification[] = [];
	const older: AppNotification[] = [];
	for (const n of items) (isRecent(n, now) ? recent : older).push(n);
	return { recent, older };
}

/** The bell's tabs, in order. */
export const BELL_TABS = ['all', 'claims', 'alerts'] as const;
export type BellTab = (typeof BELL_TABS)[number];

/**
 * Claims holds what the miner won, reward codes included; Alerts holds what needs fixing.
 * A code is a reward, so it sits with claims and keeps its red row there.
 */
const TAB_OF = {
	claim: 'claims',
	daily_reward: 'claims',
	reward_code: 'claims',
	reward_reminder: 'claims',
	needs_link: 'alerts',
	auth: 'alerts',
	auth_ok: 'alerts',
	error: 'alerts',
	canary: 'alerts'
} as const satisfies Record<NotificationKind, Exclude<BellTab, 'all'>>;

/** Whether a row shows under tab. A kind the UI doesn't know shows under Alerts. */
export function inBellTab(n: AppNotification, tab: BellTab): boolean {
	return tab === 'all' || (TAB_OF[n.kind] ?? 'alerts') === tab;
}

/** One bell row: a single notification, or needs-link rows folded together, newest first. */
export interface BellEntry {
	/** The newest row; the entry sits in its place and leads where it leads. */
	n: AppNotification;
	/** Every folded row, n first. Set only when two or more were folded. */
	group?: AppNotification[];
}

/** The fold key of a needs-link row: same account and campaign, or same link page without a campaign. */
function foldKey(n: AppNotification): string {
	if (n.kind !== 'needs_link') return '';
	const what = n.campaign ? `c:${n.campaign}` : n.link ? `l:${n.link}` : '';
	return what ? `${n.account_id}\u0000${what}` : '';
}

/** Folds needs-link rows for the same account and campaign into one entry, in the place of the newest. */
export function bellFold(items: AppNotification[]): BellEntry[] {
	const out: BellEntry[] = [];
	const seen = new Map<string, BellEntry>();
	for (const n of items) {
		const key = foldKey(n);
		const e = key ? seen.get(key) : undefined;
		if (e) {
			(e.group ??= [e.n]).push(n);
			continue;
		}
		const entry: BellEntry = { n };
		if (key) seen.set(key, entry);
		out.push(entry);
	}
	return out;
}

/** The rows of one tab, folded, split into the last 24 hours and older by each entry's newest row. */
export function bellSections(items: AppNotification[], tab: BellTab, now: number): { recent: BellEntry[]; older: BellEntry[] } {
	const recent: BellEntry[] = [];
	const older: BellEntry[] = [];
	for (const e of bellFold(items.filter((n) => inBellTab(n, tab)))) (isRecent(e.n, now) ? recent : older).push(e);
	return { recent, older };
}

type Unread = (n: AppNotification) => boolean;
const notRead: Unread = (n) => !n.read;

/** Whether an entry holds an unread row. */
export function bellEntryUnread(e: BellEntry, unread: Unread = notRead): boolean {
	return (e.group ?? [e.n]).some(unread);
}

/** Unread entries in tab: a folded group counts once. The bell badge still counts every row. */
export function bellTabCount(items: AppNotification[], tab: BellTab, unread: Unread = notRead): number {
	return bellFold(items.filter((n) => inBellTab(n, tab))).filter((e) => bellEntryUnread(e, unread)).length;
}

/** A tab's count turns red while an unread reward code waits in it. All stays plain. */
export function bellTabUrgent(items: AppNotification[], tab: BellTab, unread: Unread = notRead): boolean {
	return tab !== 'all' && items.some((n) => bellUrgent(n.kind) && inBellTab(n, tab) && unread(n));
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

/** Reward code rows are urgent: a code must be redeemed before it expires. */
export function bellUrgent(kind: NotificationKind): boolean {
	return kind === 'reward_code' || kind === 'reward_reminder';
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
