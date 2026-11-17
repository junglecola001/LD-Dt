import type {
  DesktopBridge,
  Draft,
  FeedPage,
  FeedQuery,
  NetworkPhase,
  NetworkStatusSnapshot,
  NotificationSnapshot,
  ReadAnchor,
  SearchResult,
  SiteSession,
  SubmitOperation,
  TopicDetail,
  TopicSummary,
} from './types';

const LONG_CJK_TITLE =
  '关于在 Windows 桌面端构建多文档工作区（DAW 风格可重叠窗口、停靠分屏、高 DPI 缩放、中文输入法候选词保护、429 共享限流冷却与离线草稿防丢失机制）的长标题边界压力测试与排版对齐验证样本——两百字中文标题溢出与窗口标题栏截断检查';

const DEMO_TOPICS: TopicSummary[] = [
  {
    id: 1001,
    title: '欢迎来到 LD Studio：Go + Wails + React 桌面工作台样板',
    slug: 'welcome-ld-studio',
    categoryId: 1,
    categoryName: '开发调优',
    authorUsername: 'ld_architect',
    postsCount: 4,
    replyCount: 3,
    views: 428,
    likeCount: 31,
    unreadPosts: 2,
    newPosts: 0,
    highestPostNumber: 4,
    lastReadPostNumber: 2,
    pinned: true,
    closed: false,
    archived: false,
    notificationLevel: 2,
    lastPostedAt: '2026-11-17T12:15:00Z',
  },
  {
    id: 1002,
    title: LONG_CJK_TITLE,
    slug: 'long-title-stress-test',
    categoryId: 2,
    categoryName: '界面设计',
    authorUsername: 'typography_reviewer_with_an_extraordinarily_long_username_for_overflow_testing',
    postsCount: 3,
    replyCount: 2,
    views: 214,
    likeCount: 19,
    unreadPosts: 0,
    newPosts: 1,
    highestPostNumber: 3,
    lastReadPostNumber: 0,
    pinned: false,
    closed: false,
    archived: false,
    notificationLevel: 1,
    lastPostedAt: '2026-11-17T11:40:00Z',
  },
  {
    id: 1003,
    title: 'RequestCoordinator 共享 429 冷却与 Cloudflare 验证边界讨论',
    slug: 'request-coordinator-429',
    categoryId: 1,
    categoryName: '开发调优',
    authorUsername: 'net_kernel',
    postsCount: 8,
    replyCount: 7,
    views: 812,
    likeCount: 54,
    unreadPosts: 3,
    newPosts: 0,
    highestPostNumber: 8,
    lastReadPostNumber: 5,
    pinned: false,
    closed: false,
    archived: false,
    notificationLevel: 3,
    lastPostedAt: '2026-11-17T10:05:00Z',
  },
  {
    id: 1004,
    title: 'Windows 11 ARM64 与 125% DPI 缩放下的 WebView2 字体渲染记录',
    slug: 'windows-arm64-dpi',
    categoryId: 3,
    categoryName: '搞七捻三',
    authorUsername: 'surface_tester',
    postsCount: 5,
    replyCount: 4,
    views: 319,
    likeCount: 14,
    unreadPosts: 0,
    newPosts: 0,
    highestPostNumber: 5,
    lastReadPostNumber: 5,
    pinned: false,
    closed: false,
    archived: false,
    notificationLevel: 1,
    lastPostedAt: '2026-11-17T08:30:00Z',
  },
];

