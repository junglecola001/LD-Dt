import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowClockwise, Plus, X } from '@phosphor-icons/react';
import { getDesktopBridge } from '../bridge';
import { Button, EmptyState, IconButton } from '../components/Primitives';
import { useWorkspaceStore } from '../workspace/store';
import styles from './TopicListPanel.module.css';

interface TopicListPanelProps {
  instanceId: string;
  onOpenShield?: () => void;
}

export function TopicListPanel({ instanceId, onOpenShield }: TopicListPanelProps) {
  const [density, setDensity] = useState<'comfortable' | 'compact'>('comfortable');
  const openTopicWindow = useWorkspaceStore((s) => s.openTopicWindow);
  const feedFilter = useWorkspaceStore((s) => s.feedFilter);
  const setFeedFilter = useWorkspaceStore((s) => s.setFeedFilter);

  const bridge = getDesktopBridge();
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: [
      'feed',
      bridge.isMock ? 'mock' : 'real',
      feedFilter.filter,
      feedFilter.categoryId ?? 0,
      feedFilter.tag ?? '',
    ],
    queryFn: () =>
      bridge.loadFeed(instanceId, {
        filter: feedFilter.filter,
        categoryId: feedFilter.categoryId,
        categorySlug: feedFilter.categorySlug,
        tag: feedFilter.tag,
        page: 0,
        queryGeneration: 1,
      }),
  });

  const activeSubMode = feedFilter.filter === 'unread' || feedFilter.filter === 'top' ? feedFilter.filter : 'latest';

  return (
    <div className={styles.container}>
      <div className={styles.toolbar}>
        <div className={styles.filterGroup} role="tablist" aria-label="话题筛选">
          {(['latest', 'unread', 'top'] as const).map((f) => (
            <button
              key={f}
              type="button"
              role="tab"
              aria-selected={activeSubMode === f && !feedFilter.categoryId && !feedFilter.tag}
              className={`${styles.filterTab} ${
                activeSubMode === f && !feedFilter.categoryId && !feedFilter.tag ? styles.filterTabActive : ''
              }`}
              onClick={() => setFeedFilter({ filter: f })}
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

      {(feedFilter.categoryName || feedFilter.tag) && (
        <div className={styles.activeChipBar}>
          <span>
            {feedFilter.categoryName ? `分类：${feedFilter.categoryName}` : `标签：#${feedFilter.tag}`}
          </span>
          <IconButton label="清除筛选" onClick={() => setFeedFilter({ filter: 'latest' })}>
            <X size={13} />
          </IconButton>
        </div>
      )}

      <div className={styles.list}>
        {isLoading && <EmptyState lead="正在加载话题" />}
        {isError && (
          <div style={{ padding: '20px 16px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
            <p className="serif-title" style={{ margin: 0, fontSize: '17px' }}>
              列表请求受限或暂不可达
            </p>
            <div style={{ display: 'flex', gap: '8px' }}>
              {onOpenShield && (
                <Button variant="primary" onClick={onOpenShield}>
                  打开过盾界面
                </Button>
              )}
              <Button variant="secondary" onClick={() => void refetch()}>
                重试
              </Button>
            </div>
          </div>
        )}
        {!isLoading && !isError && data && data.topics.length === 0 && (
          <EmptyState lead="当前筛选没有话题" actionLabel="回到最新" onAction={() => setFeedFilter({ filter: 'latest' })} />
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
                  className={styles.rowMainBtn}
                  onClick={() => openTopicWindow(topic.id, topic.title)}
                >
                  <p className={styles.rowTitle} title={topic.title}>
                    {topic.title}
                  </p>
                  <div className={styles.rowMeta}>
                    {topic.categoryName && (
                      <>
                        <span className={styles.categoryLabel}>{topic.categoryName}</span>
                        <span>·</span>
                      </>
                    )}
                    <span className={styles.authorLabel} title={topic.authorUsername}>
                      {topic.authorUsername}
                    </span>
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
