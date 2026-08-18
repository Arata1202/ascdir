package appstore

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestFetchAppPreviewsDownloadsVideoAndFrameTime(t *testing.T) {
	t.Parallel()
	video := []byte("video bytes")
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/appStoreVersionLocalizations/loc-1/appPreviewSets":
			writeData(t, w, []any{resourceJSON("set-1", "appPreviewSets", map[string]any{"previewType": "IPHONE_67"})})
		case "/v1/appPreviewSets/set-1/appPreviews":
			writeData(t, w, []any{resourceJSON("preview-1", "appPreviews", map[string]any{
				"fileName": "01.mp4", "sourceFileChecksum": "abc", "mimeType": "video/mp4",
				"previewFrameTimeCode": "00:00:05", "videoUrl": server.URL + "/video",
			})})
		case "/video":
			_, _ = w.Write(video)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	client.httpClient = server.Client()
	metadata := Metadata{Localizations: map[string]Localization{"en-US": {VersionLocalizationID: "loc-1"}}, AppPreviews: map[string]map[string][]Asset{}, AppPreviewSetIDs: map[string]map[string]string{}}
	if err := client.fetchAppPreviews(context.Background(), &metadata, true); err != nil {
		t.Fatal(err)
	}
	asset := metadata.AppPreviews["en-US"]["IPHONE_67"][0]
	defer os.Remove(asset.Path)
	downloaded, err := os.ReadFile(asset.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(downloaded) != string(video) || asset.PreviewFrameTimeCode != "00:00:05" || metadata.AppPreviewSetIDs["en-US"]["IPHONE_67"] != "set-1" {
		t.Fatalf("asset = %#v", asset)
	}
}

func TestUploadAppPreviewCommitsVideo(t *testing.T) {
	t.Parallel()
	content := []byte("video content")
	sum := md5.Sum(content)
	checksum := hex.EncodeToString(sum[:])
	var calls []string
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/appPreviews":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("preview-new", "appPreviews", map[string]any{
				"uploadOperations": []any{
					map[string]any{
						"method": "PUT", "url": server.URL + "/upload", "offset": 0, "length": len(content),
						"requestHeaders": []any{map[string]any{"name": "Content-Type", "value": "video/mp4"}},
					},
				},
			})})
		case r.Method == http.MethodPut && r.URL.Path == "/upload":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/appPreviews/preview-new":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/appPreviewSets/set-1/relationships/appPreviews":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	client.httpClient = server.Client()
	change := AssetSetChange{Kind: "app_previews", Locale: "en-US", DisplayType: "IPHONE_67", SetID: "set-1", After: []Asset{{FileName: "01.mp4", Checksum: checksum, Content: content, MIMEType: "video/mp4", PreviewFrameTimeCode: "00:00:05"}}}
	if err := client.applyAppPreviewSet(context.Background(), change); err != nil {
		t.Fatal(err)
	}
	want := []string{"POST /v1/appPreviews", "PUT /upload", "PATCH /v1/appPreviews/preview-new", "PATCH /v1/appPreviewSets/set-1/relationships/appPreviews"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %#v", calls)
	}
	for index := range want {
		if calls[index] != want[index] {
			t.Fatalf("calls = %#v", calls)
		}
	}
}

func TestAppPreviewPosterFrameUpdateReusesVideo(t *testing.T) {
	t.Parallel()
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/v1/appPreviews/preview-1" {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	change := AssetSetChange{Kind: "app_previews", Locale: "en-US", DisplayType: "IPHONE_67", SetID: "set-1",
		Before: []Asset{{ID: "preview-1", Checksum: "abc", PreviewFrameTimeCode: "00:00:01"}},
		After:  []Asset{{Checksum: "abc", PreviewFrameTimeCode: "00:00:05"}},
	}
	if err := client.applyAppPreviewSet(context.Background(), change); err != nil {
		t.Fatal(err)
	}
	want := []string{"PATCH /v1/appPreviews/preview-1", "PATCH /v1/appPreviewSets/set-1/relationships/appPreviews"}
	if len(calls) != len(want) || calls[0] != want[0] || calls[1] != want[1] {
		t.Fatalf("calls = %#v", calls)
	}
}