const DEMO_TOPIC_DETAILS: Record<number, TopicDetail> = {
  1001: {
    summary: DEMO_TOPICS[0],
    canReply: true,
    canCreatePost: true,
    streamPostIds: [1, 2, 3, 4],
    fetchedAt: '2026-11-17T12:15:00Z',
    stale: false,
    posts: [
      {
        id: 1,
        topicId: 1001,
        postNumber: 1,
        username: 'ld_architect',
        displayName: 'LD 架构组',
        version: 1,
        likeCount: 18,
        likedByMe: false,
        bookmarked: true,
        canEdit: false,
        createdAt: '2026-11-17T09:00:00Z',
        cookedHtml: `
          <p>当前界面运行在 <strong>Mock DesktopBridge（演示数据）</strong> 上，用于 Phase 0 验证暖纸色排版、DAW 内部可重叠窗口与草稿恢复规则。</p>
          <blockquote><p>未保存草稿不能被静默关闭；浮动、停靠与最小化只改变视图状态，不破坏编辑缓冲。</p></blockquote>
          <pre><code>// 示例：RequestCoordinator 同资源 GET 去重
const detail = await bridge.openTopic('topic-1001', 1001);</code></pre>
        `,
      },
      {
        id: 2,
        topicId: 1001,
        postNumber: 2,
        username: 'editor_lead',
        displayName: '编辑体验',
        version: 1,
        likeCount: 9,
        likedByMe: true,
        bookmarked: false,
        canEdit: true,
        rawMarkdown: '支持中文 IME 组合输入保护：`Ctrl+Enter` 在候选词阶段不会误发。',
        createdAt: '2026-11-17T10:20:00Z',
        cookedHtml: `<p>支持中文 IME 组合输入保护：<code>Ctrl+Enter</code> 在候选词阶段不会误发。</p>`,
      },
    ],
  },
  1002: {
    summary: DEMO_TOPICS[1],
    canReply: true,
    canCreatePost: true,
    streamPostIds: [201, 202, 203],
    fetchedAt: '2026-11-17T11:40:00Z',
    stale: false,
    posts: [
      {
        id: 201,
        topicId: 1002,
        postNumber: 1,
        username: 'typography_reviewer_with_an_extraordinarily_long_username_for_overflow_testing',
        displayName: '超长用户名与表格压力样本',
        version: 1,
        likeCount: 12,
        likedByMe: false,
        bookmarked: false,
        canEdit: false,
        createdAt: '2026-11-17T11:00:00Z',
        cookedHtml: `
          <p>测试超宽表格、长链接与加载失败图片对窗口边界的约束：</p>
          <p><a href="https://linux.do/t/very-long-url-segment-without-spaces-for-layout-overflow-verification-1234567890abcdef/1002">https://linux.do/t/very-long-url-segment-without-spaces-for-layout-overflow-verification-1234567890abcdef/1002</a></p>
          <table>
            <thead><tr><th>场景编号</th><th>调度上限</th><th>冷却策略</th><th>失败保留动作</th><th>跨窗口行为</th><th>备注</th></tr></thead>
            <tbody>
              <tr><td>B01</td><td>2 并发 / 3s 4 次</td><td>按主机共享 blockedUntil</td><td>保留已有行与滚动位置</td><td>取消离屏预取</td><td>演示数据</td></tr>
              <tr><td>B08</td><td>禁止写请求自动重放</td><td>进入 outcomeUnknown</td><td>保留快照与操作回执</td><td>不跨账号污染</td><td>需人工确认</td></tr>
            </tbody>
          </table>
          <img src="https://invalid.linux.do/non-existent-demo-image.png" alt="图片加载失败占位样本" />
          <script>window.__xssInjected = true;</script>
        `,
      },
    ],
  },
  1003: {
    summary: DEMO_TOPICS[2],
    canReply: true,
    canCreatePost: true,
    streamPostIds: [301],
    fetchedAt: '2026-11-17T10:05:00Z',
    stale: false,
    posts: [
      {
        id: 301,
        topicId: 1003,
        postNumber: 1,
        username: 'net_kernel',
        displayName: '网络状态机',
        version: 1,
        likeCount: 22,
        likedByMe: false,
        bookmarked: false,
        canEdit: false,
        createdAt: '2026-11-17T10:00:00Z',
        cookedHtml: `<p>10 个窗口同时打开同一主题时，仅发起 1 次进行中的 GET；取消其中 1 扇窗口不中断其余窗口。</p>`,
      },
    ],
  },
};

export class MockDesktopBridge implements DesktopBridge {
  readonly isMock = true;

  private session: SiteSession = {
    siteId: 'linux.do',
    accountId: 'demo-user',
    username: 'demo_reader',
    displayName: '演示用户（Mock）',
    sessionGeneration: 1,
    status: 'connected',
    isDemoData: true,
  };

  private network: NetworkStatusSnapshot = {
    siteId: 'linux.do',
    phase: 'ready',
    blockedUntilUnix: 0,
    retryAfterSeconds: 0,
    inFlightCount: 0,
    queuedCount: 0,
    message: '',
    isDemoData: true,
  };

  private drafts = new Map<string, Draft>([
    [
      'draft-welcome',
      {
        draftId: 'draft-welcome',
        mode: 'replyTopic',
        topicId: 1001,
        title: '回复：欢迎来到 LD Studio',
        body: '这是一份演示草稿：拖动、停靠或最小化窗口时，这里的文本与本地 revision 会保留。',
        revision: 1,
        localState: 'saved',
        cloudState: 'disabled',
        updatedAt: '2026-11-17T12:10:00Z',
      },
    ],
  ]);

