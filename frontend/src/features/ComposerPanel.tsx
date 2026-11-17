import React, { useEffect, useRef, useState } from 'react';
import { FloppyDisk, PaperPlaneRight } from '@phosphor-icons/react';
import { getDesktopBridge } from '../bridge';
import { Button } from '../components/Primitives';
import { useDraftBufferStore } from '../workspace/store';
import styles from './ComposerPanel.module.css';

export function ComposerPanel({ draftId }: { draftId: string }) {
  const buffer = useDraftBufferStore((s) => s.buffers[draftId]);
  const ensureBuffer = useDraftBufferStore((s) => s.ensureBuffer);
  const updateBody = useDraftBufferStore((s) => s.updateBody);
  const updateTitle = useDraftBufferStore((s) => s.updateTitle);
  const markSaved = useDraftBufferStore((s) => s.markSaved);
  const markSaveFailed = useDraftBufferStore((s) => s.markSaveFailed);
  const setSubmitState = useDraftBufferStore((s) => s.setSubmitState);

  const isComposingRef = useRef(false);
  const [statusNote, setStatusNote] = useState<string | null>(null);
  const bridge = getDesktopBridge();

  useEffect(() => {
    if (!buffer) {
      ensureBuffer(draftId, { mode: 'newTopic', title: '' });
    }
  }, [buffer, draftId, ensureBuffer]);

  const saveDraftNow = async () => {
    const latest = useDraftBufferStore.getState().buffers[draftId];
    if (!latest) return;
    try {
      const saved = await bridge.saveLocalDraft({
        draftId: latest.draftId,
        mode: latest.mode,
        topicId: latest.topicId,
        title: latest.title,
        body: latest.body,
        revision: latest.revision,
        localState: 'saving',
        cloudState: 'disabled',
        updatedAt: new Date().toISOString(),
      });
      markSaved(draftId, saved.revision);
    } catch (err) {
      markSaveFailed(draftId, err instanceof Error ? err.message : '保存失败');
    }
  };

  // Auto-save after 500ms idle when editing (plan.md 3.8)
  useEffect(() => {
    if (!buffer || buffer.revision === buffer.savedRevision) return;
    const timer = window.setTimeout(() => {
      void saveDraftNow();
    }, 500);
    return () => window.clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buffer?.revision, buffer?.savedRevision, draftId]);

  if (!buffer) return null;

  const handleSubmit = async () => {
    if (isComposingRef.current || buffer.submitState === 'submitting') return;
    setSubmitState(draftId, 'submitting');
    await saveDraftNow();
    const latest = useDraftBufferStore.getState().buffers[draftId];
    if (!latest) return;

    const op = await bridge.submitDraft(draftId, latest.revision);
    setSubmitState(draftId, op.state, op.errorMessage);
    if (op.state === 'published') {
      setStatusNote('已发布（演示）');
    } else if (op.state === 'enqueued') {
      setStatusNote('已提交，等待审核');
    } else if (op.state === 'outcomeUnknown') {
      setStatusNote('结果待确认，请核对原站后决定');
    } else if (op.state === 'rejected') {
      setStatusNote(op.errorMessage ?? '提交未通过，草稿已保留');
    }
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.nativeEvent.isComposing || isComposingRef.current) return;
    if (e.ctrlKey && e.key === 'Enter') {
      e.preventDefault();
      void handleSubmit();
    } else if (e.ctrlKey && e.key.toLowerCase() === 's') {
      e.preventDefault();
      void saveDraftNow();
    }
  };

  return (
    <div className={styles.container}>
      <div className={styles.inner}>
        {buffer.mode === 'newTopic' && (
          <input
            aria-label="主题标题"
            className={styles.titleInput}
            placeholder="输入主题标题"
            value={buffer.title}
            onChange={(e) => updateTitle(draftId, e.target.value)}
          />
        )}

        <div className={styles.surface}>
          <textarea
            aria-label="Markdown 正文"
            className={styles.textarea}
            placeholder="输入 Markdown 内容（Ctrl+Enter 发送，IME 组合输入期间不触发）"
            value={buffer.body}
            onChange={(e) => updateBody(draftId, e.target.value)}
            onCompositionStart={() => {
              isComposingRef.current = true;
            }}
            onCompositionEnd={() => {
              isComposingRef.current = false;
            }}
            onKeyDown={onKeyDown}
          />
          <div className={styles.footerBar}>
            <div className={styles.ghostTools}>
              <Button variant="ghost" onClick={() => void saveDraftNow()}>
                <FloppyDisk size={14} />
                {buffer.localState === 'saveFailed'
                  ? '保存失败'
                  : buffer.revision > buffer.savedRevision
                    ? '未保存'
                    : `已保存 · rev ${buffer.savedRevision}`}
              </Button>
            </div>
            <button
              type="button"
              aria-label="发布内容"
              title="发布（Ctrl+Enter）"
              className={styles.sendBtn}
              disabled={!buffer.body.trim() || buffer.submitState === 'submitting'}
              onClick={() => void handleSubmit()}
            >
              <PaperPlaneRight size={15} weight="fill" />
            </button>
          </div>
        </div>

        {statusNote && (
          <div className={styles.outcomeBanner} role="status">
            {statusNote}
          </div>
        )}
      </div>
    </div>
  );
}
