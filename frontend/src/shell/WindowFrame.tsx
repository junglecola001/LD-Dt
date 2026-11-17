import React, { useEffect, useRef, useState } from 'react';
import {
  ArrowsInSimple,
  ArrowsOutSimple,
  Bell,
  Columns,
  FileText,
  ListBullets,
  MagnifyingGlass,
  Minus,
  NotePencil,
  Palette,
  PushPin,
  X,
} from '@phosphor-icons/react';
import { IconButton } from '../components/Primitives';
import { resizeWindowBounds, snapWindowBounds } from '../workspace/geometry';
import { useDraftBufferStore, useWorkspaceStore } from '../workspace/store';
import type { ResizeHandle, SnapGuide, ViewportSize, WindowBounds, WindowState } from '../workspace/types';
import styles from './WindowFrame.module.css';

interface WindowFrameProps {
  win: WindowState;
  active: boolean;
  siblings: WindowBounds[];
  viewport: ViewportSize;
  onSnapGuidesChange?: (guides: SnapGuide[]) => void;
  children: React.ReactNode;
}

function panelIcon(panelType: WindowState['panelType']) {
  const props = { size: 16, weight: 'regular' as const };
  switch (panelType) {
    case 'topicList':
      return <ListBullets {...props} />;
    case 'topic':
      return <FileText {...props} />;
    case 'composer':
      return <NotePencil {...props} />;
    case 'search':
      return <MagnifyingGlass {...props} />;
    case 'notifications':
      return <Bell {...props} />;
    case 'designBaseline':
      return <Palette {...props} />;
    default:
      return <FileText {...props} />;
  }
}

