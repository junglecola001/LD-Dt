import { useEffect, useRef, useState } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import {
  Bell,
  Compass,
  ListBullets,
  MagnifyingGlass,
  Moon,
  NotePencil,
  Palette,
  SidebarSimple,
  Sun,
  WarningCircle,
} from '@phosphor-icons/react';
import { getDesktopBridge, type NetworkPhase, type NetworkStatusSnapshot } from '../bridge';
import { Button, IconButton } from '../components/Primitives';
import uiStyles from '../components/ui.module.css';
import { ComposerPanel } from '../features/ComposerPanel';
import { DesignBaselinePanel } from '../features/DesignBaselinePanel';
import { NotificationsPanel } from '../features/NotificationsPanel';
import { SearchPanel } from '../features/SearchPanel';
import { TopicListPanel } from '../features/TopicListPanel';
import { TopicPanel } from '../features/TopicPanel';
import { useWorkspaceStore } from '../workspace/store';
import type { SnapGuide, WindowState, WorkspacePresetId } from '../workspace/types';
import styles from './AppShell.module.css';
import { WindowFrame } from './WindowFrame';

const PRESET_LABELS: Record<WorkspacePresetId, string> = {
  browse: '浏览',
  reading: '阅读',
  writing: '写作',
  freeform: '自由工作台',
};

