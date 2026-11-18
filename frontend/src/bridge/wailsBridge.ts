import { Call, Events } from '@wailsio/runtime';
import type {
  CredentialsInput,
  DesktopBridge,
  Draft,
  FeedPage,
  FeedQuery,
  NetworkStatusSnapshot,
  NotificationSnapshot,
  ReadAnchor,
  SearchResult,
  SiteMetadata,
  SiteSession,
  SubmitOperation,
  TopicDetail,
} from './types';

const PKG = 'github.com/junglecola001/LD-Dt/internal/app';

function serviceCall<T>(service: string, method: string, ...args: unknown[]): Promise<T> {
  return Call.ByName(`${PKG}.${service}.${method}`, ...args) as Promise<T>;
}

/**
 * Real desktop bridge routing all reads/writes through Wails v3 bound Go application services
 * and the shared RequestCoordinator (never direct fetch from the UI WebView).
 */
export class WailsDesktopBridge implements DesktopBridge {
  readonly isMock = false;

  async getSession(): Promise<SiteSession> {
    return serviceCall<SiteSession>('AuthService', 'GetSession');
  }

  async getNetworkStatus(): Promise<NetworkStatusSnapshot> {
    return serviceCall<NetworkStatusSnapshot>('AuthService', 'GetNetworkStatus');
  }

  async getSiteMetadata(): Promise<SiteMetadata> {
    return serviceCall<SiteMetadata>('TopicService', 'GetSiteMetadata');
  }

  async openShieldWindow(mode: 'challenge' | 'login'): Promise<void> {
    await serviceCall<void>('AuthService', 'OpenShieldWindow', mode);
  }

  async applyCredentials(input: CredentialsInput): Promise<{ session: SiteSession; network: NetworkStatusSnapshot }> {
    return serviceCall<{ session: SiteSession; network: NetworkStatusSnapshot }>('AuthService', 'ApplyCredentials', input);
  }

  async probeRecovery(): Promise<{ session: SiteSession; network: NetworkStatusSnapshot }> {
    return serviceCall<{ session: SiteSession; network: NetworkStatusSnapshot }>('AuthService', 'ProbeRecovery');
  }

  subscribeNetworkStatus(listener: (snap: NetworkStatusSnapshot) => void): () => void {
    return Events.On('NetworkStateChanged', (event) => {
      const payload = event?.data as unknown as NetworkStatusSnapshot | undefined;
      if (payload) listener(payload);
    });
  }

  subscribeSession(listener: (sess: SiteSession) => void): () => void {
    return Events.On('SessionChanged', (event) => {
      const payload = event?.data as unknown as SiteSession | undefined;
      if (payload) listener(payload);
    });
  }

  async loadFeed(windowInstanceId: string, query: FeedQuery): Promise<FeedPage> {
    return serviceCall<FeedPage>('TopicService', 'LoadFeed', windowInstanceId, {
      siteId: 'linux.do',
      accountScope: '',
      filter: query.filter,
      categorySlug: query.categorySlug ?? '',
      parentCategorySlug: query.parentCategorySlug ?? '',
      categoryId: query.categoryId ?? 0,
      tag: query.tag ?? '',
      page: query.page,
      queryGeneration: query.queryGeneration,
    });
  }

  async openTopic(
    instanceId: string,
    topicId: number,
    targetPostNumber = 0,
  ): Promise<{ detail: TopicDetail; anchor: ReadAnchor }> {
    const res = await serviceCall<[TopicDetail, ReadAnchor]>(
      'TopicService',
      'OpenTopic',
      instanceId,
      topicId,
      targetPostNumber,
    );
    return { detail: res[0], anchor: res[1] };
  }

  async saveReadAnchor(anchor: ReadAnchor): Promise<void> {
    await serviceCall<void>('TopicService', 'SaveReadAnchor', {
      ...anchor,
      siteId: 'linux.do',
      accountScope: '',
    });
  }

  async search(instanceId: string, keyword: string, queryGeneration: number, topicId = 0): Promise<SearchResult> {
    return serviceCall<SearchResult>('SearchService', 'Execute', {
      siteId: 'linux.do',
      accountScope: '',
      instanceId,
      topicId,
      keyword,
      page: 1,
      queryGeneration,
    });
  }

  async saveLocalDraft(draft: Draft): Promise<Draft> {
    return serviceCall<Draft>('ComposerService', 'SaveLocalDraft', {
      ...draft,
      siteId: 'linux.do',
      accountScope: '',
    });
  }

  async listDrafts(): Promise<Draft[]> {
    return serviceCall<Draft[]>('ComposerService', 'ListDrafts');
  }

  async submitDraft(draftId: string, expectedRevision: number): Promise<SubmitOperation> {
    return serviceCall<SubmitOperation>('ComposerService', 'SubmitDraft', draftId, expectedRevision);
  }

  async getNotifications(): Promise<NotificationSnapshot> {
    return serviceCall<NotificationSnapshot>('NotificationService', 'GetNotifications');
  }

  async markNotificationVisited(notificationId: number, navigationSucceeded: boolean): Promise<NotificationSnapshot> {
    return serviceCall<NotificationSnapshot>(
      'NotificationService',
      'ConfirmNotificationVisited',
      notificationId,
      navigationSucceeded,
    );
  }

  async markAllNotificationsRead(): Promise<NotificationSnapshot> {
    return serviceCall<NotificationSnapshot>('NotificationService', 'MarkAllNotificationsRead');
  }

  async toggleLike(postId: number): Promise<{ postId: number; likedByMe: boolean; likeCount: number }> {
    return { postId, likedByMe: false, likeCount: 0 };
  }

  async openExternalUrl(url: string): Promise<void> {
    await serviceCall<void>('AuthService', 'OpenExternalURL', url);
  }
}