  private anchors = new Map<string, ReadAnchor>();

  private notifications: NotificationSnapshot = {
    unreadCount: 2,
    countVersion: 1,
    notifications: [
      {
        id: 501,
        notificationType: 2,
        read: false,
        topicId: 1003,
        postNumber: 1,
        topicTitle: 'RequestCoordinator 共享 429 冷却与 Cloudflare 验证边界讨论',
        actorUsername: 'net_kernel',
        excerpt: '提到了 429 与 challenge 的分流策略',
        createdAt: '2026-11-17T12:08:00Z',
      },
      {
        id: 502,
        notificationType: 1,
        read: false,
        topicId: 1002,
        postNumber: 1,
        topicTitle: '关于在 Windows 桌面端构建多文档工作区的长标题压力测试',
        actorUsername: 'typography_reviewer',
        excerpt: '附带了超宽表格与失败图片样本',
        createdAt: '2026-11-17T11:52:00Z',
      },
    ],
  };

  async getSession(): Promise<SiteSession> {
    return { ...this.session };
  }

  async getNetworkStatus(): Promise<NetworkStatusSnapshot> {
    return { ...this.network };
  }

  async simulateNetworkPhase(phase: NetworkPhase): Promise<NetworkStatusSnapshot> {
    const messages: Record<NetworkPhase, string> = {
      ready: '',
      cooling_down: '访问过于频繁，32 秒后可重试',
      challenge_required: '需要完成站点验证',
      challenge_paused: '验证后接口仍受限，已暂停自动请求',
      auth_expired: '登录状态已失效，需要重新连接',
      forbidden: '当前账号无权访问该资源',
      offline: '网络连接不可用，当前显示本地缓存',
    };
    this.network = {
      ...this.network,
      phase,
      retryAfterSeconds: phase === 'cooling_down' ? 32 : 0,
      message: messages[phase],
    };
    return { ...this.network };
  }

  async loadFeed(_windowInstanceId: string, query: FeedQuery): Promise<FeedPage> {
    const filtered = DEMO_TOPICS.filter((t) => {
      if (query.filter === 'unread') return t.unreadPosts > 0 || t.newPosts > 0;
      if (query.filter === 'top') return t.likeCount >= 18;
      return true;
    });
    return {
      queryKey: `linux.do:demo:${query.filter}`,
      queryGeneration: query.queryGeneration,
      page: query.page,
      hasMore: false,
      topics: filtered,
      fetchedAt: new Date().toISOString(),
      stale: this.network.phase !== 'ready',
    };
  }

  async openTopic(
    instanceId: string,
    topicId: number,
    targetPostNumber?: number,
  ): Promise<{ detail: TopicDetail; anchor: ReadAnchor }> {
    const fallback: TopicDetail = {
      summary: DEMO_TOPICS.find((t) => t.id === topicId) ?? DEMO_TOPICS[0],
      canReply: true,
      canCreatePost: true,
      posts: DEMO_TOPIC_DETAILS[1001].posts,
      streamPostIds: [1, 2],
      fetchedAt: new Date().toISOString(),
      stale: false,
    };
    const detail = structuredClone(DEMO_TOPIC_DETAILS[topicId] ?? fallback);
    if (targetPostNumber) {
      detail.targetPostNumber = targetPostNumber;
    }

    const key = `${instanceId}:${topicId}`;
    const existingAnchor = this.anchors.get(key);
    const anchor: ReadAnchor = targetPostNumber
      ? { instanceId, topicId, postNumber: targetPostNumber, postId: detail.posts[0]?.id ?? 1, offsetRatio: 0 }
      : existingAnchor ?? {
          instanceId,
          topicId,
          postNumber: detail.summary.lastReadPostNumber || 1,
          postId: detail.posts[0]?.id ?? 1,
          offsetRatio: 0,
        };
    this.anchors.set(key, anchor);
    return { detail, anchor };
  }

  async saveReadAnchor(anchor: ReadAnchor): Promise<void> {
    this.anchors.set(`${anchor.instanceId}:${anchor.topicId}`, { ...anchor });
  }

