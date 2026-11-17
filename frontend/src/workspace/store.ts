import { create } from 'zustand';
import type { SubmitState } from '../bridge/types';
import {
  arrangeFloatingWindows,
  clampBoundsToViewport,
  recoverBoundsInsideViewport,
} from './geometry';
import { buildPresetWindows } from './presets';
import type {
  DisplayMode,
  DockRegion,
  PanelType,
  ViewportSize,
  WindowBounds,
  WindowState,
  WorkspacePresetId,
} from './types';

export interface DraftBuffer {
  draftId: string;
  mode: 'newTopic' | 'replyTopic' | 'replyPost' | 'editPost';
  topicId?: number;
  title: string;
  body: string;
  revision: number;
  savedRevision: number;
  localState: 'editing' | 'saving' | 'saved' | 'saveFailed';
  submitState: SubmitState;
  errorMessage?: string;
}

interface DraftBufferStore {
  buffers: Record<string, DraftBuffer>;
  ensureBuffer: (draftId: string, init?: Partial<DraftBuffer>) => DraftBuffer;
  updateBody: (draftId: string, body: string) => void;
  updateTitle: (draftId: string, title: string) => void;
  markSaved: (draftId: string, revision: number) => void;
  markSaveFailed: (draftId: string, message: string) => void;
  setSubmitState: (draftId: string, state: DraftBuffer['submitState'], errorMessage?: string) => void;
}

/**
 * Keeps composer text independent from WindowFrame mount lifecycle (W06, plan.md 5.8).
 * Floating <-> docked <-> minimized transitions never drop or reset draft edits.
 */
export const useDraftBufferStore = create<DraftBufferStore>((set, get) => ({
  buffers: {
    'draft-welcome': {
      draftId: 'draft-welcome',
      mode: 'replyTopic',
      topicId: 1001,
      title: '回复：欢迎来到 LD Studio',
      body: '这是一份演示草稿：拖动、停靠或最小化窗口时，这里的文本与本地 revision 会保留。',
      revision: 1,
      savedRevision: 1,
      localState: 'saved',
      submitState: 'idle',
    },
  },
  ensureBuffer: (draftId, init) => {
    const existing = get().buffers[draftId];
    if (existing) return existing;
    const created: DraftBuffer = {
      draftId,
      mode: init?.mode ?? 'newTopic',
      topicId: init?.topicId,
      title: init?.title ?? '',
      body: init?.body ?? '',
      revision: init?.revision ?? 1,
      savedRevision: init?.savedRevision ?? 1,
      localState: 'saved',
      submitState: 'idle',
    };
    set((s) => ({ buffers: { ...s.buffers, [draftId]: created } }));
    return created;
  },
  updateBody: (draftId, body) =>
    set((s) => {
      const cur = s.buffers[draftId];
      if (!cur) return s;
      return {
        buffers: {
          ...s.buffers,
          [draftId]: { ...cur, body, revision: cur.revision + 1, localState: 'editing' },
        },
      };
    }),
  updateTitle: (draftId, title) =>
    set((s) => {
      const cur = s.buffers[draftId];
      if (!cur) return s;
      return {
        buffers: {
          ...s.buffers,
          [draftId]: { ...cur, title, revision: cur.revision + 1, localState: 'editing' },
        },
      };
    }),
  markSaved: (draftId, revision) =>
    set((s) => {
      const cur = s.buffers[draftId];
      if (!cur) return s;
      const allSaved = revision >= cur.revision;
      return {
        buffers: {
          ...s.buffers,
          [draftId]: {
            ...cur,
            savedRevision: Math.max(cur.savedRevision, revision),
            localState: allSaved ? 'saved' : cur.localState,
          },
        },
      };
    }),
  markSaveFailed: (draftId, errorMessage) =>
    set((s) => {
      const cur = s.buffers[draftId];
      if (!cur) return s;
      return {
        buffers: {
          ...s.buffers,
          [draftId]: { ...cur, localState: 'saveFailed', errorMessage },
        },
      };
    }),
  setSubmitState: (draftId, submitState, errorMessage) =>
    set((s) => {
      const cur = s.buffers[draftId];
      if (!cur) return s;
      return {
        buffers: {
          ...s.buffers,
          [draftId]: { ...cur, submitState, errorMessage },
        },
      };
    }),
}));

