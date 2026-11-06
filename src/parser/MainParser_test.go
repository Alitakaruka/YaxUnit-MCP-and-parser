package YaxUnitParser

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// FIX-01 TEST BEGIN: Fake HTTP responses keep this regression test offline.
type parserTestTransport struct{}

func (parserTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			`<div class="theme-doc-markdown"><header><h1>Test</h1></header>Content</div>`,
		)),
		Request: req,
	}, nil
}

func TestParseAllWaitsForCallbacks(t *testing.T) {
	originalTransport := http.DefaultTransport
	http.DefaultTransport = parserTestTransport{}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan int, 1)
	go func() {
		count := 0
		ParseAll(func(PageData) {
			if count == 0 {
				close(entered)
				<-release
			}
			count++
		})
		done <- count
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("parser did not invoke the callback")
	}

	select {
	case count := <-done:
		close(release)
		t.Fatalf("ParseAll returned while a callback was blocked: count=%d", count)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case count := <-done:
		if count != 2 {
			t.Fatalf("processed %d pages, want 2", count)
		}
	case <-time.After(time.Second):
		t.Fatal("ParseAll did not finish after releasing the callback")
	}
}
// FIX-01 TEST END.

// FIX-07 TEST BEGIN: Normalization must be idempotent for pagination links.
func TestGetNormalPath(t *testing.T) {
	encoded := "/yaxunit/api/%D0%9C%D0%BE%D0%BA%D0%B8%D1%82%D0%BE"
	cases := []struct {
		name string
		path string
		want string
	}{
		{"plain", "/yaxunit/docs/getting-started/", "/yaxunit/docs/getting-started"},
		{"unicode", "/yaxunit/api/\u041c\u043e\u043a\u0438\u0442\u043e", encoded},
		{"encoded", encoded, encoded},
		{"fragment", encoded + "#example", encoded},
		{"query", encoded + "?version=1", encoded},
		{"escaped slash", "/yaxunit/api/a%2Fb", "/yaxunit/api/a%2Fb"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := GetNormalPath(testCase.path)
			if got != testCase.want {
				t.Fatalf("GetNormalPath(%q) = %q, want %q", testCase.path, got, testCase.want)
			}
			if again := GetNormalPath(got); again != got {
				t.Fatalf("second normalization changed %q to %q", got, again)
			}
		})
	}
}
// FIX-07 TEST END.