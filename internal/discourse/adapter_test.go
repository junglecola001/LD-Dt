package discourse

import (
	"testing"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

func TestParseFeedAndTopicMissingFieldsTolerance(t *testing.T) {
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
}