interface WorkspaceStore {
  presetId: WorkspacePresetId;
  viewport: ViewportSize;
  navCollapsed: boolean;
  windows: WindowState[];
  activeInstanceId: string | null;
  undoSnapshot: { presetId: WorkspacePresetId; windows: WindowState[]; activeInstanceId: string | null } | null;
  topFloatingZ: number;
  topPinnedZ: number;
  keyboardMoveTargetId: string | null;

  setViewport: (viewport: ViewportSize) => void;
  toggleNavCollapsed: () => void;
  focusWindow: (instanceId: string) => void;
  cycleFocus: (reverse?: boolean) => void;
  setWindowBounds: (instanceId: string, bounds: WindowBounds) => void;
  toggleMinimize: (instanceId: string) => void;
  toggleMaximize: (instanceId: string) => void;
  togglePinned: (instanceId: string) => void;
  setDisplayMode: (instanceId: string, mode: DisplayMode, dockRegion?: DockRegion) => void;
  closeWindow: (instanceId: string) => void;
  openTopicWindow: (topicId: number, title: string, opts?: { newWindow?: boolean; targetPostNumber?: number }) => string;
  openComposerWindow: (opts: { draftId?: string; topicId?: number; title: string; mode: DraftBuffer['mode'] }) => string;
  openSingletonWindow: (panelType: PanelType, title: string) => string;
  applyPreset: (preset: WorkspacePresetId) => void;
  arrangeWindows: (mode: 'cascade' | 'tileH' | 'tileV') => void;
  showAllWindows: () => void;
  recoverOffscreenWindows: () => void;
  undoLastLayout: () => void;
  setKeyboardMoveTarget: (instanceId: string | null) => void;
  nudgeWindow: (instanceId: string, dx: number, dy: number, resize?: boolean) => void;
}

const DEFAULT_VIEWPORT: ViewportSize = { width: 1140, height: 680 };

function snapshotForUndo(s: WorkspaceStore) {
  return {
    presetId: s.presetId,
    windows: structuredClone(s.windows),
    activeInstanceId: s.activeInstanceId,
  };
}

function nextFallbackFocus(windows: WindowState[], closedId: string): string | null {
  const candidates = windows
    .filter((w) => w.instanceId !== closedId && w.viewState !== 'minimized')
    .sort((a, b) => b.zIndex - a.zIndex);
  return candidates[0]?.instanceId ?? null;
}

