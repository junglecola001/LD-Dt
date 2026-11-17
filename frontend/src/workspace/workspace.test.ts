import { describe, expect, it, beforeEach } from 'vitest';
import {
  arrangeFloatingWindows,
  clampBoundsToViewport,
  MIN_WINDOW_HEIGHT,
  MIN_WINDOW_WIDTH,
  resizeWindowBounds,
  snapWindowBounds,
} from './geometry';
import { useDraftBufferStore, useWorkspaceStore } from './store';
import { sanitizeCookedHTML } from '../utils/sanitize';

describe('Workspace Geometry & W01-W12 Rules', () => {
  beforeEach(() => {
    useWorkspaceStore.getState().applyPreset('browse');
  });

  it('W01: focuses overlapping windows and increments zIndex monotonically', () => {
    const store = useWorkspaceStore.getState();
    store.focusWindow('topic-1001');
    const z1 = useWorkspaceStore.getState().windows.find((w) => w.instanceId === 'topic-1001')!.zIndex;

    store.focusWindow('composer-welcome');
    const z2 = useWorkspaceStore.getState().windows.find((w) => w.instanceId === 'composer-welcome')!.zIndex;

    expect(z2).toBeGreaterThan(z1);
    expect(useWorkspaceStore.getState().activeInstanceId).toBe('composer-welcome');
  });

  it('W02: enforces minimum dimensions when resizing', () => {
    const resized = resizeWindowBounds(
      { x: 100, y: 100, width: 400, height: 300 },
      -350,
      -250,
      { e: true, s: true },
      { width: 1200, height: 800 },
    );
    expect(resized.width).toBe(MIN_WINDOW_WIDTH);
    expect(resized.height).toBe(MIN_WINDOW_HEIGHT);
  });

  it('W04: snaps within 8px of edges/siblings and skips snap when Alt is held', () => {
    const siblings = [{ x: 400, y: 50, width: 300, height: 300 }];
    const viewport = { width: 1200, height: 800 };

    const snapped = snapWindowBounds({ x: 394, y: 6, width: 320, height: 240 }, siblings, viewport, false);
    expect(snapped.bounds.x).toBe(400);
    expect(snapped.bounds.y).toBe(0);
    expect(snapped.guides.length).toBeGreaterThan(0);

    const unsnapped = snapWindowBounds({ x: 394, y: 6, width: 320, height: 240 }, siblings, viewport, true);
    expect(unsnapped.bounds.x).toBe(394);
    expect(unsnapped.bounds.y).toBe(6);
    expect(unsnapped.guides).toHaveLength(0);
  });

  it('W06 & W09: docking, minimizing, and resetting layout never deletes draft buffer', () => {
    useDraftBufferStore.getState().updateBody('draft-welcome', '重要草稿正文内容');
    const store = useWorkspaceStore.getState();

    store.setDisplayMode('composer-welcome', 'docked', 'right');
    store.toggleMinimize('composer-welcome');
    store.applyPreset('reading');

    expect(useDraftBufferStore.getState().buffers['draft-welcome']?.body).toBe('重要草稿正文内容');
  });

  it('W09 & W12: arranges floating windows and recovers offscreen bounds', () => {
    const viewport = { width: 1000, height: 600 };
    const store = useWorkspaceStore.getState();
    store.setWindowBounds('topic-1001', { x: 2500, y: 1800, width: 500, height: 400 });

    const clamped = clampBoundsToViewport({ x: 2500, y: 1800, width: 500, height: 400 }, viewport);
    expect(clamped.x).toBeLessThan(1000);
    expect(clamped.y).toBeLessThan(600);

    store.recoverOffscreenWindows();
    const recovered = useWorkspaceStore.getState().windows.find((w) => w.instanceId === 'topic-1001')!;
    expect(recovered.bounds.x).toBeGreaterThanOrEqual(8);
    expect(recovered.bounds.y).toBeGreaterThanOrEqual(8);

    const tiled = arrangeFloatingWindows(useWorkspaceStore.getState().windows, 'tileH', viewport);
    expect(Object.keys(tiled).length).toBeGreaterThanOrEqual(1);
  });
});

describe('HTML Sanitizer Security (plan.md 8.1)', () => {
  it('strips script tags, inline handlers, and javascript URLs while keeping safe prose', () => {
    const dirty =
      '<p>Hello</p><script>alert(1)</script><a href="javascript:alert(2)" onclick="hack()">bad</a><a href="https://linux.do/t/1">ok</a>';
    const clean = sanitizeCookedHTML(dirty);
    expect(clean).not.toContain('<script');
    expect(clean).not.toContain('javascript:');
    expect(clean).not.toContain('onclick');
    expect(clean).toContain('https://linux.do/t/1');
    expect(clean).toContain('noopener noreferrer');
  });
});
