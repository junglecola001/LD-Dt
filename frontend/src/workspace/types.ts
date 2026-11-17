export type PanelType =
  | 'topicList'
  | 'topic'
  | 'composer'
  | 'search'
  | 'notifications'
  | 'bookmarks'
  | 'designBaseline';

export type DisplayMode = 'floating' | 'docked';
export type ViewState = 'normal' | 'minimized' | 'maximized';
export type DockRegion = 'left' | 'right' | 'top' | 'bottom' | 'center';
export type WorkspacePresetId = 'browse' | 'reading' | 'writing' | 'freeform';

export interface WindowBounds {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface WindowState {
  instanceId: string;
  panelType: PanelType;
  title: string;
  resourceKey: string;
  displayMode: DisplayMode;
  viewState: ViewState;
  dockRegion?: DockRegion;
  bounds: WindowBounds;
  restoreBounds: WindowBounds;
  zIndex: number;
  pinned: boolean;
  topicId?: number;
  targetPostNumber?: number;
  draftId?: string;
  unreadBadge?: number;
  hasUnsavedDraft?: boolean;
}

export interface ViewportSize {
  width: number;
  height: number;
}

export interface SnapGuide {
  axis: 'x' | 'y';
  pos: number;
}

export interface ResizeHandle {
  n?: boolean;
  s?: boolean;
  e?: boolean;
  w?: boolean;
}
