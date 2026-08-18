package appstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAndUpdateLicenseAgreement(t *testing.T) {
	t.Parallel()
	var mutation map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/relationships/endUserLicenseAgreement":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"type": "endUserLicenseAgreements", "id": "eula-1"}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/endUserLicenseAgreements/eula-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("eula-1", "endUserLicenseAgreements", map[string]any{"agreementText": "Old terms"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/endUserLicenseAgreements/eula-1/territories":
			writeData(t, w, []any{resourceJSON("USA", "territories", nil), resourceJSON("JPN", "territories", nil)})
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/endUserLicenseAgreements/eula-1":
			if err := json.NewDecoder(r.Body).Decode(&mutation); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := testClient(t, server.URL)
	remote := Metadata{AppID: "app-1", Values: map[string]string{}}
	if err := client.fetchLicenseAgreement(context.Background(), &remote); err != nil {
		t.Fatal(err)
	}
	if remote.LicenseAgreementID != "eula-1" || remote.Values["license_agreement_text"] != "Old terms" || remote.Values["license_agreement_territories"] != "JPN,USA" {
		t.Fatalf("agreement = %#v", remote)
	}
	changes := []Change{{Field: "license_agreement_text", After: "New terms"}, {Field: "license_agreement_territories", After: "USA"}}
	if err := client.ApplyMetadata(context.Background(), remote, nil, changes); err != nil {
		t.Fatal(err)
	}
	data := mutation["data"].(map[string]any)
	if data["id"] != "eula-1" || data["attributes"].(map[string]any)["agreementText"] != "New terms" {
		t.Fatalf("mutation = %#v", mutation)
	}
}

func TestCreateAndDeleteLicenseAgreement(t *testing.T) {
	t.Parallel()
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	if err := client.applyLicenseAgreementChanges(context.Background(), Metadata{AppID: "app-1", Values: map[string]string{}}, []Change{
		{Field: "license_agreement_text", After: "Terms"}, {Field: "license_agreement_territories", After: "USA"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.applyLicenseAgreementChanges(context.Background(), Metadata{LicenseAgreementID: "eula-1", Values: map[string]string{"license_agreement_text": "Terms", "license_agreement_territories": "USA"}}, []Change{
		{Field: "license_agreement_text", After: ""}, {Field: "license_agreement_territories", After: ""},
	}); err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 || methods[0] != http.MethodPost || methods[1] != http.MethodDelete {
		t.Fatalf("methods = %#v", methods)
	}
}
