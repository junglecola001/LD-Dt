import { MockDesktopBridge } from './mockBridge';
import type { DesktopBridge } from './types';
import { WailsDesktopBridge } from './wailsBridge';

export * from './types';
export { MockDesktopBridge } from './mockBridge';
export { WailsDesktopBridge } from './wailsBridge';

let activeBridge: DesktopBridge | null = null;
let overrideForTesting: DesktopBridge | null = null;
let forceDemoMode = false;

export function isWailsDesktopRuntime(): boolean {
  if (typeof window === 'undefined') return false;
  const win = window as unknown as { _wails?: unknown };
  return (
    Boolean(win._wails) ||
    window.location.protocol === 'wails:' ||
    window.location.hostname.endsWith('wails.localhost')
  );
}

/**
 * Returns the active DesktopBridge:
 * - In Wails desktop runtime, defaults to real Linux.do requests via Go RequestCoordinator.
 * - In browser preview / Vitest (or when user explicitly enables Demo Mode), uses MockDesktopBridge.
 */
export function getDesktopBridge(): DesktopBridge {
  if (overrideForTesting) {
    return overrideForTesting;
  }
  if (forceDemoMode || !isWailsDesktopRuntime()) {
    if (!activeBridge || !activeBridge.isMock) {
      activeBridge = new MockDesktopBridge();
    }
    return activeBridge;
  }
  if (!activeBridge || activeBridge.isMock) {
    activeBridge = new WailsDesktopBridge();
  }
  return activeBridge;
}

export function setDemoModeEnabled(enabled: boolean): DesktopBridge {
  forceDemoMode = enabled;
  activeBridge = null;
  return getDesktopBridge();
}

export function setDesktopBridgeForTesting(bridge: DesktopBridge | null): void {
  overrideForTesting = bridge;
}