export function WindowFrame({
  win,
  active,
  siblings,
  viewport,
  onSnapGuidesChange,
  children,
}: WindowFrameProps) {
  const focusWindow = useWorkspaceStore((s) => s.focusWindow);
  const setWindowBounds = useWorkspaceStore((s) => s.setWindowBounds);
  const toggleMinimize = useWorkspaceStore((s) => s.toggleMinimize);
  const toggleMaximize = useWorkspaceStore((s) => s.toggleMaximize);
  const togglePinned = useWorkspaceStore((s) => s.togglePinned);
  const setDisplayMode = useWorkspaceStore((s) => s.setDisplayMode);
  const closeWindow = useWorkspaceStore((s) => s.closeWindow);
  const keyboardMoveTargetId = useWorkspaceStore((s) => s.keyboardMoveTargetId);
  const setKeyboardMoveTarget = useWorkspaceStore((s) => s.setKeyboardMoveTarget);
  const nudgeWindow = useWorkspaceStore((s) => s.nudgeWindow);

  const draftBuffer = useDraftBufferStore((s) => (win.draftId ? s.buffers[win.draftId] : undefined));

  const frameRef = useRef<HTMLElement>(null);
  const [liveBounds, setLiveBounds] = useState<WindowBounds | null>(null);
  const isKeyboardMoving = keyboardMoveTargetId === win.instanceId;

  useEffect(() => {
    if (!isKeyboardMoving) return;
    const onKeyDown = (e: KeyboardEvent) => {
      const step = 16;
      const resize = e.shiftKey;
      if (e.key === 'ArrowLeft') {
        e.preventDefault();
        nudgeWindow(win.instanceId, -step, 0, resize);
      } else if (e.key === 'ArrowRight') {
        e.preventDefault();
        nudgeWindow(win.instanceId, step, 0, resize);
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        nudgeWindow(win.instanceId, 0, -step, resize);
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        nudgeWindow(win.instanceId, 0, step, resize);
      } else if (e.key === 'Enter' || e.key === 'Escape') {
        e.preventDefault();
        setKeyboardMoveTarget(null);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [isKeyboardMoving, nudgeWindow, setKeyboardMoveTarget, win.instanceId]);

  const startDrag = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return;
    if ((e.target as HTMLElement).closest('button')) return;
    focusWindow(win.instanceId);
    if (win.displayMode !== 'floating' || win.viewState === 'maximized') return;

    e.preventDefault();
    const originX = e.clientX;
    const originY = e.clientY;
    const initial = { ...win.bounds };
    let latest = initial;
    let rafId = 0;

    const cleanup = () => {
      if (rafId) cancelAnimationFrame(rafId);
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      window.removeEventListener('keydown', onEsc);
      onSnapGuidesChange?.([]);
    };

    const onMove = (ev: PointerEvent) => {
      const dx = ev.clientX - originX;
      const dy = ev.clientY - originY;
      const snapped = snapWindowBounds(
        { ...initial, x: initial.x + dx, y: initial.y + dy },
        siblings,
        viewport,
        ev.altKey,
      );
      latest = snapped.bounds;
      if (!rafId) {
        rafId = requestAnimationFrame(() => {
          rafId = 0;
          setLiveBounds(latest);
          onSnapGuidesChange?.(snapped.guides);
        });
      }
    };

    const onEsc = (ev: KeyboardEvent) => {
      if (ev.key === 'Escape') {
        ev.preventDefault();
        cleanup();
        setLiveBounds(null);
      }
    };

    const onUp = () => {
      cleanup();
      setLiveBounds(null);
      setWindowBounds(win.instanceId, latest);
    };

    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp, { once: true });
    window.addEventListener('keydown', onEsc);
  };

  const startResize = (handle: ResizeHandle) => (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0 || win.displayMode !== 'floating' || win.viewState === 'maximized') return;
    e.preventDefault();
    e.stopPropagation();
    focusWindow(win.instanceId);

    const originX = e.clientX;
    const originY = e.clientY;
    const initial = { ...win.bounds };
    let latest = initial;
    let rafId = 0;

    const cleanup = () => {
      if (rafId) cancelAnimationFrame(rafId);
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      window.removeEventListener('keydown', onEsc);
    };

    const onMove = (ev: PointerEvent) => {
      latest = resizeWindowBounds(initial, ev.clientX - originX, ev.clientY - originY, handle, viewport);
      if (!rafId) {
        rafId = requestAnimationFrame(() => {
          rafId = 0;
          setLiveBounds(latest);
        });
      }
    };

    const onEsc = (ev: KeyboardEvent) => {
      if (ev.key === 'Escape') {
        ev.preventDefault();
        cleanup();
        setLiveBounds(null);
      }
    };

    const onUp = () => {
      cleanup();
      setLiveBounds(null);
      setWindowBounds(win.instanceId, latest);
    };

    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp, { once: true });
    window.addEventListener('keydown', onEsc);
  };

  const b = liveBounds ?? win.bounds;
  const isFloating = win.displayMode === 'floating';
  const isMaximized = win.viewState === 'maximized';

  const floatingStyle: React.CSSProperties | undefined = isFloating
    ? {
        position: 'absolute',
        left: isMaximized ? 8 : b.x,
        top: isMaximized ? 8 : b.y,
        width: isMaximized ? undefined : b.width,
        height: isMaximized ? undefined : b.height,
        zIndex: win.zIndex,
      }
    : undefined;

  const draftLabel = draftBuffer
    ? draftBuffer.localState === 'saveFailed'
      ? '保存失败'
      : draftBuffer.revision > draftBuffer.savedRevision
        ? '未保存'
        : '已保存'
    : null;

  return (
    <section
      ref={frameRef}
      role="region"
      aria-label={win.title}
      data-instance-id={win.instanceId}
      data-active={active ? 'true' : 'false'}
      style={floatingStyle}
      onPointerDown={() => focusWindow(win.instanceId)}
      className={`${styles.frame} ${active ? styles.frameActive : ''} ${
        !isFloating ? styles.frameDocked : ''
      } ${isMaximized && isFloating ? styles.frameMaximized : ''}`}
    >
      <div
        className={styles.titlebar}
        onPointerDown={startDrag}
        onDoubleClick={() => toggleMaximize(win.instanceId)}
      >
        <div className={styles.titleLeft}>
          <span className={styles.titleIcon}>{panelIcon(win.panelType)}</span>
          <span className={styles.titleText} title={win.title}>
            {win.title}
          </span>
          {draftLabel && <span className={styles.draftStatusPill}>{draftLabel}</span>}
        </div>

        <div className={styles.controls}>
          {isFloating && (
            <IconButton
              label={win.pinned ? '取消置顶' : '工作区置顶'}
              onClick={() => togglePinned(win.instanceId)}
            >
              <PushPin size={15} weight={win.pinned ? 'fill' : 'regular'} />
            </IconButton>
          )}
          <IconButton
            label={isFloating ? '停靠到左侧分栏' : '切换为浮动窗口'}
            onClick={() => setDisplayMode(win.instanceId, isFloating ? 'docked' : 'floating', 'left')}
          >
            <Columns size={15} />
          </IconButton>
          <IconButton label="最小化" onClick={() => toggleMinimize(win.instanceId)}>
            <Minus size={15} />
          </IconButton>
          <IconButton
            label={isMaximized ? '还原' : '最大化'}
            onClick={() => toggleMaximize(win.instanceId)}
          >
            {isMaximized ? <ArrowsInSimple size={15} /> : <ArrowsOutSimple size={15} />}
          </IconButton>
          <IconButton label="关闭窗口" danger onClick={() => closeWindow(win.instanceId)}>
            <X size={15} />
          </IconButton>
        </div>
      </div>

      {isKeyboardMoving && (
        <div className={styles.kbHintBar}>
          <span>方向键移动 · Shift+方向键调整大小 · Enter/Esc 完成</span>
          <button type="button" onClick={() => setKeyboardMoveTarget(null)}>
            完成
          </button>
        </div>
      )}

      <div className={styles.body}>{children}</div>

      {isFloating && !isMaximized && (
        <>
          <div className={`${styles.resizeHandle} ${styles.resizeN}`} onPointerDown={startResize({ n: true })} />
          <div className={`${styles.resizeHandle} ${styles.resizeS}`} onPointerDown={startResize({ s: true })} />
          <div className={`${styles.resizeHandle} ${styles.resizeE}`} onPointerDown={startResize({ e: true })} />
          <div className={`${styles.resizeHandle} ${styles.resizeW}`} onPointerDown={startResize({ w: true })} />
          <div className={`${styles.resizeHandle} ${styles.resizeNW}`} onPointerDown={startResize({ n: true, w: true })} />
          <div className={`${styles.resizeHandle} ${styles.resizeNE}`} onPointerDown={startResize({ n: true, e: true })} />
          <div className={`${styles.resizeHandle} ${styles.resizeSW}`} onPointerDown={startResize({ s: true, w: true })} />
          <div className={`${styles.resizeHandle} ${styles.resizeSE}`} onPointerDown={startResize({ s: true, e: true })} />
        </>
      )}
    </section>
  );
}
