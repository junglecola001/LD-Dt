package discourse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/network"
)

// Adapter encapsulates all Discourse / Linux.do HTTP endpoints behind RequestCoordinator.
// Endpoint paths and tolerance rules match FluxDO commit 185ccaa418dab8ea40192301e39065706192f866.
type Adapter struct {
	baseURL string
	coord   *network.RequestCoordinator
}

// NewAdapter creates a DiscourseAdapter targeting the given HTTPS base URL (e.g. https://linux.do).
func NewAdapter(baseURL string, coord *network.RequestCoordinator) *Adapter {
	return &Adapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		coord:   coord,
	}
}

// FetchSiteMetadata loads categories (/site.json) and tag counts (/tags.json) for the left navigation rail.
func (a *Adapter) FetchSiteMetadata(ctx context.Context) (domain.SiteMetadata, error) {
	rawSite, err := a.coord.DoSafeGET(ctx, "site:metadata", true, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, a.baseURL+"/site.json", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	if err != nil {
		return domain.SiteMetadata{}, err
	}

	meta, err := parseSiteJSON(rawSite.Body)
	if err != nil {
		return domain.SiteMetadata{}, err
	}
	if meta.Categories == nil {
		meta.Categories = []domain.CategoryItem{}
	}
	if meta.Tags == nil {
		meta.Tags = []domain.TagItem{}
	}

	// Best-effort /tags.json enrichment matching FluxDO _categories.dart#getSiteTagGroups.
	if meta.CanTagTopics {
		rawTags, tagErr := a.coord.DoSafeGET(ctx, "site:tags", true, func() (*http.Request, error) {
			req, err := http.NewRequest(http.MethodGet, a.baseURL+"/tags.json", nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Accept", "application/json")
			return req, nil
		})
		if tagErr == nil {
			if richTags := parseTagsJSON(rawTags.Body); len(richTags) > 0 {
				meta.Tags = richTags
			}
		}
	}
	return meta, nil
}

// FetchCurrentUser checks /session/current.json to verify authenticated user identity.
func (a *Adapter) FetchCurrentUser(ctx context.Context) (accountID, username, displayName string, err error) {
	raw, err := a.coord.DoSafeGET(ctx, "session:current", true, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, a.baseURL+"/session/current.json", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	if err != nil {
		return "", "", "", err
	}
	var payload struct {
		CurrentUser *struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"current_user"`
	}
	if err := json.Unmarshal(raw.Body, &payload); err != nil {
		return "", "", "", err
	}
	if payload.CurrentUser == nil {
		return "", "", "", nil
	}
	return strconv.FormatInt(payload.CurrentUser.ID, 10), payload.CurrentUser.Username, payload.CurrentUser.Name, nil
}

// FetchFeed retrieves a feed page through RequestCoordinator supporting category and tag routes (FluxDO _topics.dart).
func (a *Adapter) FetchFeed(ctx context.Context, query domain.FeedQuery) (domain.FeedPage, error) {
	subFilter := "latest"
	switch query.Filter {
	case "top", "unread", "new", "latest":
		subFilter = query.Filter
	}

	var path string
	switch {
	case query.CategoryID > 0 && query.CategorySlug != "":
		if query.ParentCategorySlug != "" {
			path = fmt.Sprintf("/c/%s/%s/%d/l/%s.json",
				url.PathEscape(query.ParentCategorySlug),
				url.PathEscape(query.CategorySlug),
				query.CategoryID,
				subFilter,
			)
		} else {
			path = fmt.Sprintf("/c/%s/%d/l/%s.json",
				url.PathEscape(query.CategorySlug),
				query.CategoryID,
				subFilter,
			)
		}
	case query.Tag != "":
		path = fmt.Sprintf("/tag/%s/l/%s.json", url.PathEscape(query.Tag), subFilter)
	default:
		path = fmt.Sprintf("/%s.json", subFilter)
	}

	params := url.Values{}
	if query.Page > 0 {
		params.Set("page", strconv.Itoa(query.Page))
	}
	if query.Order != "" {
		params.Set("order", query.Order)
	}
	if query.Period != "" {
		params.Set("period", query.Period)
	}
	if query.CategoryID > 0 && query.Tag != "" {
		params.Add("tags[]", query.Tag)
	}

	fullURL := a.baseURL + path
	if encoded := params.Encode(); encoded != "" {
		fullURL += "?" + encoded
	}

	raw, err := a.coord.DoSafeGET(ctx, "feed:"+query.CacheKey()+":"+strconv.Itoa(query.Page), true, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	if err != nil {
		return domain.FeedPage{}, err
	}

	return parseFeedJSON(raw.Body, query)
}

// FetchTopic retrieves a topic chunk through RequestCoordinator.
func (a *Adapter) FetchTopic(ctx context.Context, _ string, topicID int64, aroundPostNumber int) (domain.TopicDetail, error) {
	endpoint := fmt.Sprintf("%s/t/%d.json", a.baseURL, topicID)
	if aroundPostNumber > 1 {
		endpoint = fmt.Sprintf("%s/t/%d/%d.json", a.baseURL, topicID, aroundPostNumber)
	}
	dedupKey := fmt.Sprintf("topic:%d:%d", topicID, aroundPostNumber)

	raw, err := a.coord.DoSafeGET(ctx, dedupKey, true, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	if err != nil {
		return domain.TopicDetail{}, err
	}
	return parseTopicJSON(raw.Body)
}

// Search executes Discourse /search.json through RequestCoordinator.
func (a *Adapter) Search(ctx context.Context, query domain.SearchQuery) (domain.SearchResult, error) {
	params := url.Values{}
	q := query.Keyword
	if query.TopicID > 0 {
		q = fmt.Sprintf("%s topic:%d", q, query.TopicID)
	}
	params.Set("q", q)
	if query.Page > 1 {
		params.Set("page", strconv.Itoa(query.Page))
	}
	fullURL := a.baseURL + "/search.json?" + params.Encode()
	dedupKey := fmt.Sprintf("search:%s:%d:%s", query.AccountScope, query.TopicID, params.Encode())

	raw, err := a.coord.DoSafeGET(ctx, dedupKey, true, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	if err != nil {
		return domain.SearchResult{}, err
	}
	return parseSearchJSON(raw.Body, query)
}

// FetchNotifications retrieves recent notifications for authenticated accounts (FluxDO _notifications.dart).
func (a *Adapter) FetchNotifications(ctx context.Context, siteID, accountScope string) ([]domain.Notification, error) {
	raw, err := a.coord.DoSafeGET(ctx, "notifications:"+accountScope, true, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, a.baseURL+"/notifications?limit=30&recent=true", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	return parseNotificationsJSON(raw.Body, siteID, accountScope)
}

// MarkNotificationRead marks a single notification read via Discourse PUT /notifications/mark-read.
func (a *Adapter) MarkNotificationRead(ctx context.Context, notificationID int64) error {
	payload, _ := json.Marshal(map[string]int64{"id": notificationID})
	_, err := a.coord.DoWrite(ctx, true, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPut, a.baseURL+"/notifications/mark-read", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	return err
}

// MarkAllNotificationsRead marks all notifications read.
func (a *Adapter) MarkAllNotificationsRead(ctx context.Context) error {
	_, err := a.coord.DoWrite(ctx, true, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPut, a.baseURL+"/notifications/mark-read", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	return err
}

// FetchBookmarks loads user bookmarks from Discourse.
func (a *Adapter) FetchBookmarks(ctx context.Context, page int) ([]domain.Bookmark, error) {
	endpoint := a.baseURL + "/bookmarks.json"
	if page > 0 {
		endpoint += "?page=" + strconv.Itoa(page)
	}
	raw, err := a.coord.DoSafeGET(ctx, "bookmarks:"+strconv.Itoa(page), true, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	var payload struct {
		UserBookmarkList struct {
			Bookmarks []struct {
				ID          int64  `json:"id"`
				TopicID     int64  `json:"topic_id"`
				TopicTitle  string `json:"title"`
				LinkedPostN int    `json:"linked_post_number"`
				Name        string `json:"name"`
				ReminderAt  string `json:"reminder_at"`
			} `json:"bookmarks"`
		} `json:"user_bookmark_list"`
	}
	if err := json.Unmarshal(raw.Body, &payload); err != nil {
		return nil, err
	}
	out := make([]domain.Bookmark, 0, len(payload.UserBookmarkList.Bookmarks))
	for _, b := range payload.UserBookmarkList.Bookmarks {
		out = append(out, domain.Bookmark{
			ID:         b.ID,
			SiteID:     "linux.do",
			TopicID:    b.TopicID,
			PostNumber: b.LinkedPostN,
			Name:       b.Name,
			TopicTitle: b.TopicTitle,
			ReminderAt: b.ReminderAt,
		})
	}
	return out, nil
}

// DeleteBookmark deletes a bookmark on Discourse.
func (a *Adapter) DeleteBookmark(ctx context.Context, bookmarkID int64) error {
	_, err := a.coord.DoWrite(ctx, true, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/bookmarks/%d.json", a.baseURL, bookmarkID), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	return err
}

func parseSiteJSON(body []byte) (domain.SiteMetadata, error) {
	var raw struct {
		CanTagTopics bool `json:"can_tag_topics"`
		Categories   []struct {
			ID               int64  `json:"id"`
			Name             string `json:"name"`
			Slug             string `json:"slug"`
			Color            string `json:"color"`
			TextColor        string `json:"text_color"`
			TopicCount       int    `json:"topic_count"`
			ParentCategoryID int64  `json:"parent_category_id"`
			DescriptionText  string `json:"description_text"`
		} `json:"categories"`
		TopTags []json.RawMessage `json:"top_tags"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.SiteMetadata{}, domain.NewError(domain.ErrCodeValidationFailed, "解析站点分类失败", true, err)
	}

	cats := make([]domain.CategoryItem, 0, len(raw.Categories))
	for _, c := range raw.Categories {
		if c.Name == "" {
			continue
		}
		cats = append(cats, domain.CategoryItem{
			ID:               c.ID,
			Name:             c.Name,
			Slug:             c.Slug,
			Color:            c.Color,
			TextColor:        c.TextColor,
			TopicCount:       c.TopicCount,
			ParentCategoryID: c.ParentCategoryID,
			Description:      c.DescriptionText,
		})
	}
	sort.SliceStable(cats, func(i, j int) bool {
		return cats[i].TopicCount > cats[j].TopicCount
	})

	tags := make([]domain.TagItem, 0, len(raw.TopTags))
	seen := make(map[string]struct{})
	for _, item := range raw.TopTags {
		var s string
		if err := json.Unmarshal(item, &s); err == nil && s != "" {
			if _, ok := seen[s]; !ok {
				seen[s] = struct{}{}
				tags = append(tags, domain.TagItem{Name: s})
			}
			continue
		}
		var obj struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		if err := json.Unmarshal(item, &obj); err == nil && obj.Name != "" {
			if _, ok := seen[obj.Name]; !ok {
				seen[obj.Name] = struct{}{}
				tags = append(tags, domain.TagItem{Name: obj.Name, Count: obj.Count})
			}
		}
	}

	return domain.SiteMetadata{
		SiteID:       "linux.do",
		CanTagTopics: raw.CanTagTopics,
		Categories:   cats,
		Tags:         tags,
		FetchedAt:    time.Now(),
	}, nil
}

func parseTagsJSON(body []byte) []domain.TagItem {
	var raw struct {
		Tags []struct {
			ID         string `json:"id"`
			Text       string `json:"text"`
			Name       string `json:"name"`
			TopicCount int    `json:"topic_count"`
			Count      int    `json:"count"`
		} `json:"tags"`
		Extras struct {
			TagGroups []struct {
				Name string `json:"name"`
				Tags []struct {
					ID         string `json:"id"`
					Text       string `json:"text"`
					Name       string `json:"name"`
					TopicCount int    `json:"topic_count"`
					Count      int    `json:"count"`
				} `json:"tags"`
			} `json:"tag_groups"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	var out []domain.TagItem
	add := func(name, text, id, group string, count, topicCount int) {
		resolved := name
		if resolved == "" {
			resolved = text
		}
		if resolved == "" {
			resolved = id
		}
		if resolved == "" {
			return
		}
		if _, dup := seen[resolved]; dup {
			return
		}
		seen[resolved] = struct{}{}
		c := count
		if c == 0 {
			c = topicCount
		}
		out = append(out, domain.TagItem{Name: resolved, Count: c, GroupName: group})
	}

	for _, g := range raw.Extras.TagGroups {
		for _, t := range g.Tags {
			add(t.Name, t.Text, t.ID, g.Name, t.Count, t.TopicCount)
		}
	}
	for _, t := range raw.Tags {
		add(t.Name, t.Text, t.ID, "", t.Count, t.TopicCount)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Count > out[j].Count
	})
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func parseFeedJSON(body []byte, query domain.FeedQuery) (domain.FeedPage, error) {
	var raw struct {
		TopicList struct {
			MoreTopicsURL string `json:"more_topics_url"`
			Topics        []struct {
				ID                 int64     `json:"id"`
				Title              string    `json:"title"`
				Slug               string    `json:"slug"`
				CategoryID         int64     `json:"category_id"`
				PostsCount         int       `json:"posts_count"`
				ReplyCount         int       `json:"reply_count"`
				Views              int       `json:"views"`
				LikeCount          int       `json:"like_count"`
				UnreadPosts        int       `json:"unread_posts"`
				NewPosts           int       `json:"new_posts"`
				HighestPostNumber  int       `json:"highest_post_number"`
				LastReadPostNumber int       `json:"last_read_post_number"`
				Pinned             bool      `json:"pinned"`
				Closed             bool      `json:"closed"`
				Archived           bool      `json:"archived"`
				NotificationLevel  int       `json:"notification_level"`
				LastPostedAt       time.Time `json:"last_posted_at"`
				LastPosterUsername string    `json:"last_poster_username"`
			} `json:"topics"`
		} `json:"topic_list"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.FeedPage{}, domain.NewError(domain.ErrCodeValidationFailed, "解析列表数据失败", true, err)
	}

	topics := make([]domain.TopicSummary, 0, len(raw.TopicList.Topics))
	for _, t := range raw.TopicList.Topics {
		topics = append(topics, domain.TopicSummary{
			ID:                 t.ID,
			Title:              t.Title,
			Slug:               t.Slug,
			CategoryID:         t.CategoryID,
			AuthorUsername:     t.LastPosterUsername,
			PostsCount:         t.PostsCount,
			ReplyCount:         t.ReplyCount,
			Views:              t.Views,
			LikeCount:          t.LikeCount,
			UnreadPosts:        t.UnreadPosts,
			NewPosts:           t.NewPosts,
			HighestPostNumber:  t.HighestPostNumber,
			LastReadPostNumber: t.LastReadPostNumber,
			Pinned:             t.Pinned,
			Closed:             t.Closed,
			Archived:           t.Archived,
			NotificationLevel:  domain.NotificationLevel(t.NotificationLevel),
			LastPostedAt:       t.LastPostedAt,
		})
	}
	return domain.FeedPage{
		QueryKey:        query.CacheKey(),
		QueryGeneration: query.QueryGeneration,
		Page:            query.Page,
		HasMore:         raw.TopicList.MoreTopicsURL != "",
		Topics:          topics,
		FetchedAt:       time.Now(),
	}, nil
}

func parseTopicJSON(body []byte) (domain.TopicDetail, error) {
	var raw struct {
		ID                 int64  `json:"id"`
		Title              string `json:"title"`
		Slug               string `json:"slug"`
		CategoryID         int64  `json:"category_id"`
		PostsCount         int    `json:"posts_count"`
		HighestPostNumber  int    `json:"highest_post_number"`
		LastReadPostNumber int    `json:"last_read_post_number"`
		Details            struct {
			CanReply      bool `json:"can_reply_as_new_topic"`
			CanCreatePost bool `json:"can_create_post"`
		} `json:"details"`
		PostStream struct {
			Stream []int64 `json:"stream"`
			Posts  []struct {
				ID             int64     `json:"id"`
				TopicID        int64     `json:"topic_id"`
				PostNumber     int       `json:"post_number"`
				Username       string    `json:"username"`
				Name           string    `json:"name"`
				AvatarTemplate string    `json:"avatar_template"`
				Cooked         string    `json:"cooked"`
				Raw            string    `json:"raw"`
				Version        int       `json:"version"`
				CanEdit        bool      `json:"can_edit"`
				CanDelete      bool      `json:"can_delete"`
				CreatedAt      time.Time `json:"created_at"`
				UpdatedAt      time.Time `json:"updated_at"`
			} `json:"posts"`
		} `json:"post_stream"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.TopicDetail{}, domain.NewError(domain.ErrCodeValidationFailed, "解析主题详情失败", true, err)
	}

	posts := make([]domain.Post, 0, len(raw.PostStream.Posts))
	for _, p := range raw.PostStream.Posts {
		posts = append(posts, domain.Post{
			ID:             p.ID,
			TopicID:        p.TopicID,
			PostNumber:     p.PostNumber,
			Username:       p.Username,
			DisplayName:    p.Name,
			AvatarTemplate: p.AvatarTemplate,
			CookedHTML:     p.Cooked,
			RawMarkdown:    p.Raw,
			Version:        p.Version,
			CanEdit:        p.CanEdit,
			CanDelete:      p.CanDelete,
			CreatedAt:      p.CreatedAt,
			UpdatedAt:      p.UpdatedAt,
		})
	}

	return domain.TopicDetail{
		Summary: domain.TopicSummary{
			ID:                 raw.ID,
			Title:              raw.Title,
			Slug:               raw.Slug,
			CategoryID:         raw.CategoryID,
			PostsCount:         raw.PostsCount,
			HighestPostNumber:  raw.HighestPostNumber,
			LastReadPostNumber: raw.LastReadPostNumber,
		},
		CanReply:      raw.Details.CanReply,
		CanCreatePost: raw.Details.CanCreatePost,
		Posts:         posts,
		StreamPostIDs: raw.PostStream.Stream,
		FetchedAt:     time.Now(),
	}, nil
}

func parseSearchJSON(body []byte, query domain.SearchQuery) (domain.SearchResult, error) {
	var raw struct {
		Topics []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"topics"`
		Posts []struct {
			ID         int64  `json:"id"`
			TopicID    int64  `json:"topic_id"`
			PostNumber int    `json:"post_number"`
			Username   string `json:"username"`
			Blurb      string `json:"blurb"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.SearchResult{}, domain.NewError(domain.ErrCodeValidationFailed, "解析搜索结果失败", true, err)
	}

	titles := make(map[int64]string, len(raw.Topics))
	for _, t := range raw.Topics {
		titles[t.ID] = t.Title
	}
	hits := make([]domain.SearchHit, 0, len(raw.Posts))
	for _, p := range raw.Posts {
		hits = append(hits, domain.SearchHit{
			TopicID:    p.TopicID,
			PostID:     p.ID,
			PostNumber: p.PostNumber,
			Title:      titles[p.TopicID],
			Excerpt:    p.Blurb,
			Author:     p.Username,
		})
	}
	return domain.SearchResult{
		InstanceID:      query.InstanceID,
		QueryGeneration: query.QueryGeneration,
		Page:            query.Page,
		Hits:            hits,
	}, nil
}

func parseNotificationsJSON(body []byte, siteID, accountScope string) ([]domain.Notification, error) {
	var raw struct {
		Notifications []struct {
			ID               int64     `json:"id"`
			NotificationType int       `json:"notification_type"`
			Read             bool      `json:"read"`
			TopicID          int64     `json:"topic_id"`
			PostNumber       int       `json:"post_number"`
			CreatedAt        time.Time `json:"created_at"`
			Data             struct {
				TopicTitle       string `json:"topic_title"`
				OriginalUsername string `json:"original_username"`
				DisplayUsername  string `json:"display_username"`
				Excerpt          string `json:"excerpt"`
			} `json:"data"`
		} `json:"notifications"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, domain.NewError(domain.ErrCodeValidationFailed, "解析通知失败", true, err)
	}

	out := make([]domain.Notification, 0, len(raw.Notifications))
	for _, n := range raw.Notifications {
		actor := n.Data.OriginalUsername
		if actor == "" {
			actor = n.Data.DisplayUsername
		}
		out = append(out, domain.Notification{
			ID:               n.ID,
			SiteID:           siteID,
			AccountScope:     accountScope,
			NotificationType: n.NotificationType,
			Read:             n.Read,
			TopicID:          n.TopicID,
			PostNumber:       n.PostNumber,
			TopicTitle:       n.Data.TopicTitle,
			ActorUsername:    actor,
			Excerpt:          n.Data.Excerpt,
			CreatedAt:        n.CreatedAt,
		})
	}
	return out, nil
}
