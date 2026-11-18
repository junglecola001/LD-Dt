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
  // All three panels stay floating (draggable & edge-resizable) while tiling
  // seamlessly across the full viewport, with notifications as a full-height right strip.
  const gap = 6;
  const fullH = Math.max(260, h - gap * 2);
  const availW = Math.max(760, w - gap * 4);
  const leftW = Math.min(316, Math.max(250, Math.round(availW * 0.27)));
  const rightW = Math.min(264, Math.max(220, Math.round(availW * 0.22)));
  const centerX = gap + leftW + gap;
  const centerW = Math.max(280, w - centerX - rightW - gap * 2);
  const rightX = centerX + centerW + gap;

  return [
    {
      instanceId: 'feed-latest',
      panelType: 'topicList',
      title: '最新话题',
      resourceKey: 'feed:latest',
      displayMode: 'floating',
      viewState: 'normal',
      bounds: { x: gap, y: gap, width: leftW, height: fullH },
      restoreBounds: { x: gap, y: gap, width: leftW, height: fullH },
      zIndex: 11,
      pinned: false,
    },
    {
      instanceId: 'topic-1001',
      panelType: 'topic',
      title: '主题阅读',
      resourceKey: 'topic:1001',
      displayMode: 'floating',
      viewState: 'normal',
      bounds: { x: centerX, y: gap, width: centerW, height: fullH },
      restoreBounds: { x: centerX, y: gap, width: centerW, height: fullH },
      zIndex: 12,
      pinned: false,
      topicId: 1001,
    },
    {
      instanceId: 'notifications-main',
      panelType: 'notifications',
      title: '通知',
      resourceKey: 'notifications:main',
      displayMode: 'floating',
      viewState: 'normal',
      bounds: { x: rightX, y: gap, width: Math.max(220, w - rightX - gap), height: fullH },
      restoreBounds: { x: rightX, y: gap, width: Math.max(220, w - rightX - gap), height: fullH },
      zIndex: 13,
      pinned: false,
    },
  ];
}
