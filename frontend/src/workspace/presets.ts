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
        bounds: { x: 16, y: 16, width: 320, height: 480 },
        restoreBounds: { x: 16, y: 16, width: 320, height: 480 },
        zIndex: 10,
        pinned: false,
      },
      {
        instanceId: 'topic-1001',
        panelType: 'topic',
        title: '主题阅读',
        resourceKey: 'topic:1001',
        displayMode: 'docked',
        viewState: 'normal',
        dockRegion: 'center',
        bounds: { x: 0, y: 0, width: w, height: h },
        restoreBounds: { x: 80, y: 24, width: Math.min(780, w - 120), height: Math.max(360, h - 64) },
        zIndex: 12,
        pinned: false,
        topicId: 1001,
      },
    ];
  }

  if (preset === 'writing') {
    const leftW = Math.max(320, Math.floor((w - 24) * 0.52));
    const rightW = Math.max(320, w - 24 - leftW);
    return [
      {
        instanceId: 'topic-1001',
        panelType: 'topic',
        title: '主题阅读',
        resourceKey: 'topic:1001',
        displayMode: 'docked',
        viewState: 'normal',
        dockRegion: 'left',
        bounds: { x: 0, y: 0, width: leftW, height: h },
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
        bounds: { x: leftW + 8, y: 0, width: rightW, height: h },
        restoreBounds: { x: leftW + 8, y: 12, width: rightW, height: h - 24 },
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
        bounds: { x: 16, y: 16, width: Math.min(560, w - 80), height: Math.min(460, h - 48) },
        restoreBounds: { x: 16, y: 16, width: Math.min(560, w - 80), height: Math.min(460, h - 48) },
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
        bounds: { x: Math.min(260, w - 420), y: 48, width: Math.min(540, w - 80), height: Math.min(430, h - 72) },
        restoreBounds: { x: Math.min(260, w - 420), y: 48, width: Math.min(540, w - 80), height: Math.min(430, h - 72) },
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
        bounds: { x: Math.max(40, w - 480), y: Math.max(40, h - 340), width: 450, height: 310 },
        restoreBounds: { x: Math.max(40, w - 480), y: Math.max(40, h - 340), width: 450, height: 310 },
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
        bounds: { x: 48, y: 48, width: 320, height: 420 },
        restoreBounds: { x: 48, y: 48, width: 320, height: 420 },
        zIndex: 10,
        pinned: false,
        unreadBadge: 2,
      },
    ];
  }

  // Default compact "browse" layout:
  // Docked left topic list + docked center reading view + docked right vertical notifications strip (full height)
  return [
    {
      instanceId: 'feed-latest',
      panelType: 'topicList',
      title: '最新话题',
      resourceKey: 'feed:latest',
      displayMode: 'docked',
      viewState: 'normal',
      dockRegion: 'left',
      bounds: { x: 0, y: 0, width: 300, height: h },
      restoreBounds: { x: 24, y: 24, width: 340, height: Math.min(520, h - 48) },
      zIndex: 1,
      pinned: false,
    },
    {
      instanceId: 'topic-1001',
      panelType: 'topic',
      title: '主题阅读',
      resourceKey: 'topic:1001',
      displayMode: 'docked',
      viewState: 'normal',
      dockRegion: 'center',
      bounds: { x: 306, y: 0, width: Math.max(320, w - 580), height: h },
      restoreBounds: { x: 180, y: 24, width: Math.min(640, w - 220), height: Math.min(520, h - 48) },
      zIndex: 2,
      pinned: false,
      topicId: 1001,
    },
    {
      instanceId: 'notifications-main',
      panelType: 'notifications',
      title: '通知',
      resourceKey: 'notifications:main',
      displayMode: 'docked',
      viewState: 'normal',
      dockRegion: 'right',
      bounds: { x: Math.max(600, w - 268), y: 0, width: 268, height: h },
      restoreBounds: { x: Math.max(40, w - 340), y: 24, width: 310, height: Math.min(520, h - 48) },
      zIndex: 3,
      pinned: false,
    },
  ];
}
