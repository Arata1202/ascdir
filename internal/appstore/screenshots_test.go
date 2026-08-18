package appstore

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchScreenshotsDownloadsConfiguredAssets(t *testing.T) {
	t.Parallel()
	image := []byte("png bytes")
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
			writeData(t, w, []any{resourceJSON("set-1", "appScreenshotSets", map[string]any{"screenshotDisplayType": "APP_IPHONE_67"})})
		case "/v1/appScreenshotSets/set-1/appScreenshots":
			writeData(t, w, []any{resourceJSON("shot-1", "appScreenshots", map[string]any{
				"fileName": "01.png", "sourceFileChecksum": "checksum",
				"imageAsset": map[string]any{"templateUrl": server.URL + "/image-{w}x{h}.{f}", "width": 1, "height": 1},
			})})
		case "/image-1x1.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(image)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	client.httpClient = server.Client()
	metadata := Metadata{Localizations: map[string]Localization{"en-US": {VersionLocalizationID: "loc-1"}}, Screenshots: map[string]map[string][]Asset{}, ScreenshotSetIDs: map[string]map[string]string{}}
	if err := client.fetchScreenshots(context.Background(), &metadata, true); err != nil {
		t.Fatal(err)
	}
	assets := metadata.Screenshots["en-US"]["APP_IPHONE_67"]
	if metadata.ScreenshotSetIDs["en-US"]["APP_IPHONE_67"] != "set-1" || len(assets) != 1 || string(assets[0].Content) != string(image) {
		t.Fatalf("screenshots = %#v", metadata.Screenshots)
	}
}

func TestUploadScreenshotCommitsThenOrdersAsset(t *testing.T) {
	t.Parallel()
	content := []byte("asset content")
	sum := md5.Sum(content)
	checksum := hex.EncodeToString(sum[:])
	var calls []string
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/appScreenshots":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("shot-new", "appScreenshots", map[string]any{
				"uploadOperations": []any{map[string]any{"method": "PUT", "url": server.URL + "/upload", "offset": 0, "length": len(content), "requestHeaders": []any{}}},
			})})
		case r.Method == http.MethodPut && r.URL.Path == "/upload":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/appScreenshots/shot-new":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	client.httpClient = server.Client()
	change := AssetSetChange{Kind: "screenshots", Locale: "en-US", DisplayType: "APP_IPHONE_67", SetID: "set-1", After: []Asset{{FileName: "01.png", Checksum: checksum, Content: content}}}
	if err := client.applyScreenshotSet(context.Background(), change); err != nil {
		t.Fatal(err)
	}
	want := []string{"POST /v1/appScreenshots", "PUT /upload", "PATCH /v1/appScreenshots/shot-new", "PATCH /v1/appScreenshotSets/set-1/relationships/appScreenshots"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %#v", calls)
	}
	for index := range want {
		if calls[index] != want[index] {
			t.Fatalf("calls = %#v", calls)
		}
	}
}

func TestValidatedUploadOperationsRequiresExactCoverage(t *testing.T) {
	t.Parallel()
	operation := func(offset, length float64) map[string]any {
		return map[string]any{"method": "PUT", "url": "https://upload.example.test/asset", "offset": offset, "length": length}
	}
	if _, err := validatedUploadOperations([]any{operation(0, 4), operation(4, 6)}, 10); err != nil {
		t.Fatal(err)
	}
	for _, operations := range []any{
		nil,
		[]any{},
		[]any{operation(0, 4), operation(5, 5)},
		[]any{operation(0, 6), operation(5, 5)},
	} {
		if _, err := validatedUploadOperations(operations, 10); err == nil {
			t.Fatalf("expected validation error for %#v", operations)
		}
	}
}

func TestUploadScreenshotCleansReservationAfterUploadFailure(t *testing.T) {
	t.Parallel()
	content := []byte("asset content")
	sum := md5.Sum(content)
	checksum := hex.EncodeToString(sum[:])
	deleted := false
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/appScreenshots":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("shot-reserved", "appScreenshots", map[string]any{
				"uploadOperations": []any{map[string]any{"method": "PUT", "url": server.URL + "/upload", "offset": 0, "length": len(content), "requestHeaders": []any{}}},
			})})
		case r.Method == http.MethodPut && r.URL.Path == "/upload":
			http.Error(w, "upload failed", http.StatusInternalServerError)
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/appScreenshots/shot-reserved":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	client.httpClient = server.Client()
	_, err := client.uploadScreenshot(context.Background(), "set-1", Asset{FileName: "01.png", Checksum: checksum, Content: content})
	if err == nil {
		t.Fatal("upload unexpectedly succeeded")
	}
	if !deleted {
		t.Fatal("reserved screenshot was not deleted")
	}
}
