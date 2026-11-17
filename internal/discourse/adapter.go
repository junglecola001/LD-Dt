package discourse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/network"
)

// Adapter encapsulates all Discourse / Linux.do HTTP endpoints behind RequestCoordinator.
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

// FetchFeed retrieves a feed page through RequestCoordinator with full query identity.
func (a *Adapter) FetchFeed(ctx context.Context, query domain.FeedQuery) (domain.FeedPage, error) {
	path := "/latest.json"
	switch query.Filter {
	case "top":
		path = "/top.json"
	case "unread":
		path = "/unread.json"
	case "category":
		if query.CategorySlug != "" && query.CategoryID > 0 {
			path = fmt.Sprintf("/c/%s/%d.json", url.PathEscape(query.CategorySlug), query.CategoryID)
		}
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
		ID                 int64 `json:"id"`
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
