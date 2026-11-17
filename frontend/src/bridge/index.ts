import { MockDesktopBridge } from './mockBridge';
import type { DesktopBridge } from './types';

export * from './types';
export { MockDesktopBridge } from './mockBridge';

let activeBridge: DesktopBridge | null = null;

/**
 * Returns the active DesktopBridge. Uses MockDesktopBridge in browser preview/tests
 * until Wails bindings are generated and bound in desktop runtime.
 */
export function getDesktopBridge(): DesktopBridge {
  if (!activeBridge) {
    activeBridge = new MockDesktopBridge();
  }
  return activeBridge;
}

export function setDesktopBridgeForTesting(bridge: DesktopBridge | null): void {
  activeBridge = bridge;
}