export const useWorkspaceStore = create<WorkspaceStore>((set, get) => ({
  presetId: 'browse',
  viewport: DEFAULT_VIEWPORT,
  navCollapsed: false,
  windows: buildPresetWindows('browse', DEFAULT_VIEWPORT),
  activeInstanceId: 'composer-welcome',
  undoSnapshot: null,
  topFloatingZ: 20,
  topPinnedZ: 110,
  keyboardMoveTargetId: null,

  setViewport: (viewport) =>
    set((s) => ({
      viewport,
      navCollapsed: viewport.width < 960 ? true : s.navCollapsed,
      windows: s.windows.map((w) => ({
        ...w,
        bounds: clampBoundsToViewport(w.bounds, viewport),
      })),
    })),

  toggleNavCollapsed: () => set((s) => ({ navCollapsed: !s.navCollapsed })),

  focusWindow: (instanceId) =>
    set((s) => {
      const target = s.windows.find((w) => w.instanceId === instanceId);
      if (!target) return s;

      const nextZ = target.pinned ? s.topPinnedZ + 1 : s.topFloatingZ + 1;
      return {
        activeInstanceId: instanceId,
        topPinnedZ: target.pinned ? nextZ : s.topPinnedZ,
        topFloatingZ: target.pinned ? s.topFloatingZ : nextZ,
        windows: s.windows.map((w) => {
          if (w.instanceId !== instanceId) return w;
          return {
            ...w,
            viewState: w.viewState === 'minimized' ? 'normal' : w.viewState,
            zIndex: w.displayMode === 'floating' ? nextZ : w.zIndex,
          };
        }),
      };
    }),

  cycleFocus: (reverse = false) =>
    set((s) => {
      const visible = s.windows.filter((w) => w.viewState !== 'minimized');
      if (visible.length === 0) return s;
      const curIdx = visible.findIndex((w) => w.instanceId === s.activeInstanceId);
      const nextIdx =
        curIdx === -1
          ? 0
          : (curIdx + (reverse ? -1 : 1) + visible.length) % visible.length;
      const target = visible[nextIdx];
      const nextZ = target.pinned ? s.topPinnedZ + 1 : s.topFloatingZ + 1;
      return {
        activeInstanceId: target.instanceId,
        topPinnedZ: target.pinned ? nextZ : s.topPinnedZ,
        topFloatingZ: target.pinned ? s.topFloatingZ : nextZ,
        windows: s.windows.map((w) =>
          w.instanceId === target.instanceId && w.displayMode === 'floating'
            ? { ...w, zIndex: nextZ }
            : w,
        ),
      };
    }),

  setWindowBounds: (instanceId, bounds) =>
    set((s) => ({
      windows: s.windows.map((w) =>
        w.instanceId === instanceId
          ? {
              ...w,
              bounds: clampBoundsToViewport(bounds, s.viewport),
              restoreBounds: w.viewState === 'normal' ? clampBoundsToViewport(bounds, s.viewport) : w.restoreBounds,
            }
          : w,
      ),
    })),

  toggleMinimize: (instanceId) =>
    set((s) => {
      const target = s.windows.find((w) => w.instanceId === instanceId);
      if (!target) return s;
      const minimizing = target.viewState !== 'minimized';
      const nextZ = s.topFloatingZ + 1;
      return {
        undoSnapshot: snapshotForUndo(s),
        topFloatingZ: minimizing ? s.topFloatingZ : nextZ,
        activeInstanceId: minimizing
          ? s.activeInstanceId === instanceId
            ? nextFallbackFocus(s.windows, instanceId)
            : s.activeInstanceId
          : instanceId,
        windows: s.windows.map((w) =>
          w.instanceId === instanceId
            ? {
                ...w,
                viewState: minimizing ? 'minimized' : 'normal',
                zIndex: minimizing ? w.zIndex : nextZ,
              }
            : w,
        ),
      };
    }),

  toggleMaximize: (instanceId) =>
    set((s) => {
      const target = s.windows.find((w) => w.instanceId === instanceId);
      if (!target) return s;
      const maximizing = target.viewState !== 'maximized';
      const nextZ = target.pinned ? s.topPinnedZ + 1 : s.topFloatingZ + 1;
      return {
        undoSnapshot: snapshotForUndo(s),
        activeInstanceId: instanceId,
        topPinnedZ: target.pinned ? nextZ : s.topPinnedZ,
        topFloatingZ: target.pinned ? s.topFloatingZ : nextZ,
        windows: s.windows.map((w) => {
          if (w.instanceId !== instanceId) return w;
          if (maximizing) {
            return {
              ...w,
              restoreBounds: w.viewState === 'normal' ? w.bounds : w.restoreBounds,
              viewState: 'maximized',
              zIndex: w.displayMode === 'floating' ? nextZ : w.zIndex,
            };
          }
          return {
            ...w,
            viewState: 'normal',
            bounds: clampBoundsToViewport(w.restoreBounds, s.viewport),
            zIndex: w.displayMode === 'floating' ? nextZ : w.zIndex,
          };
        }),
      };
    }),

  togglePinned: (instanceId) =>
    set((s) => {
      const nextPinnedZ = s.topPinnedZ + 1;
      const nextFloatZ = s.topFloatingZ + 1;
      return {
        topPinnedZ: nextPinnedZ,
        topFloatingZ: nextFloatZ,
        windows: s.windows.map((w) =>
          w.instanceId === instanceId
            ? {
                ...w,
                pinned: !w.pinned,
                zIndex: !w.pinned ? nextPinnedZ : nextFloatZ,
              }
            : w,
        ),
      };
    }),

  setDisplayMode: (instanceId, displayMode, dockRegion = 'left') =>
    set((s) => {
      const nextZ = s.topFloatingZ + 1;
      return {
        undoSnapshot: snapshotForUndo(s),
        topFloatingZ: nextZ,
        activeInstanceId: instanceId,
        windows: s.windows.map((w) =>
          w.instanceId === instanceId
            ? {
                ...w,
                displayMode,
                dockRegion: displayMode === 'docked' ? dockRegion : undefined,
                viewState: 'normal',
                bounds: displayMode === 'floating' ? clampBoundsToViewport(w.restoreBounds, s.viewport) : w.bounds,
                zIndex: displayMode === 'floating' ? nextZ : 1,
              }
            : w,
        ),
      };
    }),

  closeWindow: (instanceId) =>
    set((s) => ({
      undoSnapshot: snapshotForUndo(s),
      windows: s.windows.filter((w) => w.instanceId !== instanceId),
      activeInstanceId:
        s.activeInstanceId === instanceId ? nextFallbackFocus(s.windows, instanceId) : s.activeInstanceId,
    })),

  openTopicWindow: (topicId, title, opts) => {
    const s = get();
    const existing = !opts?.newWindow
      ? s.windows.find((w) => w.panelType === 'topic' && w.topicId === topicId)
      : undefined;
    if (existing) {
      set((cur) => ({
        activeInstanceId: existing.instanceId,
        topFloatingZ: cur.topFloatingZ + 1,
        windows: cur.windows.map((w) =>
          w.instanceId === existing.instanceId
            ? {
                ...w,
                viewState: w.viewState === 'minimized' ? 'normal' : w.viewState,
                targetPostNumber: opts?.targetPostNumber ?? w.targetPostNumber,
                zIndex: w.displayMode === 'floating' ? cur.topFloatingZ + 1 : w.zIndex,
              }
            : w,
        ),
      }));
      return existing.instanceId;
    }

    const offset = (s.windows.length % 5) * 24;
    const instanceId = `topic-${topicId}-${Date.now().toString(36)}`;
    const bounds = clampBoundsToViewport(
      { x: 360 + offset, y: 24 + offset, width: 620, height: 480 },
      s.viewport,
    );
    const nextZ = s.topFloatingZ + 1;
    set((cur) => ({
      topFloatingZ: nextZ,
      activeInstanceId: instanceId,
      windows: [
        ...cur.windows,
        {
          instanceId,
          panelType: 'topic',
          title,
          resourceKey: `topic:${topicId}`,
          displayMode: 'floating',
          viewState: 'normal',
          bounds,
          restoreBounds: bounds,
          zIndex: nextZ,
          pinned: false,
          topicId,
          targetPostNumber: opts?.targetPostNumber,
        },
      ],
    }));
    return instanceId;
  },

  openComposerWindow: ({ draftId, topicId, title, mode }) => {
    const resolvedDraftId = draftId ?? `draft-${Date.now().toString(36)}`;
    useDraftBufferStore.getState().ensureBuffer(resolvedDraftId, { mode, topicId, title });

    const s = get();
    const existing = s.windows.find((w) => w.panelType === 'composer' && w.draftId === resolvedDraftId);
    if (existing) {
      s.focusWindow(existing.instanceId);
      return existing.instanceId;
    }

    const instanceId = `composer-${resolvedDraftId}`;
    const bounds = clampBoundsToViewport(
      { x: Math.max(40, s.viewport.width - 500), y: Math.max(40, s.viewport.height - 340), width: 460, height: 310 },
      s.viewport,
    );
    const nextZ = s.topFloatingZ + 1;
    set((cur) => ({
      topFloatingZ: nextZ,
      activeInstanceId: instanceId,
      windows: [
        ...cur.windows,
        {
          instanceId,
          panelType: 'composer',
          title,
          resourceKey: `draft:${resolvedDraftId}`,
          displayMode: 'floating',
          viewState: 'normal',
          bounds,
          restoreBounds: bounds,
          zIndex: nextZ,
          pinned: false,
          topicId,
          draftId: resolvedDraftId,
        },
      ],
    }));
    return instanceId;
  },

  openSingletonWindow: (panelType, title) => {
    const s = get();
    const existing = s.windows.find((w) => w.panelType === panelType);
    if (existing) {
      s.focusWindow(existing.instanceId);
      return existing.instanceId;
    }
    const instanceId = `${panelType}-main`;
    const bounds = clampBoundsToViewport({ x: 180, y: 48, width: 560, height: 460 }, s.viewport);
    const nextZ = s.topFloatingZ + 1;
    set((cur) => ({
      topFloatingZ: nextZ,
      activeInstanceId: instanceId,
      windows: [
        ...cur.windows,
        {
          instanceId,
          panelType,
          title,
          resourceKey: `${panelType}:main`,
          displayMode: 'floating',
          viewState: 'normal',
          bounds,
          restoreBounds: bounds,
          zIndex: nextZ,
          pinned: false,
        },
      ],
    }));
    return instanceId;
  },

  applyPreset: (presetId) =>
    set((s) => {
      const nextWindows = buildPresetWindows(presetId, s.viewport);
      return {
        undoSnapshot: snapshotForUndo(s),
        presetId,
        windows: nextWindows,
        activeInstanceId: nextWindows.find((w) => w.viewState !== 'minimized')?.instanceId ?? null,
      };
    }),

  arrangeWindows: (mode) =>
    set((s) => {
      const nextMap = arrangeFloatingWindows(s.windows, mode, s.viewport);
      return {
        undoSnapshot: snapshotForUndo(s),
        windows: s.windows.map((w) =>
          nextMap[w.instanceId]
            ? { ...w, viewState: 'normal', bounds: nextMap[w.instanceId], restoreBounds: nextMap[w.instanceId] }
            : w,
        ),
      };
    }),

  showAllWindows: () =>
    set((s) => ({
      undoSnapshot: snapshotForUndo(s),
      windows: s.windows.map((w) => ({
        ...w,
        viewState: w.viewState === 'minimized' ? 'normal' : w.viewState,
      })),
    })),

  recoverOffscreenWindows: () =>
    set((s) => ({
      undoSnapshot: snapshotForUndo(s),
      windows: s.windows.map((w) => {
        const recovered = recoverBoundsInsideViewport(w.bounds, s.viewport);
        return {
          ...w,
          viewState: w.viewState === 'minimized' ? 'normal' : w.viewState,
          bounds: recovered,
          restoreBounds: recovered,
        };
      }),
    })),

  undoLastLayout: () =>
    set((s) => {
      if (!s.undoSnapshot) return s;
      return {
        presetId: s.undoSnapshot.presetId,
        windows: s.undoSnapshot.windows,
        activeInstanceId: s.undoSnapshot.activeInstanceId,
        undoSnapshot: null,
      };
    }),

  setKeyboardMoveTarget: (instanceId) => set({ keyboardMoveTargetId: instanceId }),

  nudgeWindow: (instanceId, dx, dy, resize = false) =>
    set((s) => ({
      windows: s.windows.map((w) => {
        if (w.instanceId !== instanceId) return w;
        const nextBounds = resize
          ? { ...w.bounds, width: w.bounds.width + dx, height: w.bounds.height + dy }
          : { ...w.bounds, x: w.bounds.x + dx, y: w.bounds.y + dy };
        const clamped = clampBoundsToViewport(nextBounds, s.viewport);
        return { ...w, bounds: clamped, restoreBounds: clamped };
      }),
    })),
}));
