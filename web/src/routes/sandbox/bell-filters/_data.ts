import type { AppNotification, NotificationKind } from '$lib/api/types';

const MIN = 60_000;

/** Flat stand-ins for drop art, so the mock needs no network. */
const art = (bg: string, fg: string, shape: string): string =>
	'data:image/svg+xml,' +
	encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" fill="${bg}"/>${shape.replaceAll('FG', fg)}</svg>`);

const PUMPKIN = art(
	'#2b1a0e',
	'#f28c28',
	'<rect x="15" y="6" width="2.5" height="5" rx="1" fill="#5a7d2a"/><ellipse cx="11" cy="19" rx="6" ry="8" fill="FG"/><ellipse cx="21" cy="19" rx="6" ry="8" fill="FG"/><ellipse cx="16" cy="19" rx="5" ry="8.5" fill="#ff9d3d"/>'
);
const EMOTE = art('#1d2b3a', '#ffd166', '<circle cx="16" cy="17" r="9" fill="FG"/><rect x="9" y="12" width="14" height="5" rx="2.5" fill="#e0a96d"/>');
const DAILY = art('#14261c', '#53fc18', '<rect x="8" y="13" width="16" height="11" rx="1.5" fill="FG"/><rect x="7" y="10" width="18" height="4" rx="1" fill="#3fbf12"/>');

type Seed = Partial<AppNotification> & { kind: NotificationKind; ago: number };

const KIRA = { account_id: 'acc_kira', account_name: 'kira_mines', platform: 'kick' };
const NOVA = { account_id: 'acc_nova', account_name: 'nova.tv', platform: 'twitch' };

/** The staging flood: ten needs-link rows for one campaign, newest first, burying one claim. */
const seeds: Seed[] = [
	...Array.from({ length: 10 }, (_, i): Seed => ({
		kind: 'needs_link',
		ago: 2 + i,
		title: `Drop ${10 - i}`,
		campaign: 'Echoes of Halloween',
		game: 'Rust',
		image_url: PUMPKIN,
		link: 'https://example.com/link-kick',
		...KIRA
	})),
	{ kind: 'claim', ago: 13, title: 'Face, Meet Palm Emote', game: 'Marvel Rivals', image_url: EMOTE, ...NOVA },
	{ kind: 'reward_code', ago: 40, title: 'Twitch Prime Loot Crate', body: 'Expires in 6 days', ...NOVA },
	{ kind: 'auth', ago: 95, body: 'Session expired', ...KIRA },
	{ kind: 'error', ago: 180, body: 'Kick sidecar stopped', ...KIRA, read: true },
	{ kind: 'daily_reward', ago: 300, title: '50 Kick points', image_url: DAILY, ...KIRA, read: true },
	{ kind: 'canary', ago: 26 * 60, platform: 'twitch', account_id: '', account_name: '', read: true },
	{ kind: 'auth_ok', ago: 28 * 60, ...NOVA, read: true },
	{ kind: 'reward_reminder', ago: 30 * 60, title: 'Steam key: Gunfire Reborn DLC', ...NOVA, read: true },
	{ kind: 'claim', ago: 31 * 60, title: 'Skull Banner', game: 'Rust', ...NOVA, read: true }
];

/** The fake feed, timed against loadedAt so "5 min ago" reads right. */
export function feed(loadedAt: number): AppNotification[] {
	return seeds.map(({ ago, ...s }, i) => ({
		id: 100 - i,
		created_at: new Date(loadedAt - ago * MIN).toISOString(),
		event: s.kind,
		account_id: '',
		account_name: '',
		platform: '',
		title: '',
		body: '',
		game: '',
		campaign: '',
		image_url: '',
		link: '',
		read: false,
		...s
	}));
}

// ---- v1 / v3: three tabs ----

export type Tab = 'all' | 'claims' | 'alerts';

/**
 * Codes sit under Claims: a code is a reward the miner won, not a fault. Their rows stay red
 * there, and the Claims count turns red while one waits, like the bell badge does today.
 */
const TAB_OF: Record<NotificationKind, Exclude<Tab, 'all'>> = {
	claim: 'claims',
	daily_reward: 'claims',
	reward_code: 'claims',
	reward_reminder: 'claims',
	needs_link: 'alerts',
	auth: 'alerts',
	auth_ok: 'alerts',
	error: 'alerts',
	canary: 'alerts'
};

export const inTab = (n: AppNotification, tab: Tab): boolean => tab === 'all' || TAB_OF[n.kind] === tab;

// ---- v2: multi-select chips ----

export type Chip = 'claims' | 'links' | 'login' | 'errors' | 'codes';

export const CHIPS: { id: Chip; label: string }[] = [
	{ id: 'claims', label: 'Claims' },
	{ id: 'links', label: 'Links' },
	{ id: 'login', label: 'Login' },
	{ id: 'errors', label: 'Errors' },
	{ id: 'codes', label: 'Codes' }
];

export const CHIP_OF: Record<NotificationKind, Chip> = {
	claim: 'claims',
	daily_reward: 'claims',
	needs_link: 'links',
	auth: 'login',
	auth_ok: 'login',
	error: 'errors',
	canary: 'errors',
	reward_code: 'codes',
	reward_reminder: 'codes'
};

// ---- v3: collapse repeated needs-link rows ----

/** One row, or several needs-link rows for the same account and campaign folded into one. */
export type Entry = { n: AppNotification; group?: AppNotification[] };

/** Folds needs-link rows that share account and campaign into the slot of the newest one. */
export function collapse(items: AppNotification[]): Entry[] {
	const out: Entry[] = [];
	const at = new Map<string, Entry>();
	for (const n of items) {
		if (n.kind !== 'needs_link' || !n.campaign) {
			out.push({ n });
			continue;
		}
		const key = `${n.account_id}\u0000${n.campaign}`;
		const e = at.get(key);
		if (e) (e.group ??= [e.n]).push(n);
		else {
			const fresh: Entry = { n };
			at.set(key, fresh);
			out.push(fresh);
		}
	}
	return out;
}
