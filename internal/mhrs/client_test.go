package mhrs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchAppointments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/kurum-rss/randevu/slot-sorgulama/arama" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer jwt-value" {
			t.Fatalf("authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]any{
			"mhrsIlId": float64(34), "mhrsKlinikId": float64(1001), "aksiyonId": float64(200),
		} {
			if got := body[key]; got != want {
				t.Fatalf("%s = %v, want %v", key, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"hastane":[{"hekim":{"ad":"Ada","soyad":"Hekim"},"kurum":{"kurumAdi":"Test Hastanesi"},"klinik":{"mhrsKlinikAdi":"Test Klinigi"},"muayeneYeri":{"adi":"Oda 1"},"baslangicZamaniStr":{"zaman":"12.08.2026 09:30"}}]}}`))
	}))
	defer server.Close()

	client := newClient(server.URL, server.Client())
	client.token = "jwt-value"
	items, err := client.SearchAppointments(context.Background(), SearchCriteria{
		ActionID: 200, Gender: "F", DoctorID: -1, CityID: 34, DistrictID: -1,
		ClinicID: 1001, InstitutionID: -1, ExaminationID: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].DoctorName != "Ada Hekim" || items[0].StartTime != "12.08.2026 09:30" {
		t.Fatalf("items = %#v", items)
	}
}

func TestListClinicsUsesDependentMHRSCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/kurum/kurum/kurum-klinik/il/34/ilce/-1/kurum/-1/aksiyon/200/randevuTuru/-1/select-input"
		if r.Method != http.MethodGet || r.URL.Path != wantPath {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[{"text":"Test Grubu","value":"10","children":[{"text":"Test Klinigi","value":"1001"}]}]}`))
	}))
	defer server.Close()

	client := newClient(server.URL, server.Client())
	client.token = "jwt-value"
	clinics, err := client.ListClinics(context.Background(), SearchCriteria{
		ActionID: 200, CityID: 34, DistrictID: -1, InstitutionID: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(clinics) != 2 || clinics[1].ID != 1001 || clinics[1].Name != "Test Klinigi" {
		t.Fatalf("clinics = %#v", clinics)
	}
}

func TestSearchAppointmentsTreatsNoSlotAsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"kodu":"RND4010","mesaj":"Randevu yok"}]}`))
	}))
	defer server.Close()
	client := newClient(server.URL, server.Client())
	items, err := client.SearchAppointments(context.Background(), SearchCriteria{})
	if err != nil || len(items) != 0 {
		t.Fatalf("items = %#v, error = %v", items, err)
	}
}

func TestAuthenticatedRequestReportsExpiredSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"kodu":"LGN1004","mesaj":"Oturum gecersiz"}]}`))
	}))
	defer server.Close()
	client := newClient(server.URL, server.Client())
	client.token = "expired"
	_, err := client.ListClinics(context.Background(), SearchCriteria{})
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("error = %v, want ErrSessionExpired", err)
	}
}