export function AppShell() {
  const [theme, setTheme] = useState<'light' | 'dark'>('light');
  const [lowResource, setLowResource] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [paletteFilter, setPaletteFilter] = useState('');
  const [snapGuides, setSnapGuides] = useState<SnapGuide[]>([]);
  const [network, setNetwork] = useState<NetworkStatusSnapshot>({
    siteId: 'linux.do',
    phase: 'ready',
    blockedUntilUnix: 0,
    retryAfterSeconds: 0,
    inFlightCount: 0,
    queuedCount: 0,
    message: '',
    isDemoData: getDesktopBridge().isMock,
  });

  const viewportRef = useRef<HTMLDivElement>(null);

  const presetId = useWorkspaceStore((s) => s.presetId);
  const viewport = useWorkspaceStore((s) => s.viewport);
  const navCollapsed = useWorkspaceStore((s) => s.navCollapsed);
  const windows = useWorkspaceStore((s) => s.windows);
  const activeInstanceId = useWorkspaceStore((s) => s.activeInstanceId);

  const setViewport = useWorkspaceStore((s) => s.setViewport);
  const toggleNavCollapsed = useWorkspaceStore((s) => s.toggleNavCollapsed);
  const focusWindow = useWorkspaceStore((s) => s.focusWindow);
  const cycleFocus = useWorkspaceStore((s) => s.cycleFocus);
  const closeWindow = useWorkspaceStore((s) => s.closeWindow);
  const applyPreset = useWorkspaceStore((s) => s.applyPreset);
  const arrangeWindows = useWorkspaceStore((s) => s.arrangeWindows);
  const showAllWindows = useWorkspaceStore((s) => s.showAllWindows);
  const recoverOffscreenWindows = useWorkspaceStore((s) => s.recoverOffscreenWindows);
  const undoLastLayout = useWorkspaceStore((s) => s.undoLastLayout);
  const openComposerWindow = useWorkspaceStore((s) => s.openComposerWindow);
  const openSingletonWindow = useWorkspaceStore((s) => s.openSingletonWindow);
  const setKeyboardMoveTarget = useWorkspaceStore((s) => s.setKeyboardMoveTarget);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
  }, [theme]);

  useEffect(() => {
    document.documentElement.setAttribute('data-low-resource', lowResource ? 'true' : 'false');
  }, [lowResource]);

  useEffect(() => {
    void getDesktopBridge().getNetworkStatus().then(setNetwork);
  }, []);

  useEffect(() => {
    const el = viewportRef.current;
    if (!el || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setViewport({
          width: Math.max(360, Math.round(entry.contentRect.width)),
          height: Math.max(260, Math.round(entry.contentRect.height)),
        });
      }
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, [setViewport]);

  // Global keyboard shortcuts (plan.md 4.5)
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.isComposing) return;
      if (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === 'p') {
        e.preventDefault();
        setPaletteOpen((v) => !v);
      } else if (e.ctrlKey && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        openSingletonWindow('search', '全局搜索');
      } else if (e.ctrlKey && e.key.toLowerCase() === 'n') {
        e.preventDefault();
        openComposerWindow({ mode: 'newTopic', title: '新建主题草稿' });
      } else if (e.ctrlKey && e.key.toLowerCase() === 'w') {
        e.preventDefault();
        if (activeInstanceId) closeWindow(activeInstanceId);
      } else if (e.ctrlKey && e.key === 'Tab') {
        e.preventDefault();
        cycleFocus(e.shiftKey);
      } else if (e.ctrlKey && ['1', '2', '3', '4'].includes(e.key)) {
        e.preventDefault();
        const presets: WorkspacePresetId[] = ['browse', 'reading', 'writing', 'freeform'];
        applyPreset(presets[Number(e.key) - 1]);
      } else if (e.key === 'Escape' && paletteOpen) {
        setPaletteOpen(false);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [activeInstanceId, applyPreset, closeWindow, cycleFocus, openComposerWindow, openSingletonWindow, paletteOpen]);

  const handleNetworkPhaseChange = async (phase: NetworkPhase) => {
    const bridge = getDesktopBridge();
    if (bridge.simulateNetworkPhase) {
      const next = await bridge.simulateNetworkPhase(phase);
      setNetwork(next);
    }
  };

  const renderPanelContent = (win: WindowState) => {
    switch (win.panelType) {
      case 'topicList':
        return <TopicListPanel instanceId={win.instanceId} />;
      case 'topic':
        return (
          <TopicPanel
            instanceId={win.instanceId}
            topicId={win.topicId ?? 1001}
            targetPostNumber={win.targetPostNumber}
          />
        );
      case 'composer':
        return <ComposerPanel draftId={win.draftId ?? 'draft-welcome'} />;
      case 'search':
        return <SearchPanel instanceId={win.instanceId} />;
      case 'notifications':
        return <NotificationsPanel />;
      case 'designBaseline':
        return (
          <DesignBaselinePanel
            theme={theme}
            onToggleTheme={() => setTheme((t) => (t === 'light' ? 'dark' : 'light'))}
            lowResource={lowResource}
            onToggleLowResource={() => setLowResource((v) => !v)}
            onNetworkPhaseChange={handleNetworkPhaseChange}
          />
        );
      default:
        return null;
    }
  };

  const dockedWindows = windows.filter((w) => w.displayMode === 'docked' && w.viewState !== 'minimized');
  const floatingWindows = windows.filter((w) => w.displayMode === 'floating' && w.viewState !== 'minimized');

  const commands = [
    { id: 'new-draft', label: '新建主题草稿', shortcut: 'Ctrl+N', run: () => openComposerWindow({ mode: 'newTopic', title: '新建主题草稿' }) },
    { id: 'search', label: '打开全局搜索', shortcut: 'Ctrl+K', run: () => openSingletonWindow('search', '全局搜索') },
    { id: 'notif', label: '打开通知面板', shortcut: '', run: () => openSingletonWindow('notifications', '通知') },
    { id: 'baseline', label: '打开视觉基线对照', shortcut: '', run: () => openSingletonWindow('designBaseline', '视觉基线') },
    { id: 'cascade', label: '层叠排列浮动窗口', shortcut: '', run: () => arrangeWindows('cascade') },
    { id: 'tile-h', label: '横向平铺浮动窗口', shortcut: '', run: () => arrangeWindows('tileH') },
    { id: 'recover', label: '找回所有离屏窗口', shortcut: '', run: () => recoverOffscreenWindows() },
    { id: 'theme', label: theme === 'light' ? '切换到深色模式' : '切换到浅色模式', shortcut: '', run: () => setTheme((t) => (t === 'light' ? 'dark' : 'light')) },
  ].filter((c) => !paletteFilter.trim() || c.label.toLowerCase().includes(paletteFilter.toLowerCase()));

  return (
    <div className={styles.shell}>
      <header className={styles.appTitlebar}>
        <div className={styles.brandGroup}>
          <span className={styles.brandDot} aria-hidden="true" />
          <span>LD Studio</span>
          {getDesktopBridge().isMock && <span className={styles.demoTag}>演示数据</span>}
        </div>
        <span style={{ fontSize: '12px', color: 'var(--ink-muted)' }}>
          工作区：{PRESET_LABELS[presetId]}
        </span>
      </header>

      <div className={styles.commandBar}>
        <div className={styles.barLeft}>
          <IconButton label="折叠或展开导航" onClick={toggleNavCollapsed}>
            <SidebarSimple size={16} />
          </IconButton>

          <DropdownMenu.Root>
            <DropdownMenu.Trigger asChild>
              <Button variant="ghost" style={{ height: 28 }}>
                窗口
              </Button>
            </DropdownMenu.Trigger>
            <DropdownMenu.Portal>
              <DropdownMenu.Content className={styles.menuContent} align="start" sideOffset={4}>
                <DropdownMenu.Item className={styles.menuItem} onSelect={() => arrangeWindows('cascade')}>
                  层叠排列
                </DropdownMenu.Item>
                <DropdownMenu.Item className={styles.menuItem} onSelect={() => arrangeWindows('tileH')}>
                  横向平铺
                </DropdownMenu.Item>
                <DropdownMenu.Item className={styles.menuItem} onSelect={() => arrangeWindows('tileV')}>
                  纵向平铺
                </DropdownMenu.Item>
                <DropdownMenu.Item className={styles.menuItem} onSelect={showAllWindows}>
                  显示所有窗口
                </DropdownMenu.Item>
                <DropdownMenu.Item className={styles.menuItem} onSelect={recoverOffscreenWindows}>
                  找回离屏窗口
                </DropdownMenu.Item>
                <DropdownMenu.Separator className={styles.menuSeparator} />
                <DropdownMenu.Item
                  className={styles.menuItem}
                  disabled={!activeInstanceId}
                  onSelect={() => activeInstanceId && setKeyboardMoveTarget(activeInstanceId)}
                >
                  键盘移动活动窗口
                </DropdownMenu.Item>
                <DropdownMenu.Item className={styles.menuItem} onSelect={undoLastLayout}>
                  撤销上次布局变更
                </DropdownMenu.Item>
                <DropdownMenu.Item className={styles.menuItem} onSelect={() => applyPreset('browse')}>
                  恢复默认布局
                </DropdownMenu.Item>
              </DropdownMenu.Content>
            </DropdownMenu.Portal>
          </DropdownMenu.Root>
        </div>

        <div className={styles.barCenter} role="group" aria-label="工作区预设">
          {(['browse', 'reading', 'writing', 'freeform'] as WorkspacePresetId[]).map((id) => (
            <button
              key={id}
              type="button"
              className={`${styles.presetBtn} ${presetId === id ? styles.presetBtnActive : ''}`}
              onClick={() => applyPreset(id)}
            >
              {PRESET_LABELS[id]}
            </button>
          ))}
        </div>

        <div className={styles.barRight}>
          <Button variant="ghost" style={{ height: 28 }} onClick={() => openSingletonWindow('search', '全局搜索')}>
            <MagnifyingGlass size={15} />
            搜索
          </Button>
          <Button
            variant="primary"
            style={{ height: 28 }}
            onClick={() => openComposerWindow({ mode: 'newTopic', title: '新建主题草稿' })}
          >
            <NotePencil size={15} />
            新主题
          </Button>
          <IconButton
            label={theme === 'light' ? '切换到深色模式' : '切换到浅色模式'}
            onClick={() => setTheme((t) => (t === 'light' ? 'dark' : 'light'))}
          >
            {theme === 'light' ? <Moon size={16} /> : <Sun size={16} />}
          </IconButton>
        </div>
      </div>

      <div className={styles.mainArea}>
        <nav
          aria-label="社区导航"
          className={`${styles.navRail} ${navCollapsed ? styles.navRailCollapsed : ''}`}
        >
          <div className={styles.navList}>
            <button
              type="button"
              className={`${styles.navItem} ${styles.navItemActive}`}
              onClick={() => openSingletonWindow('topicList', '最新话题')}
            >
              <ListBullets size={16} />
              {!navCollapsed && <span>话题</span>}
            </button>
            <button
              type="button"
              className={styles.navItem}
              onClick={() => openSingletonWindow('search', '全局搜索')}
            >
              <Compass size={16} />
              {!navCollapsed && <span>搜索</span>}
            </button>
            <button
              type="button"
              className={styles.navItem}
              onClick={() => openSingletonWindow('notifications', '通知')}
            >
              <Bell size={16} />
              {!navCollapsed && <span>通知</span>}
            </button>
            <button
              type="button"
              className={styles.navItem}
              onClick={() => openSingletonWindow('designBaseline', '视觉基线')}
            >
              <Palette size={16} />
              {!navCollapsed && <span>视觉基线</span>}
            </button>
          </div>

          <div className={styles.navList}>
            <button type="button" className={styles.navItem} onClick={() => setPaletteOpen(true)}>
              <MagnifyingGlass size={16} />
              {!navCollapsed && <span>命令面板</span>}
            </button>
          </div>
        </nav>

        <main ref={viewportRef} className={styles.workspaceViewport}>
          {dockedWindows.length > 0 && (
            <div className={styles.dockHost}>
              {dockedWindows.map((win) => (
                <div
                  key={win.instanceId}
                  className={`${styles.dockSlot} ${
                    win.dockRegion === 'left' && dockedWindows.length === 1 && floatingWindows.length > 0
                      ? styles.dockSlotSide
                      : ''
                  }`}
                >
                  <WindowFrame
                    win={win}
                    active={activeInstanceId === win.instanceId}
                    siblings={[]}
                    viewport={viewport}
                  >
                    {renderPanelContent(win)}
                  </WindowFrame>
                </div>
              ))}
            </div>
          )}

          <div className={styles.floatingHost}>
            {floatingWindows.map((win) => {
              const siblings = floatingWindows
                .filter((other) => other.instanceId !== win.instanceId)
                .map((other) => other.bounds);
              return (
                <WindowFrame
                  key={win.instanceId}
                  win={win}
                  active={activeInstanceId === win.instanceId}
                  siblings={siblings}
                  viewport={viewport}
                  onSnapGuidesChange={setSnapGuides}
                >
                  {renderPanelContent(win)}
                </WindowFrame>
              );
            })}
          </div>

          {snapGuides.map((g, idx) =>
            g.axis === 'x' ? (
              <div key={`gx-${idx}`} className={styles.snapGuideX} style={{ left: g.pos }} />
            ) : (
              <div key={`gy-${idx}`} className={styles.snapGuideY} style={{ top: g.pos }} />
            ),
          )}
        </main>
      </div>

      <footer className={styles.windowBar} aria-label="窗口栏">
        {windows.map((w) => {
          const isActive = activeInstanceId === w.instanceId && w.viewState !== 'minimized';
          return (
            <button
              key={w.instanceId}
              type="button"
              title={w.title}
              className={`${styles.windowTab} ${isActive ? styles.windowTabActive : ''}`}
              onClick={() => focusWindow(w.instanceId)}
            >
              {typeof w.unreadBadge === 'number' && w.unreadBadge > 0 && (
                <span
                  style={{
                    width: 6,
                    height: 6,
                    borderRadius: 999,
                    background: 'var(--accent-graphic)',
                    flexShrink: 0,
                  }}
                />
              )}
              <span className={styles.windowTabLabel}>{w.title}</span>
              {typeof w.unreadBadge === 'number' && w.unreadBadge > 0 && (
                <span className="tabular-nums">{w.unreadBadge}</span>
              )}
            </button>
          );
        })}
      </footer>

      <div className={styles.statusBar} role="status" aria-live="polite">
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          {network.phase !== 'ready' && <WarningCircle size={14} style={{ color: 'var(--warning)' }} />}
          <span>
            {network.phase === 'ready'
              ? '在线 · 共享请求调度就绪'
              : network.message || '网络受限，仅展示本地缓存'}
          </span>
        </div>
        <span className="tabular-nums">
          窗口 {windows.length} · {viewport.width}×{viewport.height}
        </span>
      </div>

      <Dialog.Root open={paletteOpen} onOpenChange={setPaletteOpen}>
        <Dialog.Portal>
          <Dialog.Overlay className={styles.dialogOverlay} />
          <Dialog.Content className={styles.dialogContent} aria-describedby={undefined}>
            <Dialog.Title style={{ margin: '0 0 8px', fontSize: 14, fontWeight: 600 }}>命令面板</Dialog.Title>
            <input
              autoFocus
              className={uiStyles.input}
              placeholder="输入命令..."
              value={paletteFilter}
              onChange={(e) => setPaletteFilter(e.target.value)}
            />
            <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 2 }}>
              {commands.map((cmd) => (
                <button
                  key={cmd.id}
                  type="button"
                  className={styles.menuItem}
                  style={{ border: 'none', background: 'transparent', width: '100%' }}
                  onClick={() => {
                    cmd.run();
                    setPaletteOpen(false);
                    setPaletteFilter('');
                  }}
                >
                  <span>{cmd.label}</span>
                  {cmd.shortcut && <span className={styles.menuShortcut}>{cmd.shortcut}</span>}
                </button>
              ))}
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  );
}
