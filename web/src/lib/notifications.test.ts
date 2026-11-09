import { beforeEach, describe, expect, it } from 'vitest';
import type { AppNotification, NotificationKind, NotificationsResp } from '$lib/api/types';
import { setLang, tn } from '$lib/i18n/index.svelte';
import {
	BELL_TABS,
	MAX_BELL,
	applyBellEvent,
	badgeText,
	bellAlert,
	bellFold,
	bellGroups,
	bellIcon,
	bellLine,
	bellSections,
	bellTabCount,
	bellTabUrgent,
	bellTarget,
	bellUrgent,
	inBellTab,
	type BellTab
} from './notifications';

beforeEach(() => setLang('en'));

const now = Date.parse('2026-11-03T12:00:00Z');

function note(id: number, over: Partial<AppNotification> = {}): AppNotification {
	return {
		id,
		created_at: '2026-11-03T11:00:00Z',
		event: 'claim',
		kind: 'claim',
		account_id: 'a1',
		account_name: 'Alice',
		platform: 'twitch',
		title: 'Cape',
		body: '',
		game: 'Minecraft',
		campaign: '',
		image_url: '',
		link: '',
		read: false,
		...over
	};
}

describe('badgeText', () => {
	it('is empty at zero and caps at 99+', () => {
		expect(badgeText(0)).toBe('');
		expect(badgeText(-1)).toBe('');
		expect(badgeText(7)).toBe('7');
		expect(badgeText(99)).toBe('99');
		expect(badgeText(100)).toBe('99+');
	});
});

describe('applyBellEvent', () => {
	const base = (): NotificationsResp => ({ items: [note(2), note(1, { read: true })], unread: 1 });

	it('prepends a new row once and counts it', () => {
		const s = applyBellEvent(base(), { item: note(3) })!;
		expect(s.items.map((n) => n.id)).toEqual([3, 2, 1]);
		expect(s.unread).toBe(2);
		expect(applyBellEvent(s, { item: note(3) })).toBe(s);
	});

	it('read marks every row read and clears the count', () => {
		const s = applyBellEvent(base(), { read: true })!;
		expect(s.unread).toBe(0);
		expect(s.items.every((n) => n.read)).toBe(true);
	});

	it('keeps the list capped', () => {
		let s: NotificationsResp | null = { items: [], unread: 0 };
		for (let i = 1; i <= MAX_BELL + 5; i++) s = applyBellEvent(s, { item: note(i) });
		expect(s!.items).toHaveLength(MAX_BELL);
		expect(s!.items[0].id).toBe(MAX_BELL + 5);
	});

	it('does nothing before the first load', () => {
		expect(applyBellEvent(null, { item: note(1) })).toBeNull();
	});
});

describe('bellGroups', () => {
	it('splits at 24 hours, keeping order', () => {
		const g = bellGroups(
			[note(3), note(2, { created_at: '2026-11-02T13:00:00Z' }), note(1, { created_at: '2026-11-01T12:00:00Z' }), note(0, { created_at: null })],
			now
		);
		expect(g.recent.map((n) => n.id)).toEqual([3, 2]);
		expect(g.older.map((n) => n.id)).toEqual([1, 0]);
	});
});

describe('bellTarget', () => {
	it('opens an outside link page in a new tab', () => {
		expect(bellTarget(note(1, { kind: 'needs_link', link: 'https://example.com/link' }))).toEqual({
			href: 'https://example.com/link',
			external: true
		});
	});

	it('sends a needs-link row without a safe link to Inventory', () => {
		expect(bellTarget(note(1, { kind: 'needs_link' }))).toEqual({ href: '/inventory#needs-link', external: false });
		expect(bellTarget(note(1, { kind: 'needs_link', link: 'javascript:alert(1)' })).href).toBe('/inventory#needs-link');
	});

	it('opens the account for login rows', () => {
		expect(bellTarget(note(1, { kind: 'auth', account_id: 'acc 1' })).href).toBe('/settings?account=acc%201#accounts');
		expect(bellTarget(note(1, { kind: 'auth_ok', account_id: '' })).href).toBe('/settings#accounts');
	});

	it('sends canary and errors to Health, the rest to Inventory', () => {
		expect(bellTarget(note(1, { kind: 'canary' })).href).toBe('/settings#health');
		expect(bellTarget(note(1, { kind: 'error' })).href).toBe('/settings#health');
		expect(bellTarget(note(1, { kind: 'claim' })).href).toBe('/inventory');
		expect(bellTarget(note(1, { kind: 'reward_code' })).href).toBe('/inventory');
		expect(bellTarget(note(1, { kind: 'reward_reminder' })).href).toBe('/inventory');
	});
});

