import type { Blocked, ClaimsQuery, Runner, Target } from '$lib/api/types';

export const PAGE_SIZES = [20, 50, 100] as const;
export const DEFAULT_PAGE_SIZE = 20;
export const PAGE_SIZE_KEY = 'grub.rewards.page_size';

export type ProgressRow = {
	target: Target;
	runner: Runner;
	pct: number;
	/** Minutes still to watch. */
	left: number;
	/** The drop the account is mining now. */
	current: boolean;
	/** Why a waiting drop cannot be mined yet; unset for current and queued drops. */
	reason?: Blocked['reason'];
};

function row(runner: Runner, target: Target, current: boolean, reason?: Blocked['reason']): ProgressRow {
	const req = target.required_minutes;
	const pct = req > 0 ? Math.min(100, Math.round((target.minutes / req) * 100)) : 0;
	return { target, runner, pct, left: Math.max(0, req - target.minutes), current, reason };
}

/**
 * Every started drop per account: the current drop, then queued and blocked
 * drops with minutes but not yet complete. One row per account and drop.
 * Current drops come first, then the rest furthest along first.
 */
export function progressRows(r: Runner[]): ProgressRow[] {
	const out: ProgressRow[] = [];
	const seen = new Set<string>();
	const add = (x: ProgressRow) => {
		const k = `${x.runner.account_id}:${x.target.benefit_id}`;
		if (seen.has(k)) return;
		seen.add(k);
		out.push(x);
	};
	const started = (t: Target) => t.minutes > 0 && t.required_minutes > 0 && t.minutes < t.required_minutes;
	for (const runner of r) {
		if (runner.state === 'needs_login') continue;
		if (runner.current) add(row(runner, runner.current, true));
		for (const t of runner.up_next ?? []) if (started(t)) add(row(runner, t, false));
		for (const b of runner.blocked ?? []) if (started(b.target)) add(row(runner, b.target, false, b.reason));
	}
	return out.sort((a, b) => Number(b.current) - Number(a.current) || b.pct - a.pct);
}

export type ProgressGroup = { runner: Runner; current: ProgressRow | null; rows: ProgressRow[] };

/**
 * progressRows split per account, accounts in the order of their first row.
 * rows starts with the current drop when there is one.
 */
export function progressGroups(list: ProgressRow[]): ProgressGroup[] {
	const out: ProgressGroup[] = [];
	const by = new Map<string, ProgressGroup>();
	for (const r of list) {
		let g = by.get(r.runner.account_id);
		if (!g) {
			g = { runner: r.runner, current: null, rows: [] };
			by.set(r.runner.account_id, g);
			out.push(g);
		}
		if (r.current) g.current = r;
		g.rows.push(r);
	}
	return out;
}

export function pageQuery(q: ClaimsQuery, page: number, size: number): ClaimsQuery {
	return { ...q, limit: size, offset: (Math.max(1, page) - 1) * size };
}

export function pageCount(total: number, size: number): number {
	return Math.max(1, Math.ceil(total / size));
}

/** 1-based item range shown on a page; 0 to 0 when there is nothing. */
export function pageRange(page: number, size: number, total: number): { from: number; to: number } {
	if (total <= 0) return { from: 0, to: 0 };
	const from = (page - 1) * size + 1;
	return { from, to: Math.min(total, page * size) };
}

export function validPageSize(n: number): number {
	return (PAGE_SIZES as readonly number[]).includes(n) ? n : DEFAULT_PAGE_SIZE;
}

export function readPageSize(): number {
	try {
		return validPageSize(Number(localStorage.getItem(PAGE_SIZE_KEY)));
	} catch {
		return DEFAULT_PAGE_SIZE;
	}
}

export function writePageSize(n: number): void {
	try {
		localStorage.setItem(PAGE_SIZE_KEY, String(n));
	} catch {
		/* storage unavailable: the choice lasts for this visit */
	}
}
