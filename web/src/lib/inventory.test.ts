import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Runner, Target } from '$lib/api/types';
import runnersMining from '$lib/fixtures/runners.mining.json';
import {
	inProgress,
	progressRows,
	pageCount,
	pageQuery,
	pageRange,
	readPageSize,
	validPageSize,
	writePageSize
} from './inventory';

const R = () => structuredClone(runnersMining) as Runner[];

describe('inProgress', () => {
	it('sorts by pct desc', () => {
		const p = inProgress(R());
		expect(p.map((x) => x.pct)).toEqual([70, 30]);
		expect(p.some((x) => x.runner.state === 'needs_login')).toBe(false);
	});
	it('left minutes', () => {
		expect(inProgress(R())[0].left).toBe(18);
	});
});

describe('progressRows', () => {
	const tgt = (id: string, minutes: number, req = 60): Target => ({
		game: 'Rust',
		platform: 'twitch',
		campaign_id: 'c1',
		campaign_name: 'Rust Drops',
		benefit_id: id,
		benefit_name: id,
		image_url: '',
		required_minutes: req,
		minutes,
		ends_at: null,
		channels: [],
		channel_count: 0
	});
	const runner = (id: string, over: Partial<Runner>): Runner => ({
		...R()[0],
		account_id: id,
		account_name: id,
		current: null,
		up_next: [],
		blocked: [],
		...over
	});
	const ids = (rows: ReturnType<typeof progressRows>) => rows.map((x) => `${x.runner.account_id}:${x.target.benefit_id}`);

	it('lists the current drop and started queued drops', () => {
		const rows = progressRows([runner('a', { current: tgt('cur', 6), up_next: [tgt('q1', 30), tgt('q2', 12)] })]);
		expect(ids(rows)).toEqual(['a:cur', 'a:q1', 'a:q2']);
		expect(rows.map((x) => x.current)).toEqual([true, false, false]);
	});
	it('puts current drops first, then by percent', () => {
		const rows = progressRows([
			runner('a', { current: tgt('a-cur', 3), up_next: [tgt('a-q', 50)] }),
			runner('b', { current: tgt('b-cur', 30), up_next: [tgt('b-q', 20)] })
		]);
		expect(ids(rows)).toEqual(['b:b-cur', 'a:a-cur', 'a:a-q', 'b:b-q']);
	});
	it('does not list the current drop twice', () => {
		const rows = progressRows([
			runner('a', {
				current: tgt('x', 20),
				up_next: [tgt('x', 20), tgt('y', 10)],
				blocked: [{ target: tgt('y', 10), reason: 'skipped' }]
			})
		]);
		expect(ids(rows)).toEqual(['a:x', 'a:y']);
		expect(rows[1].reason).toBeUndefined();
	});
	it('keeps the same drop on two accounts', () => {
		const rows = progressRows([runner('a', { up_next: [tgt('x', 20)] }), runner('b', { up_next: [tgt('x', 40)] })]);
		expect(ids(rows)).toEqual(['b:x', 'a:x']);
	});
	it('leaves out zero-minute and complete drops', () => {
		const rows = progressRows([runner('a', { up_next: [tgt('zero', 0), tgt('done', 60), tgt('over', 90), tgt('half', 30)] })]);
		expect(ids(rows)).toEqual(['a:half']);
	});
	it('includes blocked drops only with progress', () => {
		const rows = progressRows([
			runner('a', {
				blocked: [
					{ target: tgt('b0', 0), reason: 'not_linked' },
					{ target: tgt('b1', 15), reason: 'precondition' }
				]
			})
		]);
		expect(ids(rows)).toEqual(['a:b1']);
		expect(rows[0]).toMatchObject({ current: false, reason: 'precondition', pct: 25, left: 45 });
	});
	it('skips signed-out accounts', () => {
		expect(progressRows([runner('a', { state: 'needs_login', up_next: [tgt('x', 20)] })])).toEqual([]);
	});
	it('lists the fixture runners', () => {
		expect(progressRows(R()).map((x) => x.pct)).toEqual([70, 30]);
	});
});

describe('pageQuery', () => {
	it('offsets by page and size', () => {
		expect(pageQuery({ game: 'Rust', q: 'hat' }, 3, 50)).toEqual({ game: 'Rust', q: 'hat', limit: 50, offset: 100 });
		expect(pageQuery({}, 1, 20)).toEqual({ limit: 20, offset: 0 });
	});
});

describe('pageCount', () => {
	it('is at least 1', () => {
		expect(pageCount(0, 20)).toBe(1);
		expect(pageCount(20, 20)).toBe(1);
		expect(pageCount(21, 20)).toBe(2);
		expect(pageCount(805, 100)).toBe(9);
	});
});

describe('pageRange', () => {
	it('gives from and to', () => {
		expect(pageRange(1, 20, 0)).toEqual({ from: 0, to: 0 });
		expect(pageRange(1, 20, 7)).toEqual({ from: 1, to: 7 });
		expect(pageRange(2, 20, 45)).toEqual({ from: 21, to: 40 });
		expect(pageRange(3, 20, 45)).toEqual({ from: 41, to: 45 });
	});
});

describe('page size storage', () => {
	afterEach(() => vi.unstubAllGlobals());
	const stub = (v: string | null) =>
		vi.stubGlobal('localStorage', { getItem: () => v, setItem: vi.fn() });
	it('accepts 20, 50, 100', () => {
		for (const n of [20, 50, 100]) expect(validPageSize(n)).toBe(n);
	});
	it('falls back to 20', () => {
		for (const v of [null, '', 'abc', '10', '75']) {
			stub(v);
			expect(readPageSize()).toBe(20);
		}
		stub('50');
		expect(readPageSize()).toBe(50);
	});
	it('survives throwing storage', () => {
		vi.stubGlobal('localStorage', {
			getItem: () => {
				throw new Error('blocked');
			},
			setItem: () => {
				throw new Error('blocked');
			}
		});
		expect(readPageSize()).toBe(20);
		expect(() => writePageSize(50)).not.toThrow();
	});
});