describe('bellLine', () => {
	it('reads one short line per kind', () => {
		expect(bellLine(note(1))).toBe('Claimed Cape');
		expect(bellLine(note(1, { title: '' }))).toBe('Drop claimed');
		expect(bellLine(note(1, { kind: 'needs_link' }))).toBe('Link your account to claim Cape');
		expect(bellLine(note(1, { kind: 'needs_link', title: '' }))).toBe('Link your account to claim');
		expect(bellLine(note(1, { kind: 'auth' }))).toBe('Login needed');
		expect(bellLine(note(1, { kind: 'auth_ok' }))).toBe('Logged in again');
		expect(bellLine(note(1, { kind: 'canary', platform: 'kick' }))).toBe('Kick watch check failed');
		expect(bellLine(note(1, { kind: 'canary', platform: '' }))).toBe('Watch check failed');
		expect(bellLine(note(1, { kind: 'reward_code' }))).toBe('New code: Cape');
		expect(bellLine(note(1, { kind: 'reward_reminder' }))).toBe('Code expires soon: Cape');
		expect(bellLine(note(1, { kind: 'daily_reward', title: 'Gold (rare)' }))).toBe('Kick daily reward: Gold (rare)');
		expect(bellLine(note(1, { kind: 'daily_reward', title: '' }))).toBe('Kick daily reward claimed');
		expect(bellLine(note(1, { kind: 'error', body: 'boom' }))).toBe('Error: boom');
		expect(bellLine(note(1, { kind: 'error' }))).toBe('Mining error');
	});

	it('tones problems and picks an icon', () => {
		expect(bellAlert('needs_link')).toBe(true);
		expect(bellAlert('claim')).toBe(false);
		expect(bellUrgent('reward_code')).toBe(true);
		expect(bellUrgent('reward_reminder')).toBe(true);
		expect(bellUrgent('claim')).toBe(false);
		expect(bellIcon('claim')).toBe('gift');
		expect(bellIcon('canary')).toBe('pulse');
	});
});

describe('bell tabs', () => {
	const kinds: [NotificationKind, BellTab][] = [
		['claim', 'claims'],
		['daily_reward', 'claims'],
		['reward_code', 'claims'],
		['reward_reminder', 'claims'],
		['needs_link', 'alerts'],
		['auth', 'alerts'],
		['auth_ok', 'alerts'],
		['error', 'alerts'],
		['canary', 'alerts']
	];

	it.each(kinds)('puts %s under %s and All', (kind, tab) => {
		const n = note(1, { kind });
		expect(inBellTab(n, 'all')).toBe(true);
		expect(inBellTab(n, tab)).toBe(true);
		expect(inBellTab(n, tab === 'claims' ? 'alerts' : 'claims')).toBe(false);
	});

	it('lists the tabs in order', () => {
		expect([...BELL_TABS]).toEqual(['all', 'claims', 'alerts']);
	});
});

