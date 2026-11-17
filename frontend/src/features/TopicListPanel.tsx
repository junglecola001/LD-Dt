import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowClockwise, Plus } from '@phosphor-icons/react';
import { getDesktopBridge } from '../bridge';
import { EmptyState, IconButton } from '../components/Primitives';
import { useWorkspaceStore } from '../workspace/store';
import styles from './TopicListPanel.module.css';

export function TopicListPanel({ instanceId }: { instanceId: string }) {
  const [filter, setFilter] = useState<'latest' | 'unread' | 'top'>('latest');
  const [density, setDensity] = useState<'comfortable' | 'compact'>('comfortable');
  const openTopicWindow = useWorkspaceStore((s) => s.openTopicWindow);

  const bridge = getDesktopBridge();
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['feed', filter],
    queryFn: () => bridge.loadFeed(instanceId, { filter, page: 0, queryGeneration: 1 }),
  });

  return (
    <div className={styles.container}>
      <div className={styles.toolbar}>
        <div className={styles.filterGroup} role="tablist" aria-label="话题筛选">
          {(['latest', 'unread', 'top'] as const).map((f) => (
            <button
              key={f}
              type="button"
              role="tab"
              aria-selected={filter === f}
              className={`${styles.filterTab} ${filter === f ? styles.filterTabActive : ''}`}
              onClick={() => setFilter(f)}
            >
              {f === 'latest' ? '最新' : f === 'unread' ? '未读' : '热门'}
            </button>
          ))}
        </div>
        <div className={styles.filterGroup}>
          <button
            type="button"
            className={styles.filterTab}
            onClick={() => setDensity((d) => (d === 'comfortable' ? 'compact' : 'comfortable'))}
          >
            {density === 'comfortable' ? '紧凑' : '舒适'}
          </button>
          <IconButton label="刷新列表" onClick={() => void refetch()}>
            <ArrowClockwise size={15} />
          </IconButton>
        </div>
      </div>

      <div className={styles.list}>
        {isLoading && <EmptyState lead="正在加载话题" />}
        {isError && <EmptyState lead="列表暂时无法读取" actionLabel="重试" onAction={() => void refetch()} />}
        {!isLoading && !isError && data && data.topics.length === 0 && (
          <EmptyState lead="当前筛选没有话题" actionLabel="回到最新" onAction={() => setFilter('latest')} />
        )}
        {!isLoading &&
          !isError &&
          data?.topics.map((topic) => {
            const hasUnread = topic.unreadPosts > 0 || topic.newPosts > 0;
            return (
              <div
                key={topic.id}
                className={`${styles.row} ${hasUnread ? styles.rowUnread : ''}`}
                data-density={density}
              >
                <button
                  type="button"
                  style={{ all: 'unset', cursor: 'pointer', flex: 1, minWidth: 0 }}
                  onClick={() => openTopicWindow(topic.id, topic.title)}
                >
                  <div className={styles.rowMain}>
                    <p className={styles.rowTitle} title={topic.title}>
                      {topic.title}
                    </p>
                    <div className={styles.rowMeta}>
                      <span>{topic.categoryName}</span>
                      <span>·</span>
                      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {topic.authorUsername}
                      </span>
                    </div>
                  </div>
                </button>
                <div className={styles.rowBadges}>
                  {hasUnread && <span className={styles.unreadDot} aria-hidden="true" />}
                  <span className={`${styles.countPill} tabular-nums`}>
                    {topic.unreadPosts > 0 ? `+${topic.unreadPosts}` : topic.postsCount}
                  </span>
                  <IconButton
                    label="在新窗口打开"
                    onClick={() => openTopicWindow(topic.id, topic.title, { newWindow: true })}
                  >
                    <Plus size={14} />
                  </IconButton>
                </div>
              </div>
            );
          })}
      </div>
    </div>
  );
}
