package discourse

import (
	"testing"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

func TestParseFeedTopicSiteTagsSearchAndNotifications(t *testing.T) {
	feedPayload := []byte(`{
		"topic_list": {
			"more_topics_url": "/latest?page=1",
			"topics": [
				{"id": 1001, "title": "Go + Wails", "slug": "go-wails", "posts_count": 5, "highest_post_number": 5}
			]
		}
	}`)
	page, err := parseFeedJSON(feedPayload, domain.FeedQuery{SiteID: "linux.do", Filter: "latest"})
	if err != nil {
		t.Fatalf("parseFeedJSON failed: %v", err)
	}
	if !page.HasMore || len(page.Topics) != 1 || page.Topics[0].ID != 1001 {
		t.Fatalf("unexpected feed page: %+v", page)
	}

	topicPayload := []byte(`{
		"id": 1001,
		"title": "Go + Wails",
		"post_stream": {
			"stream": [1, 2],
			"posts": [
				{"id": 1, "topic_id": 1001, "post_number": 1, "username": "neo", "cooked": "<p>hello</p>"}
			]
		}
	}`)
	detail, err := parseTopicJSON(topicPayload)
	if err != nil {
		t.Fatalf("parseTopicJSON failed: %v", err)
	}
	if detail.Summary.ID != 1001 || len(detail.Posts) != 1 || detail.Posts[0].CookedHTML != "<p>hello</p>" {
		t.Fatalf("unexpected topic detail: %+v", detail)
	}

	sitePayload := []byte(`{
		"can_tag_topics": true,
		"categories": [
			{"id": 11, "name": "开发调优", "slug": "develop", "color": "0088CC", "topic_count": 1200}
		],
		"top_tags": ["wails", "cloudflare"]
	}`)
	meta, err := parseSiteJSON(sitePayload)
	if err != nil || !meta.CanTagTopics || len(meta.Categories) != 1 || len(meta.Tags) != 2 {
		t.Fatalf("unexpected site metadata: %+v err=%v", meta, err)
	}

	tagsPayload := []byte(`{
		"tags": [{"id": "go", "name": "go", "topic_count": 320}],
		"extras": {"tag_groups": [{"name": "技术", "tags": [{"name": "rust", "count": 500}]}]}
	}`)
	richTags := parseTagsJSON(tagsPayload)
	if len(richTags) != 2 || richTags[0].Name != "rust" || richTags[0].Count != 500 {
		t.Fatalf("unexpected rich tags: %+v", richTags)
	}

	searchPayload := []byte(`{
		"topics": [{"id": 1001, "title": "Go + Wails"}],
		"posts": [{"id": 1, "topic_id": 1001, "post_number": 1, "username": "neo", "blurb": "hit"}]
	}`)
	searchRes, err := parseSearchJSON(searchPayload, domain.SearchQuery{InstanceID: "s1", QueryGeneration: 2})
	if err != nil || len(searchRes.Hits) != 1 || searchRes.Hits[0].Title != "Go + Wails" {
		t.Fatalf("unexpected search result: %+v err=%v", searchRes, err)
	}

	notifPayload := []byte(`{
		"notifications": [{"id": 9, "notification_type": 2, "read": false, "topic_id": 1001, "post_number": 3, "data": {"topic_title": "Go + Wails", "original_username": "neo", "excerpt": "replied"}}]
	}`)
	notifs, err := parseNotificationsJSON(notifPayload, "linux.do", "linux.do:anon")
	if err != nil || len(notifs) != 1 || notifs[0].ActorUsername != "neo" {
		t.Fatalf("unexpected notifications: %+v err=%v", notifs, err)
	}
}
