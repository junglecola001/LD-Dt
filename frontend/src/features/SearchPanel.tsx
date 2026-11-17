import { useRef, useState } from 'react';
import { MagnifyingGlass } from '@phosphor-icons/react';
import { getDesktopBridge, type SearchHit } from '../bridge';
import { Button, EmptyState } from '../components/Primitives';
import uiStyles from '../components/ui.module.css';
import { useWorkspaceStore } from '../workspace/store';

export function SearchPanel({ instanceId }: { instanceId: string }) {
  const [keyword, setKeyword] = useState('');
  const [recent, setRecent] = useState<string[]>(['Wails', '429', 'ARM64']);
  const [hits, setHits] = useState<SearchHit[] | null>(null);
  const [searching, setSearching] = useState(false);
  const genRef = useRef(0);
  const composingRef = useRef(false);
  const openTopicWindow = useWorkspaceStore((s) => s.openTopicWindow);

  const runSearch = async (q: string) => {
    const trimmed = q.trim();
    if (!trimmed || composingRef.current) return;
    const nextGen = ++genRef.current;
    setSearching(true);
    try {
      const res = await getDesktopBridge().search(instanceId, trimmed, nextGen);
      if (res.queryGeneration === genRef.current) {
        setHits(res.hits);
        setRecent((prev) => [trimmed, ...prev.filter((x) => x !== trimmed)].slice(0, 6));
      }
    } finally {
      if (nextGen === genRef.current) setSearching(false);
    }
  };

  return (
    <div style={{ padding: '16px', display: 'flex', flexDirection: 'column', gap: '12px', height: '100%' }}>
      <form
        style={{ display: 'flex', gap: '8px' }}
        onSubmit={(e) => {
          e.preventDefault();
          void runSearch(keyword);
        }}
      >
        <input
          aria-label="全局搜索关键词"
          className={uiStyles.input}
          placeholder="搜索话题、分类或用户"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          onCompositionStart={() => {
            composingRef.current = true;
          }}
          onCompositionEnd={() => {
            composingRef.current = false;
          }}
        />
        <Button type="submit" variant="primary" disabled={searching}>
          <MagnifyingGlass size={14} />
          搜索
        </Button>
      </form>

      {recent.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '6px' }}>
          {recent.map((item) => (
            <Button
              key={item}
              variant="ghost"
              style={{ height: '26px', fontSize: '12px' }}
              onClick={() => {
                setKeyword(item);
                void runSearch(item);
              }}
            >
              {item}
            </Button>
          ))}
          <Button variant="ghost" style={{ height: '26px', fontSize: '12px' }} onClick={() => setRecent([])}>
            清除记录
          </Button>
        </div>
      )}

      <div style={{ flex: 1, overflowY: 'auto' }}>
        {hits === null && <EmptyState lead="输入关键词开始搜索" />}
        {hits !== null && hits.length === 0 && (
          <EmptyState lead="没有匹配的结果" actionLabel="清空" onAction={() => setHits(null)} />
        )}
        {hits?.map((hit) => (
          <button
            key={`${hit.topicId}-${hit.postId}`}
            type="button"
            style={{
              display: 'block',
              width: '100%',
              textAlign: 'left',
              padding: '10px 8px',
              border: 'none',
              borderBottom: '1px solid var(--line)',
              background: 'transparent',
              cursor: 'pointer',
            }}
            onClick={() => openTopicWindow(hit.topicId, hit.title, { targetPostNumber: hit.postNumber })}
          >
            <div style={{ fontSize: '14px', fontWeight: 500 }}>{hit.title}</div>
            <div style={{ fontSize: '12px', color: 'var(--ink-muted)', marginTop: '2px' }}>{hit.excerpt}</div>
          </button>
        ))}
      </div>
    </div>
  );
}
