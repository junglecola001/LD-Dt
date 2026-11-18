import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getDesktopBridge } from '../bridge';
import { Button, EmptyState } from '../components/Primitives';
import { useWorkspaceStore } from '../workspace/store';

export function NotificationsPanel({ onOpenShield }: { onOpenShield?: () => void }) {
  const bridge = getDesktopBridge();
  const queryClient = useQueryClient();
  const openTopicWindow = useWorkspaceStore((s) => s.openTopicWindow);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['notifications', bridge.isMock ? 'mock' : 'real'],
    queryFn: () => bridge.getNotifications(),
  });

  if (isLoading) return <EmptyState lead="正在同步通知" />;
  if (isError) {
    return (
      <div style={{ padding: '16px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
        <p className="serif-title" style={{ margin: 0, fontSize: '16px' }}>
          连接账号或过盾后同步通知
        </p>
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
          {onOpenShield && (
            <Button variant="primary" onClick={onOpenShield}>
              连接 / 过盾
            </Button>
          )}
          <Button variant="secondary" onClick={() => void refetch()}>
            重试
          </Button>
        </div>
      </div>
    );
  }
  if (!data || !data.notifications || data.notifications.length === 0) {
    return <EmptyState lead="暂时没有未读通知" />;
  }

  const handleOpen = async (id: number, topicId: number, postNumber: number, title: string) => {
    openTopicWindow(topicId, title, { targetPostNumber: postNumber });
    const updated = await bridge.markNotificationVisited(id, true);
    queryClient.setQueryData(['notifications', bridge.isMock ? 'mock' : 'real'], updated);
  };

  const handleMarkAll = async () => {
    const updated = await bridge.markAllNotificationsRead();
    queryClient.setQueryData(['notifications', bridge.isMock ? 'mock' : 'real'], updated);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '6px 12px',
          borderBottom: '1px solid var(--line)',
        }}
      >
        <span className="tabular-nums" style={{ fontSize: '12px', color: 'var(--ink-muted)' }}>
          未读 {data.unreadCount} 条
        </span>
        <Button variant="ghost" style={{ height: 26, fontSize: 12 }} onClick={() => void handleMarkAll()}>
          全部已读
        </Button>
      </div>
      <div style={{ flex: 1, overflowY: 'auto' }}>
        {data.notifications.map((n) => (
          <button
            key={n.id}
            type="button"
            style={{
              display: 'flex',
              alignItems: 'flex-start',
              gap: '8px',
              width: '100%',
              textAlign: 'left',
              padding: '9px 12px',
              border: 'none',
              borderBottom: '1px solid var(--line)',
              background: n.read ? 'transparent' : 'var(--accent-wash)',
              cursor: 'pointer',
            }}
            onClick={() => void handleOpen(n.id, n.topicId, n.postNumber, n.topicTitle)}
          >
            {!n.read && (
              <span
                style={{
                  width: 6,
                  height: 6,
                  marginTop: 6,
                  borderRadius: 999,
                  background: 'var(--accent-graphic)',
                  flexShrink: 0,
                }}
              />
            )}
            <div style={{ minWidth: 0, flex: 1 }}>
              <div style={{ fontSize: '13px', fontWeight: 500, lineHeight: 1.4 }}>{n.topicTitle}</div>
              <div style={{ fontSize: '12px', color: 'var(--ink-muted)', marginTop: 2 }}>
                @{n.actorUsername} · {n.excerpt}
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
