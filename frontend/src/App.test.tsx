import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import App from './App';
import { MockDesktopBridge, setDesktopBridgeForTesting } from './bridge';
import { ComposerPanel } from './features/ComposerPanel';

describe('AppShell & Composer IME Guard', () => {
  it('renders Phase 0 browse workspace with demo badge, topic window, composer window, and window bar', async () => {
    setDesktopBridgeForTesting(new MockDesktopBridge());
    render(<App />);

    expect(screen.getByText('LD Studio')).toBeInTheDocument();
    expect(screen.getByText('演示数据')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: /欢迎来到 LD Studio/ })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: /回复草稿/ })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole('status')).toHaveTextContent('在线');
    });
  });

  it('does not submit on Ctrl+Enter while IME composition is active', async () => {
    const mock = new MockDesktopBridge();
    const submitSpy = vi.spyOn(mock, 'submitDraft');
    setDesktopBridgeForTesting(mock);

    render(<ComposerPanel draftId="draft-welcome" />);
    const textarea = screen.getByLabelText('Markdown 正文');

    fireEvent.compositionStart(textarea);
    fireEvent.keyDown(textarea, { key: 'Enter', ctrlKey: true });
    expect(submitSpy).not.toHaveBeenCalled();

    fireEvent.compositionEnd(textarea);
    fireEvent.keyDown(textarea, { key: 'Enter', ctrlKey: true });
    await waitFor(() => {
      expect(submitSpy).toHaveBeenCalledTimes(1);
    });
  });
});