describe('bellFold', () => {
	const link = (id: number, over: Partial<AppNotification> = {}) =>
		note(id, { kind: 'needs_link', title: `Drop ${id}`, campaign: 'Echoes', link: 'https://example.com/a', ...over });

	it('folds needs-link rows of one account and campaign into the newest', () => {
		const out = bellFold([link(5), note(4), link(3), link(2)]);
		expect(out.map((e) => e.n.id)).toEqual([5, 4]);
		expect(out[0].group?.map((n) => n.id)).toEqual([5, 3, 2]);
		expect(out[1].group).toBeUndefined();
	});

	it('keeps other accounts and campaigns apart', () => {
		const out = bellFold([link(4), link(3, { account_id: 'a2' }), link(2, { campaign: 'Other' }), link(1)]);
		expect(out.map((e) => e.n.id)).toEqual([4, 3, 2]);
		expect(out[0].group?.map((n) => n.id)).toEqual([4, 1]);
	});

	it('leaves a lone row unfolded and folds by link when there is no campaign', () => {
		expect(bellFold([link(1)])[0].group).toBeUndefined();
		const out = bellFold([link(3, { campaign: '' }), link(2, { campaign: '' }), link(1, { campaign: '', link: '' })]);
		expect(out.map((e) => e.n.id)).toEqual([3, 1]);
		expect(out[0].group).toHaveLength(2);
	});

	it('never folds other kinds', () => {
		expect(bellFold([note(2, { campaign: 'Echoes' }), note(1, { campaign: 'Echoes' })])).toHaveLength(2);
	});
});

describe('bellSections', () => {
	it('filters by tab, folds, then splits by the newest row', () => {
		const items = [
			note(4, { kind: 'needs_link', campaign: 'Echoes' }),
			note(3),
			note(2, { kind: 'needs_link', campaign: 'Echoes', created_at: '2026-11-01T12:00:00Z' }),
			note(1, { kind: 'auth', created_at: '2026-11-01T12:00:00Z' })
		];
		const a = bellSections(items, 'alerts', now);
		expect(a.recent.map((e) => e.n.id)).toEqual([4]);
		expect(a.recent[0].group).toHaveLength(2);
		expect(a.older.map((e) => e.n.id)).toEqual([1]);
		expect(bellSections(items, 'claims', now).recent.map((e) => e.n.id)).toEqual([3]);
	});
});

describe('bellTabCount and bellTabUrgent', () => {
	const flood = [
		...Array.from({ length: 10 }, (_, i) => note(20 - i, { kind: 'needs_link', campaign: 'Echoes' })),
		note(5),
		note(4, { kind: 'reward_code' }),
		note(3, { kind: 'auth', read: true })
	];

	it('counts a folded group once per tab', () => {
		expect(bellTabCount(flood, 'all')).toBe(3);
		expect(bellTabCount(flood, 'claims')).toBe(2);
		expect(bellTabCount(flood, 'alerts')).toBe(1);
	});

	it('counts a group unread while any row in it is', () => {
		const items = [note(2, { kind: 'needs_link', campaign: 'E', read: true }), note(1, { kind: 'needs_link', campaign: 'E' })];
		expect(bellTabCount(items, 'alerts')).toBe(1);
		expect(bellTabCount(items, 'alerts', () => false)).toBe(0);
	});

	it('uses the given unread test, so rows marked read on open still count', () => {
		const read = flood.map((n) => ({ ...n, read: true }));
		expect(bellTabCount(read, 'claims')).toBe(0);
		expect(bellTabCount(read, 'claims', (n) => n.id !== 3)).toBe(2);
	});

	it('turns Claims red while an unread reward code waits, never All', () => {
		expect(bellTabUrgent(flood, 'claims')).toBe(true);
		expect(bellTabUrgent(flood, 'alerts')).toBe(false);
		expect(bellTabUrgent(flood, 'all')).toBe(false);
		const seen = flood.map((n) => (n.kind === 'reward_code' ? { ...n, read: true } : n));
		expect(bellTabUrgent(seen, 'claims')).toBe(false);
		expect(bellTabUrgent([note(1, { kind: 'reward_reminder' })], 'claims')).toBe(true);
	});
});

describe('needs-link group copy', () => {
	it('reads the folded count', () => {
		expect(tn('bell.needs_link_group', 10)).toBe('Link your account to claim 10 drops');
		expect(tn('bell.show_drops', 10)).toBe('Show 10 drops');
	});
});