  async search(instanceId: string, keyword: string, queryGeneration: number): Promise<SearchResult> {
    const trimmed = keyword.trim().toLowerCase();
    const hits = !trimmed
      ? []
      : DEMO_TOPICS.filter(
          (t) =>
            t.title.toLowerCase().includes(trimmed) ||
            t.categoryName.toLowerCase().includes(trimmed) ||
            t.authorUsername.toLowerCase().includes(trimmed),
        ).map((t) => ({
          topicId: t.id,
          postId: t.id * 10,
          postNumber: 1,
          title: t.title,
          excerpt: `命中关键词「${keyword}」· ${t.categoryName}`,
          author: t.authorUsername,
          category: t.categoryName,
        }));
    return { instanceId, queryGeneration, page: 0, hasMore: false, hits };
  }

  async saveLocalDraft(draft: Draft): Promise<Draft> {
    const existing = this.drafts.get(draft.draftId);
    if (existing && draft.revision < existing.revision) {
      return { ...existing };
    }
    const saved: Draft = {
      ...draft,
      localState: 'saved',
      updatedAt: new Date().toISOString(),
    };
    this.drafts.set(draft.draftId, saved);
    return { ...saved };
  }

  async listDrafts(): Promise<Draft[]> {
    return Array.from(this.drafts.values()).map((d) => ({ ...d }));
  }

  async submitDraft(draftId: string, expectedRevision: number): Promise<SubmitOperation> {
    const draft = this.drafts.get(draftId);
    if (!draft) {
      return {
        operationId: `op-missing-${Date.now()}`,
        draftId,
        mode: 'replyTopic',
        snapshotRevision: expectedRevision,
        snapshotBody: '',
        state: 'rejected',
        errorMessage: '找不到草稿',
      };
    }
    if (draft.revision !== expectedRevision) {
      return {
        operationId: `op-conflict-${Date.now()}`,
        draftId,
        mode: draft.mode,
        snapshotRevision: draft.revision,
        snapshotBody: draft.body,
        state: 'rejected',
        errorMessage: '正文在确认期间已更新，请重新确认',
      };
    }
    if (this.network.phase === 'offline') {
      return {
        operationId: `op-offline-${Date.now()}`,
        draftId,
        mode: draft.mode,
        snapshotRevision: draft.revision,
        snapshotBody: draft.body,
        state: 'rejected',
        errorMessage: '当前处于离线状态，草稿已安全保存在本地',
      };
    }
    if (draft.body.includes('[enqueue]')) {
      return {
        operationId: `op-enq-${Date.now()}`,
        draftId,
        mode: draft.mode,
        snapshotRevision: draft.revision,
        snapshotBody: draft.body,
        state: 'enqueued',
      };
    }
    return {
      operationId: `op-pub-${Date.now()}`,
      draftId,
      mode: draft.mode,
      snapshotRevision: draft.revision,
      snapshotBody: draft.body,
      state: 'published',
      createdTopicId: draft.topicId ?? 1001,
      createdPostId: Date.now(),
    };
  }

  async getNotifications(): Promise<NotificationSnapshot> {
    return structuredClone(this.notifications);
  }

  async markNotificationVisited(notificationId: number, navigationSucceeded: boolean): Promise<NotificationSnapshot> {
    if (!navigationSucceeded) {
      return structuredClone(this.notifications);
    }
    this.notifications.notifications = this.notifications.notifications.map((n) => {
      if (n.id === notificationId && !n.read) {
        if (this.notifications.unreadCount > 0) this.notifications.unreadCount--;
        return { ...n, read: true };
      }
      return n;
    });
    this.notifications.countVersion++;
    return structuredClone(this.notifications);
  }

  async markAllNotificationsRead(): Promise<NotificationSnapshot> {
    this.notifications.notifications = this.notifications.notifications.map((n) => ({ ...n, read: true }));
    this.notifications.unreadCount = 0;
    this.notifications.countVersion++;
    return structuredClone(this.notifications);
  }

  async toggleLike(postId: number): Promise<{ postId: number; likedByMe: boolean; likeCount: number }> {
    for (const detail of Object.values(DEMO_TOPIC_DETAILS)) {
      const post = detail.posts.find((p) => p.id === postId);
      if (post) {
        post.likedByMe = !post.likedByMe;
        post.likeCount += post.likedByMe ? 1 : -1;
        return { postId, likedByMe: post.likedByMe, likeCount: post.likeCount };
      }
    }
    return { postId, likedByMe: true, likeCount: 1 };
  }

  async openExternalUrl(url: string): Promise<void> {
    if (/^https?:\/\//i.test(url) && typeof window !== 'undefined') {
      window.open(url, '_blank', 'noopener,noreferrer');
    }
  }
}
