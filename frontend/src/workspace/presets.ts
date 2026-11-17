import type { ViewportSize, WindowState, WorkspacePresetId } from './types';

export function buildPresetWindows(preset: WorkspacePresetId, viewport: ViewportSize): WindowState[] {
  const w = Math.max(800, viewport.width);
  const h = Math.max(460, viewport.height);

  if (preset === 'reading') {
    return [
      {
        instanceId: 'feed-latest',
        panelType: 'topicList',
        title: '最新话题',
        resourceKey: 'feed:latest',
        displayMode: 'floating',
        viewState: 'minimized',
        bounds: { x: 16, y: 16, width: 340, height: 480 },
        restoreBounds: { x: 16, y: 16, width: 340, height: 480 },
        zIndex: 10,
        pinned: false,
      },
      {
        instanceId: 'topic-1001',
        panelType: 'topic',
        title: '欢迎来到 LD Studio：Go + Wails + React 桌面工作台样板',
        resourceKey: 'topic:1001',
        displayMode: 'floating',
        viewState: 'maximized',
        bounds: { x: 24, y: 24, width: Math.min(820, w - 48), height: Math.max(360, h - 48) },
        restoreBounds: { x: 80, y: 24, width: Math.min(780, w - 120), height: Math.max(360, h - 64) },
        zIndex: 12,
        pinned: false,
        topicId: 1001,
      },
    ];
  }

  if (preset === 'writing') {
    const leftW = Math.max(320, Math.floor((w - 36) * 0.52));
    const rightW = Math.max(320, w - 36 - leftW);
    return [
      {
        instanceId: 'topic-1001',
        panelType: 'topic',
        title: '欢迎来到 LD Studio：Go + Wails + React 桌面工作台样板',
        resourceKey: 'topic:1001',
        displayMode: 'docked',
        viewState: 'normal',
        dockRegion: 'left',
        bounds: { x: 12, y: 12, width: leftW, height: h - 24 },
        restoreBounds: { x: 12, y: 12, width: leftW, height: h - 24 },
        zIndex: 1,
        pinned: false,
        topicId: 1001,
      },
      {
        instanceId: 'composer-welcome',
        panelType: 'composer',
        title: '回复草稿 · #1001',
        resourceKey: 'draft:draft-welcome',
        displayMode: 'docked',
        viewState: 'normal',
        dockRegion: 'right',
        bounds: { x: 24 + leftW, y: 12, width: rightW, height: h - 24 },
        restoreBounds: { x: 24 + leftW, y: 12, width: rightW, height: h - 24 },
        zIndex: 2,
        pinned: false,
        topicId: 1001,
        draftId: 'draft-welcome',
        hasUnsavedDraft: false,
      },
    ];
  }

  if (preset === 'freeform') {
    return [
      {
        instanceId: 'topic-1001',
        panelType: 'topic',
        title: '欢迎来到 LD Studio：Go + Wails + React 桌面工作台样板',
        resourceKey: 'topic:1001',
        displayMode: 'floating',
        viewState: 'normal',
        bounds: { x: 24, y: 20, width: Math.min(560, w - 80), height: Math.min(460, h - 60) },
        restoreBounds: { x: 24, y: 20, width: Math.min(560, w - 80), height: Math.min(460, h - 60) },
        zIndex: 11,
        pinned: false,
        topicId: 1001,
      },
      {
        instanceId: 'topic-1002',
        panelType: 'topic',
        title: '长标题与超宽表格压力样本',
        resourceKey: 'topic:1002',
        displayMode: 'floating',
        viewState: 'normal',
        bounds: { x: Math.min(260, w - 420), y: 56, width: Math.min(540, w - 80), height: Math.min(430, h - 80) },
        restoreBounds: { x: Math.min(260, w - 420), y: 56, width: Math.min(540, w - 80), height: Math.min(430, h - 80) },
        zIndex: 12,
        pinned: false,
        topicId: 1002,
      },
      {
        instanceId: 'composer-welcome',
        panelType: 'composer',
        title: '回复草稿 · #1001',
        resourceKey: 'draft:draft-welcome',
        displayMode: 'floating',
        viewState: 'normal',
        bounds: { x: Math.max(40, w - 500), y: Math.max(40, h - 360), width: 460, height: 320 },
        restoreBounds: { x: Math.max(40, w - 500), y: Math.max(40, h - 360), width: 460, height: 320 },
        zIndex: 13,
        pinned: false,
        topicId: 1001,
        draftId: 'draft-welcome',
      },
      {
        instanceId: 'notifications-main',
        panelType: 'notifications',
        title: '通知',
        resourceKey: 'notifications:main',
        displayMode: 'floating',
        viewState: 'minimized',
        bounds: { x: 48, y: 48, width: 380, height: 400 },
        restoreBounds: { x: 48, y: 48, width: 380, height: 400 },
        zIndex: 10,
        pinned: false,
        unreadBadge: 2,
      },
    ];
  }

  // Default "browse" preset: docked topic list on left + two overlapping floating windows + minimized notifications
  const dockedListW = Math.min(340, Math.max(300, Math.floor(w * 0.31)));
  const floatBaseX = dockedListW + 18;
  const availFloatW = Math.max(320, w - floatBaseX - 18);

  return [
    {
      instanceId: 'feed-latest',
      panelType: 'topicList',
      title: '最新话题',
      resourceKey: 'feed:latest',
      displayMode: 'docked',
      viewState: 'normal',
      dockRegion: 'left',
      bounds: { x: 0, y: 0, width: dockedListW, height: h },
      restoreBounds: { x: 0, y: 0, width: dockedListW, height: h },
      zIndex: 1,
      pinned: false,
    },
    {
      instanceId: 'topic-1001',
      panelType: 'topic',
      title: '欢迎来到 LD Studio：Go + Wails + React 桌面工作台样板',
      resourceKey: 'topic:1001',
      displayMode: 'floating',
      viewState: 'normal',
      bounds: { x: floatBaseX, y: 16, width: Math.min(620, availFloatW), height: Math.min(480, h - 60) },
      restoreBounds: { x: floatBaseX, y: 16, width: Math.min(620, availFloatW), height: Math.min(480, h - 60) },
      zIndex: 11,
      pinned: false,
      topicId: 1001,
    },
    {
      instanceId: 'composer-welcome',
      panelType: 'composer',
      title: '回复草稿 · #1001',
      resourceKey: 'draft:draft-welcome',
      displayMode: 'floating',
      viewState: 'normal',
      bounds: {
        x: Math.max(floatBaseX + 32, w - 480),
        y: Math.max(64, h - 340),
        width: Math.min(456, availFloatW),
        height: 300,
      },
      restoreBounds: {
        x: Math.max(floatBaseX + 32, w - 480),
        y: Math.max(64, h - 340),
        width: Math.min(456, availFloatW),
        height: 300,
      },
      zIndex: 12,
      pinned: false,
      topicId: 1001,
      draftId: 'draft-welcome',
    },
    {
      instanceId: 'notifications-main',
      panelType: 'notifications',
      title: '通知',
      resourceKey: 'notifications:main',
      displayMode: 'floating',
      viewState: 'minimized',
      bounds: { x: floatBaseX + 24, y: 40, width: 380, height: 400 },
      restoreBounds: { x: floatBaseX + 24, y: 40, width: 380, height: 400 },
      zIndex: 10,
      pinned: false,
      unreadBadge: 2,
    },
  ];
}
