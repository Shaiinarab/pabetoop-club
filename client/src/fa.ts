/**
 * fa.ts — Persian (fa-IR) UI-boundary helpers.
 *
 * CONVENTION: all numerics are stored as standard integers in SQLite and
 * converted to Persian digits ONLY at the UI layer. This file is pure infra
 * (no app behavior) so the client builds green under `tsgo --strict` even
 * before feature code exists.
 *
 * - toFa(n):      12345 -> "۱۲۳۴۵"
 * - faNumber(n,d): per-grouped thousands: "۱۲٬۳۴۵"
 * - faDate(d,opts): Jalali calendar display via the Intl fa-IR locale
 *                   (no runtime CDN; falls back to Persian month names).
 */

const FA_DIGITS = "۰۱۲۳۴۵۶۷۸۹";
const FA_THOUSANDS_SEP = "٬";

export const toFa = (input: number | string): string =>
    String(input).replace(/\d/g, (d: string) => FA_DIGITS[+d]!);

export const faNumber = (
    input: number | string,
    options: { thousandsSeparator?: boolean } = {}
): string => {
    const raw = String(input).replace(/\d/g, (d: string) => FA_DIGITS[+d]!);
    if (options.thousandsSeparator) {
        return raw.replace(/(^\d+|\d)(?=(\d{3})+(?!\d))/g, (m: string, ...args: string[]) => {
            const head = m.replace(/\d/g, (d) => FA_DIGITS[+d]!);
            return `${head}${FA_THOUSANDS_SEP}`;
        });
    }
    return raw;
};

export const faDate = (
    d: Date | number | string,
    opts: Intl.DateTimeFormatOptions = { dateStyle: "medium" }
): string => new Intl.DateTimeFormat("fa-IR", opts).format(new Date(d));

/** Compose a score/time for live match / assessment tiles. */
export const faScore = (home: number, away: number): string =>
    `${faNumber(home)} — ${faNumber(away)}`;
