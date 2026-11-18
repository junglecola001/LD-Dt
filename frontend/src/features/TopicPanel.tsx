import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Heart, MagnifyingGlass, NotePencil } from '@phosphor-icons/react';
import { getDesktopBridge } from '../bridge';
import { Button, EmptyState } from '../components/Primitives';
import { sanitizeCookedHTML } from '../utils/sanitize';
import { useWorkspaceStore } from '../workspace/store';
import styles from './TopicPanel.module.css';

export function TopicPanel({
  instanceId,
  topicId,
  targetPostNumber,
  onOpenShield,
}: {
  instanceId: string;
  topicId: number;
  targetPostNumber?: number;
  onOpenShield?: () => void;
}) {
  const [serifReading, setSerifReading] = useState(false);
  const [showFind, setShowFind] = useState(false);
  const [findText, setFindText] = useState('');
  const openComposerWindow = useWorkspaceStore((s) => s.openComposerWindow);
  const queryClient = useQueryClient();
  const bridge = getDesktopBridge();

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['topic', bridge.isMock ? 'mock' : 'real', topicId, targetPostNumber ?? 0],
    queryFn: () => bridge.openTopic(instanceId, topicId, targetPostNumber),
  });

  const posts = useMemo(() => {
    if (!data) return [];
    if (!findText.trim()) return data.detail.posts;
    const q = findText.trim().toLowerCase();
    return data.detail.posts.filter(
      (p) => p.cookedHtml.toLowerCase().includes(q) || p.username.toLowerCase().includes(q),
    );
  }, [data, findText]);

  if (isLoading) return <EmptyState lead="正在打开主题" />;
  if (isError || !data) {
    return (
      <div style={{ padding: '24px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
        <p className="serif-title" style={{ margin: 0, fontSize: '20px' }}>
          主题内容暂时无法打开
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
    );
  }

  const { detail, anchor } = data;

  const handleLike = async (postId: number) => {
    const updated = await bridge.toggleLike(postId);
    queryClient.setQueryData(['topic', topicId, targetPostNumber ?? 0], (prev: typeof data) => {
      if (!prev) return prev;
      return {
        ...prev,
        detail: {
          ...prev.detail,
          posts: prev.detail.posts.map((p) =>
            p.id === updated.postId
              ? { ...p, likedByMe: updated.likedByMe, likeCount: updated.likeCount }
              : p,
          ),
        },
      };
    });
  };

  return (
    <div className={styles.container}>
      <div className={styles.inner}>
        <header className={styles.header}>
          <h1 className={styles.topicTitle}>{detail.summary.title}</h1>
          <div className={styles.headerMeta}>
            <span className="tabular-nums">
              {detail.summary.categoryName} · 阅读锚点 #{anchor.postNumber}
            </span>
            <div className={styles.actions}>
              <Button variant="ghost" onClick={() => setShowFind((v) => !v)}>
                <MagnifyingGlass size={14} />
                页内查找
              </Button>
              <Button variant="ghost" onClick={() => setSerifReading((v) => !v)}>
                {serifReading ? '无衬线正文' : '衬线阅读'}
              </Button>
              <Button
                variant="primary"
                onClick={() =>
                  openComposerWindow({
                    mode: 'replyTopic',
                    topicId: detail.summary.id,
                    title: `回复 · #${detail.summary.id}`,
                  })
                }
              >
                <NotePencil size={14} />
                回复
              </Button>
            </div>
          </div>
        </header>

        {showFind && (
          <div className={styles.findBar}>
            <MagnifyingGlass size={14} />
            <input
              aria-label="页内查找"
              className={styles.findInput}
              placeholder="只筛选当前窗口已加载楼层..."
              value={findText}
              onChange={(e) => setFindText(e.target.value)}
            />
          </div>
        )}

        {posts.map((post) => (
          <article key={post.id} className={styles.post}>
            <div className={styles.postHeader}>
              <span className={styles.postAuthor}>
                {post.displayName || post.username} <span style={{ color: 'var(--ink-muted)' }}>@{post.username}</span>
              </span>
              <span className="tabular-nums">#{post.postNumber}</span>
            </div>
            <div
              className="prose"
              data-serif-reading={serifReading ? 'true' : 'false'}
              dangerouslySetInnerHTML={{ __html: sanitizeCookedHTML(post.cookedHtml) }}
            />
            <div className={styles.postActions}>
              <Button variant="ghost" onClick={() => void handleLike(post.id)}>
                <Heart size={14} weight={post.likedByMe ? 'fill' : 'regular'} />
                <span className="tabular-nums">{post.likeCount}</span>
              </Button>
              {post.canEdit && (
                <Button
                  variant="ghost"
                  onClick={() =>
                    openComposerWindow({
                      draftId: `edit-post-${post.id}`,
                      mode: 'editPost',
                      topicId: detail.summary.id,
                      title: `编辑 #${post.postNumber}`,
                    })
                  }
                >
                  编辑
                </Button>
              )}
            </div>
          </article>
        ))}
      </div>
    </div>
  );
}
