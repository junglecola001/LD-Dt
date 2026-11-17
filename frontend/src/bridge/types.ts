export type SessionStatus = 'anonymous' | 'verifying' | 'connected' | 'reauth_needed';

export type NetworkPhase =
  | 'ready'
  | 'cooling_down'
  | 'challenge_required'
  | 'challenge_paused'
  | 'auth_expired'
  | 'forbidden'
  | 'offline';

export type ComposerMode = 'newTopic' | 'replyTopic' | 'replyPost' | 'editPost';

export type LocalDraftState = 'restoring' | 'editing' | 'saving' | 'saved' | 'saveFailed';

export type CloudDraftState = 'disabled' | 'pending' | 'syncing' | 'synced' | 'conflict' | 'failed';

export type SubmitState = 'idle' | 'preparing' | 'submitting' | 'published' | 'enqueued' | 'rejected' | 'outcomeUnknown';

export type UploadState = 'queued' | 'uploading' | 'uploaded' | 'inserted' | 'failed' | 'cancelled';

export interface SiteSession {
  siteId: string;
  accountId: string;
  username: string;
  displayName: string;
  sessionGeneration: number;
  status: SessionStatus;
  isDemoData?: boolean;
}

export interface NetworkStatusSnapshot {
  siteId: string;
  phase: NetworkPhase;
  blockedUntilUnix: number;
  retryAfterSeconds: number;
  inFlightCount: number;
  queuedCount: number;
  lastErrorCode?: string;
  message: string;
  isDemoData?: boolean;
}

export interface TopicSummary {
  id: number;
  title: string;
  slug: string;
  categoryId: number;
  categoryName: string;
  authorUsername: string;
  postsCount: number;
  replyCount: number;
  views: number;
  likeCount: number;
  unreadPosts: number;
  newPosts: number;
  highestPostNumber: number;
  lastReadPostNumber: number;
  pinned: boolean;
  closed: boolean;
  archived: boolean;
  notificationLevel: number;
  lastPostedAt: string;
}

export interface FeedQuery {
  filter: 'latest' | 'unread' | 'top' | 'category';
  categorySlug?: string;
  categoryId?: number;
  page: number;
  queryGeneration: number;
}

export interface FeedPage {
  queryKey: string;
  queryGeneration: number;
  page: number;
  hasMore: boolean;
  topics: TopicSummary[];
  fetchedAt: string;
  stale: boolean;
}

export interface Post {
  id: number;
  topicId: number;
  postNumber: number;
  username: string;
  displayName: string;
  cookedHtml: string;
  rawMarkdown?: string;
  version: number;
  likeCount: number;
  likedByMe: boolean;
  bookmarked: boolean;
  canEdit: boolean;
  createdAt: string;
}

export interface TopicDetail {
  summary: TopicSummary;
  canReply: boolean;
  canCreatePost: boolean;
  posts: Post[];
  streamPostIds: number[];
  targetPostNumber?: number;
  fetchedAt: string;
  stale: boolean;
}

export interface ReadAnchor {
  instanceId: string;
  topicId: number;
  postNumber: number;
  postId: number;
  offsetRatio: number;
}

export interface SearchHit {
  topicId: number;
  postId: number;
  postNumber: number;
  title: string;
  excerpt: string;
  author: string;
  category: string;
}

export interface SearchResult {
  instanceId: string;
  queryGeneration: number;
  page: number;
  hasMore: boolean;
  hits: SearchHit[];
}

export interface UploadTask {
  uploadId: string;
  draftId: string;
  fileName: string;
  state: UploadState;
  shortUrl?: string;
  resolvedUrl?: string;
  errorMessage?: string;
  generation: number;
}

export interface Draft {
  draftId: string;
  mode: ComposerMode;
  topicId?: number;
  replyToPostNumber?: number;
  editPostId?: number;
  initialRaw?: string;
  editReason?: string;
  title?: string;
  categoryId?: number;
  body: string;
  revision: number;
  localState: LocalDraftState;
  cloudState: CloudDraftState;
  conflictBody?: string;
  uploads?: UploadTask[];
  updatedAt: string;
}

export interface SubmitOperation {
  operationId: string;
  draftId: string;
  mode: ComposerMode;
  snapshotRevision: number;
  snapshotBody: string;
  state: SubmitState;
  createdTopicId?: number;
  createdPostId?: number;
  errorMessage?: string;
}

export interface NotificationItem {
  id: number;
  notificationType: number;
  read: boolean;
  topicId: number;
  postNumber: number;
  topicTitle: string;
  actorUsername: string;
  excerpt: string;
  createdAt: string;
}

export interface NotificationSnapshot {
  unreadCount: number;
  countVersion: number;
  notifications: NotificationItem[];
}

export interface DesktopBridge {
  readonly isMock: boolean;
  getSession(): Promise<SiteSession>;
  getNetworkStatus(): Promise<NetworkStatusSnapshot>;
  simulateNetworkPhase?(phase: NetworkPhase): Promise<NetworkStatusSnapshot>;
  loadFeed(windowInstanceId: string, query: FeedQuery): Promise<FeedPage>;
  openTopic(instanceId: string, topicId: number, targetPostNumber?: number): Promise<{ detail: TopicDetail; anchor: ReadAnchor }>;
  saveReadAnchor(anchor: ReadAnchor): Promise<void>;
  search(instanceId: string, keyword: string, queryGeneration: number, topicId?: number): Promise<SearchResult>;
  saveLocalDraft(draft: Draft): Promise<Draft>;
  listDrafts(): Promise<Draft[]>;
  submitDraft(draftId: string, expectedRevision: number): Promise<SubmitOperation>;
  getNotifications(): Promise<NotificationSnapshot>;
  markNotificationVisited(notificationId: number, navigationSucceeded: boolean): Promise<NotificationSnapshot>;
  markAllNotificationsRead(): Promise<NotificationSnapshot>;
  toggleLike(postId: number): Promise<{ postId: number; likedByMe: boolean; likeCount: number }>;
  openExternalUrl(url: string): Promise<void>;
}
