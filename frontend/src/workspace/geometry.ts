import type { ResizeHandle, SnapGuide, ViewportSize, WindowBounds, WindowState } from './types';

export const MIN_WINDOW_WIDTH = 220;
export const MIN_WINDOW_HEIGHT = 220;
export const TITLEBAR_HEIGHT = 32;
export const SNAP_THRESHOLD = 8;

/**
 * Computes snapped position and visual guide lines against viewport edges and sibling windows (W04).
 * Holding Alt skips snapping entirely.
 */
export function snapWindowBounds(
  candidate: WindowBounds,
  siblings: WindowBounds[],
  viewport: ViewportSize,
  altPressed: boolean,
): { bounds: WindowBounds; guides: SnapGuide[] } {
  if (altPressed) {
    return { bounds: clampBoundsToViewport(candidate, viewport), guides: [] };
  }

  let nextX = candidate.x;
  let nextY = candidate.y;
  const guides: SnapGuide[] = [];

  const xTargets: number[] = [0, viewport.width];
  const yTargets: number[] = [0, viewport.height];

  for (const s of siblings) {
    xTargets.push(s.x, s.x + s.width);
    yTargets.push(s.y, s.y + s.height);
  }

  let bestDx = SNAP_THRESHOLD + 1;
  let snapGuideX: number | null = null;
  for (const tx of xTargets) {
    const leftDiff = Math.abs(candidate.x - tx);
    if (leftDiff <= SNAP_THRESHOLD && leftDiff < bestDx) {
      bestDx = leftDiff;
      nextX = tx;
      snapGuideX = tx;
    }
    const rightDiff = Math.abs(candidate.x + candidate.width - tx);
    if (rightDiff <= SNAP_THRESHOLD && rightDiff < bestDx) {
      bestDx = rightDiff;
      nextX = tx - candidate.width;
      snapGuideX = tx;
    }
  }

  let bestDy = SNAP_THRESHOLD + 1;
  let snapGuideY: number | null = null;
  for (const ty of yTargets) {
    const topDiff = Math.abs(candidate.y - ty);
    if (topDiff <= SNAP_THRESHOLD && topDiff < bestDy) {
      bestDy = topDiff;
      nextY = ty;
      snapGuideY = ty;
    }
    const bottomDiff = Math.abs(candidate.y + candidate.height - ty);
    if (bottomDiff <= SNAP_THRESHOLD && bottomDiff < bestDy) {
      bestDy = bottomDiff;
      nextY = ty - candidate.height;
      snapGuideY = ty;
    }
  }

  if (snapGuideX !== null) guides.push({ axis: 'x', pos: snapGuideX });
  if (snapGuideY !== null) guides.push({ axis: 'y', pos: snapGuideY });

  return {
    bounds: clampBoundsToViewport({ ...candidate, x: nextX, y: nextY }, viewport),
    guides,
  };
}

/**
 * Resizes bounds along any edge/corner while enforcing minimum window dimensions (W02).
 */
export function resizeWindowBounds(
  start: WindowBounds,
  dx: number,
  dy: number,
  handle: ResizeHandle,
  viewport: ViewportSize,
): WindowBounds {
  let { x, y, width, height } = start;

  if (handle.e) {
    width = Math.max(MIN_WINDOW_WIDTH, start.width + dx);
  }
  if (handle.s) {
    height = Math.max(MIN_WINDOW_HEIGHT, start.height + dy);
  }
  if (handle.w) {
    const nextW = Math.max(MIN_WINDOW_WIDTH, start.width - dx);
    x = start.x + (start.width - nextW);
    width = nextW;
  }
  if (handle.n) {
    const nextH = Math.max(MIN_WINDOW_HEIGHT, start.height - dy);
    y = start.y + (start.height - nextH);
    height = nextH;
  }

  return clampBoundsToViewport({ x, y, width, height }, viewport);
}

/**
 * Keeps at least the window titlebar and a 96px horizontal grip inside the viewport (W12).
 */
