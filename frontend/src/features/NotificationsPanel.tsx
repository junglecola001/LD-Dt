import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getDesktopBridge } from '../bridge';
import { Button, EmptyState } from '../components/Primitives';
import { useWorkspaceStore } from '../workspace/store';

export function NotificationsPanel() {
  const bridge = getDesktopBridge();
  const queryClient = useQueryClient();
  const openTopicWindow = useWorkspaceStore((s) => s.openTopicWindow);

  const { data, isLoading } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => bridge.getNotifications(),
  });

  if (isLoading) return <EmptyState lead="正在同步通知" />;
  if (!data || data.notifications.length === 0) {
    return <EmptyState lead="暂时没有未读通知" />;
  }

  const handleOpen = async (id: number, topicId: number, postNumber: number, title: string) => {
    openTopicWindow(topicId, title, { targetPostNumber: postNumber });
    const updated = await bridge.markNotificationVisited(id, true);
    queryClient.setQueryData(['notifications'], updated);
  };

  const handleMarkAll = async () => {
    const updated = await bridge.markAllNotificationsRead();
    queryClient.setQueryData(['notifications'], updated);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '8px 12px',
          borderBottom: '1px solid var(--line)',
        }}
      >
        <span className="tabular-nums" style={{ fontSize: '12px', color: 'var(--ink-muted)' }}>
          未读 {data.unreadCount} 条
        </span>
        <Button variant="ghost" onClick={() => void handleMarkAll()}>
          全部标为已读
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
              padding: '10px 12px',
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
              <div style={{ fontSize: '13px', fontWeight: 500 }}>{n.topicTitle}</div>
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
