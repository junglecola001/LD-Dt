import { useEffect, useState } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import {
  ArrowSquareOut,
  CheckCircle,
  Key,
  ShieldCheck,
  ShieldWarning,
  SignIn,
  X,
} from '@phosphor-icons/react';
import type { NetworkStatusSnapshot, SiteSession } from '../bridge';
import { getDesktopBridge, isWailsDesktopRuntime } from '../bridge';
import { Button, IconButton } from '../components/Primitives';
import uiStyles from '../components/ui.module.css';
import styles from './ShieldModal.module.css';

interface ShieldModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  network: NetworkStatusSnapshot;
  session: SiteSession;
  demoMode: boolean;
  onToggleDemoMode: () => void;
  onRecovered: (nextSess: SiteSession, nextNet: NetworkStatusSnapshot) => void;
}

export function ShieldModal({
  open,
  onOpenChange,
  network,
  session,
  demoMode,
  onToggleDemoMode,
  onRecovered,
}: ShieldModalProps) {
  const [showManual, setShowManual] = useState(false);
  const [cookieHeader, setCookieHeader] = useState('');
  const [userApiKey, setUserApiKey] = useState('');
  const [userAgent, setUserAgent] = useState('');
  const [busy, setBusy] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [countdown, setCountdown] = useState(network.retryAfterSeconds || 0);

  useEffect(() => {
    setCountdown(network.retryAfterSeconds || 0);
  }, [network.retryAfterSeconds, network.phase]);

  useEffect(() => {
    if (countdown <= 0) return;
    const id = window.setInterval(() => {
      setCountdown((c) => (c > 0 ? c - 1 : 0));
    }, 1000);
    return () => window.clearInterval(id);
  }, [countdown]);

  const bridge = getDesktopBridge();
  const inDesktop = isWailsDesktopRuntime();

  const phaseTitle = (() => {
    if (demoMode || bridge.isMock) return '演示数据模式';
    switch (network.phase) {
      case 'challenge_required':
        return '触发 Cloudflare 人机验证（403 / 429）';
      case 'challenge_paused':
        return '验证后仍受限，已暂停自动试探';
      case 'cooling_down':
        return countdown > 0 ? `429 限流冷却中（剩余 ${countdown} 秒）` : '429 限流冷却已结束，可探测恢复';
      case 'auth_expired':
        return '登录状态已失效，需要重新连接';
      case 'offline':
        return '网络连接不可用';
      default:
        return session.status === 'connected' ? `已连接 @${session.username}` : '匿名连接 Linux.do 就绪';
    }
  })();

  const handleOpenBuiltInShield = async (mode: 'challenge' | 'login') => {
    setFeedback(null);
    try {
      await bridge.openShieldWindow(mode);
      setFeedback(
        mode === 'challenge'
          ? '已打开独立 Edge 过盾窗口：通过 Cloudflare 验证后软件会自动同步 Cookie 并切换至 Edge 网络栈联动；若未自动刷新请点下方“单次探测恢复”。'
          : '已打开独立 Edge 登录窗口：在窗口中登录 Linux.do 后，软件会自动通过本地 CDP 同步登录态与 @用户名（保持窗口打开或最小化可免 TLS 拦截；也可随时点击“单次探测恢复”手动触发同步）。',
      );
    } catch (err) {
      setFeedback(err instanceof Error ? err.message : '无法打开独立验证窗口');
    }
  };

  const handleProbe = async () => {
    setBusy(true);
    setFeedback(null);
    try {
      const res = await bridge.probeRecovery();
      onRecovered(res.session, res.network);
      if (res.network.phase === 'ready') {
        setFeedback(
          res.session.status === 'connected'
            ? `已同步登录 @${res.session.username}，真实数据已恢复`
            : '站点探测通过，已恢复匿名真实数据读取（如需登录账号请点击“独立登录窗口”）',
        );
      } else {
        setFeedback(res.network.message || `当前状态：${res.network.phase}`);
      }
    } catch (err) {
      setFeedback(err instanceof Error ? err.message : '探测失败');
    } finally {
      setBusy(false);
    }
  };

  const handleApplyManual = async () => {
    if (!cookieHeader.trim() && !userApiKey.trim()) {
      setFeedback('请先填写 Cookie（如 cf_clearance / _t）或 User-Api-Key');
      return;
    }
    setBusy(true);
    setFeedback(null);
    try {
      const res = await bridge.applyCredentials({
        cookieHeader: cookieHeader.trim(),
        userApiKey: userApiKey.trim(),
        userAgent: userAgent.trim(),
      });
      onRecovered(res.session, res.network);
      if (res.network.phase === 'ready') {
        setFeedback('凭证已同步并探测成功');
        setShowManual(false);
      } else {
        setFeedback(res.network.message || '凭证已保存，但站点仍返回拦截，请确认 User-Agent 与浏览器一致');
      }
    } catch (err) {
      setFeedback(err instanceof Error ? err.message : '保存或探测失败');
    } finally {
      setBusy(false);
    }
  };

  const isWarning = network.phase !== 'ready' && !bridge.isMock;

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className={styles.overlay} />
        <Dialog.Content className={styles.content} aria-describedby={undefined}>
          <div className={styles.header}>
            <Dialog.Title className={styles.title}>Linux.do 连接与过盾中心</Dialog.Title>
            <IconButton label="关闭" onClick={() => onOpenChange(false)}>
              <X size={16} />
            </IconButton>
          </div>

          <div className={styles.statusCard}>
            <div>
              <div className={styles.statusLead}>{phaseTitle}</div>
              <div className={styles.statusMeta}>
                {network.message || (inDesktop ? '共享 RequestCoordinator 限流保护生效中（2 并发 / 750ms 间隔）' : '当前处于浏览器前端预览环境')}
              </div>
            </div>
            {isWarning ? (
              <ShieldWarning size={22} style={{ color: 'var(--warning)', flexShrink: 0 }} />
            ) : (
              <ShieldCheck size={22} style={{ color: 'var(--success)', flexShrink: 0 }} />
            )}
          </div>

          <div className={styles.actionRow}>
            <Button variant="primary" onClick={() => void handleOpenBuiltInShield('challenge')} disabled={!inDesktop || busy}>
              <ShieldCheck size={15} />
              打开独立过盾窗口
            </Button>
            <Button variant="secondary" onClick={() => void handleOpenBuiltInShield('login')} disabled={!inDesktop || busy}>
              <SignIn size={15} />
              独立登录窗口
            </Button>
            <Button
              variant="secondary"
              onClick={() => void bridge.openExternalUrl('https://linux.do/challenge')}
            >
              <ArrowSquareOut size={15} />
              系统浏览器过盾
            </Button>
            <Button variant="secondary" onClick={() => void handleProbe()} disabled={busy || countdown > 0}>
              <CheckCircle size={15} />
              {countdown > 0 ? `冷却中 (${countdown}s)` : '单次探测恢复'}
            </Button>
          </div>

          <div className={styles.actionRow}>
            <Button variant="ghost" onClick={() => setShowManual((v) => !v)}>
              <Key size={15} />
              {showManual ? '收起 Cookie / API Key 导入' : '导入浏览器 Cookie / User-Api-Key'}
            </Button>
          </div>

          {showManual && (
            <div className={styles.manualSection}>
              <label className={styles.fieldLabel} htmlFor="ld-cookie-input">
                浏览器 Cookie（例如 cf_clearance=...; _t=...; _forum_session=...）
              </label>
              <textarea
                id="ld-cookie-input"
                className={styles.textarea}
                placeholder="粘贴请求头中的 Cookie 值..."
                value={cookieHeader}
                onChange={(e) => setCookieHeader(e.target.value)}
              />
              <label className={styles.fieldLabel} htmlFor="ld-apikey-input">
                Discourse User-Api-Key（可选）
              </label>
              <input
                id="ld-apikey-input"
                className={uiStyles.input}
                placeholder="可选：User-Api-Key"
                value={userApiKey}
                onChange={(e) => setUserApiKey(e.target.value)}
              />
              <label className={styles.fieldLabel} htmlFor="ld-ua-input">
                浏览器 User-Agent（与获取 cf_clearance 的浏览器保持一致）
              </label>
              <input
                id="ld-ua-input"
                className={uiStyles.input}
                placeholder="留空则使用默认 Edge UA"
                value={userAgent}
                onChange={(e) => setUserAgent(e.target.value)}
              />
              <div>
                <Button variant="primary" onClick={() => void handleApplyManual()} disabled={busy}>
                  保存并探测连接
                </Button>
              </div>
            </div>
          )}

          {feedback && (
            <div className={styles.feedback} role="status">
              {feedback}
            </div>
          )}

          <div className={styles.footerBar}>
            <Button variant="ghost" onClick={onToggleDemoMode}>
              {demoMode ? '切回真实 Linux.do 数据' : '切换为演示数据（Mock）'}
            </Button>
            <Button variant="secondary" onClick={() => onOpenChange(false)}>
              完成
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
