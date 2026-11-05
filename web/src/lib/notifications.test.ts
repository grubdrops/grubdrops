import { beforeEach, describe, expect, it } from 'vitest';
import type { AppNotification, NotificationsResp } from '$lib/api/types';
import { setLang } from '$lib/i18n/index.svelte';
import { MAX_BELL, applyBellEvent, badgeText, bellAlert, bellGroups, bellIcon, bellLine, bellTarget } from './notifications';

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
		expect(bellIcon('claim')).toBe('gift');
		expect(bellIcon('canary')).toBe('pulse');
	});
});