export function clampBoundsToViewport(bounds: WindowBounds, viewport: ViewportSize): WindowBounds {
  const maxW = Math.max(MIN_WINDOW_WIDTH, viewport.width);
  const maxH = Math.max(MIN_WINDOW_HEIGHT, viewport.height);
  const width = Math.min(Math.max(MIN_WINDOW_WIDTH, bounds.width), maxW);
  const height = Math.min(Math.max(MIN_WINDOW_HEIGHT, bounds.height), maxH);

  const minVisibleX = 96;
  const minX = -width + minVisibleX;
  const maxX = Math.max(0, viewport.width - minVisibleX);
  const minY = 0;
  const maxY = Math.max(0, viewport.height - TITLEBAR_HEIGHT);

  return {
    x: Math.round(Math.min(Math.max(bounds.x, minX), maxX)),
    y: Math.round(Math.min(Math.max(bounds.y, minY), maxY)),
    width: Math.round(width),
    height: Math.round(height),
  };
}

/**
 * Pulls every window fully inside the visible viewport ("Recover All Windows", W12).
 */
export function recoverBoundsInsideViewport(bounds: WindowBounds, viewport: ViewportSize): WindowBounds {
  const width = Math.min(Math.max(MIN_WINDOW_WIDTH, bounds.width), Math.max(MIN_WINDOW_WIDTH, viewport.width - 16));
  const height = Math.min(Math.max(MIN_WINDOW_HEIGHT, bounds.height), Math.max(MIN_WINDOW_HEIGHT, viewport.height - 16));
  const maxX = Math.max(8, viewport.width - width - 8);
  const maxY = Math.max(8, viewport.height - height - 8);
  return {
    x: Math.round(Math.min(Math.max(8, bounds.x), maxX)),
    y: Math.round(Math.min(Math.max(8, bounds.y), maxY)),
    width: Math.round(width),
    height: Math.round(height),
  };
}

/**
 * Computes cascade or tiled bounds for floating windows (W09).
 */
export function arrangeFloatingWindows(
  windows: WindowState[],
  mode: 'cascade' | 'tileH' | 'tileV',
  viewport: ViewportSize,
): Record<string, WindowBounds> {
  const visible = windows.filter((w) => w.displayMode === 'floating' && w.viewState !== 'minimized');
  const out: Record<string, WindowBounds> = {};
  if (visible.length === 0) return out;

  const pad = 12;
  const usableW = Math.max(MIN_WINDOW_WIDTH, viewport.width - pad * 2);
  const usableH = Math.max(MIN_WINDOW_HEIGHT, viewport.height - pad * 2);

  if (mode === 'cascade') {
    visible.forEach((w, idx) => {
      const offset = idx * 28;
      const width = Math.min(640, Math.max(MIN_WINDOW_WIDTH, usableW - offset));
      const height = Math.min(480, Math.max(MIN_WINDOW_HEIGHT, usableH - offset));
      out[w.instanceId] = clampBoundsToViewport(
        { x: pad + offset, y: pad + offset, width, height },
        viewport,
      );
    });
    return out;
  }

  if (mode === 'tileH') {
    const cellW = Math.max(MIN_WINDOW_WIDTH, Math.floor((usableW - pad * (visible.length - 1)) / visible.length));
    visible.forEach((w, idx) => {
      out[w.instanceId] = clampBoundsToViewport(
        { x: pad + idx * (cellW + pad), y: pad, width: cellW, height: usableH },
        viewport,
      );
    });
    return out;
  }

  const cellH = Math.max(MIN_WINDOW_HEIGHT, Math.floor((usableH - pad * (visible.length - 1)) / visible.length));
  visible.forEach((w, idx) => {
    out[w.instanceId] = clampBoundsToViewport(
      { x: pad, y: pad + idx * (cellH + pad), width: usableW, height: cellH },
      viewport,
    );
  });
  return out;
}
