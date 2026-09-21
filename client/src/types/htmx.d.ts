/**
 * Ambient type declarations for htmx 4 ("The Fetchening").
 *
 * htmx 4 has no official published @types; these are minimal, strict-mode-safe
 * declarations capturing the surfaces this project uses (morph swaps, SSE,
 * native fetch config, custom events). Extend as the implementer wires features.
 *
 * NOTE: htmx is served self-hosted from /assets (no CDN). `window.htmx` is the
 * global entry point.
 */

export interface HxConfig {
    credentials?: RequestCredentials;
    headers?: Record<string, string>;
    // htmx 4 native fetch options
    fetch?: RequestInit;
}

export interface HxEvent extends CustomEvent {
    detail: {
        value: unknown;
        elt: HTMLElement;
        // Idiomorph 3 swap context
        shouldSwap: boolean;
        target: Element | null;
    };
}

// Minimal global augmentation (no runtime import — pure ambient).
declare global {
    interface Window {
        htmx: {
            process: (root: HTMLElement | Document) => void;
            trigger: (
                elt: Element,
                event: string,
                configOrDetail?: Record<string, unknown>
            ) => void;
            on: (event: string, listener: (e: HxEvent) => void) => void;
            off: (event: string, listener: (e: HxEvent) => void) => void;
            config: Record<string, unknown>;
        };
        // htmx 4 SSE / WebSocket extensions mount hook
        htmxSSE: unknown;
        htmxWS: unknown;
    }
}

export {};
