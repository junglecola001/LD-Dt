import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import App from './App';
import { MockDesktopBridge, setDesktopBridgeForTesting } from './bridge';
import { ComposerPanel } from './features/ComposerPanel';

describe('AppShell & Composer IME Guard', () => {
  it('renders compact 3-column browse workspace with right notifications strip, nav taxonomy, composer trigger, and shield modal', async () => {
    setDesktopBridgeForTesting(new MockDesktopBridge());
    render(<App />);

    expect(screen.getByText('LD Studio')).toBeInTheDocument();
    expect(screen.getByText('演示数据')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '最新话题' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '主题阅读' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '通知' })).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('#wails')).toBeInTheDocument();
      expect(screen.getByText('在线（演示数据）· 共享请求调度就绪')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /新主题/ }));
    expect(screen.getByRole('region', { name: '新建主题草稿' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /过盾 \/ 连接/ }));
    expect(screen.getByRole('dialog', { name: 'Linux.do 连接与过盾中心' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '打开独立过盾窗口' })).toBeInTheDocument();
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
