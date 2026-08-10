package mhrs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	ProductionBaseURL = "https://prd.mhrs.gov.tr/api/"
)

var ErrSessionExpired = errors.New("MHRS oturumu sona ermis")

type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

func NewAuthenticatedClient(token string) *Client {
	client := newClient(ProductionBaseURL, &http.Client{Timeout: 20 * time.Second})
	client.token = strings.TrimSpace(token)
	return client
}

func newClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/") + "/",
		httpClient: httpClient,
	}
}

type SearchCriteria struct {
	ActionID      int64
	ActionIDs     []int64
	Gender        string
	DoctorID      int64
	CityID        int64
	DistrictID    int64
	ClinicID      int64
	InstitutionID int64
	ExaminationID int64
}

type ClinicOption struct {
	ID   int64
	Name string
}

type Availability struct {
	DoctorName      string
	InstitutionName string
	ClinicName      string
	ExaminationName string
	StartTime       string
}

func (c *Client) SearchAppointments(ctx context.Context, criteria SearchCriteria) ([]Availability, error) {
	payload := struct {
		ActionID         int64   `json:"aksiyonId"`
		ActionIDs        []int64 `json:"aksiyonIdList,omitempty"`
		Gender           string  `json:"cinsiyet"`
		DoctorID         int64   `json:"mhrsHekimId"`
		CityID           int64   `json:"mhrsIlId"`
		DistrictID       int64   `json:"mhrsIlceId"`
		ClinicID         int64   `json:"mhrsKlinikId"`
		InstitutionID    int64   `json:"mhrsKurumId"`
		ExaminationID    int64   `json:"muayeneYeriId"`
		AllAppointments  bool    `json:"tumRandevular"`
		ExtraAppointment bool    `json:"ekRandevu"`
		AppointmentTimes []any   `json:"randevuZamaniList"`
	}{
		ActionID:         criteria.ActionID,
		ActionIDs:        criteria.ActionIDs,
		Gender:           criteria.Gender,
		DoctorID:         criteria.DoctorID,
		CityID:           criteria.CityID,
		DistrictID:       criteria.DistrictID,
		ClinicID:         criteria.ClinicID,
		InstitutionID:    criteria.InstitutionID,
		ExaminationID:    criteria.ExaminationID,
		AllAppointments:  false,
		ExtraAppointment: true,
		AppointmentTimes: []any{},
	}

	var data struct {
		Hospitals []struct {
			Doctor struct {
				FirstName string `json:"ad"`
				LastName  string `json:"soyad"`
			} `json:"hekim"`
			Examination struct {
				Name string `json:"adi"`
			} `json:"muayeneYeri"`
			Clinic struct {
				Name string `json:"mhrsKlinikAdi"`
			} `json:"klinik"`
			Institution struct {
				Name string `json:"kurumAdi"`
			} `json:"kurum"`
			StartTime json.RawMessage `json:"baslangicZamaniStr"`
		} `json:"hastane"`
	}
	if err := c.post(ctx, "kurum-rss/randevu/slot-sorgulama/arama", payload, &data); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if apiErr.HasCode("RND4010") || apiErr.HasCode("RND4030") {
				return nil, nil
			}
			if apiErr.HasCode("LGN1004") || apiErr.HasCode("LGN2001") || apiErr.Status == http.StatusUnauthorized {
				return nil, fmt.Errorf("%w: %v", ErrSessionExpired, err)
			}
		}
		return nil, err
	}

	result := make([]Availability, 0, len(data.Hospitals))
	for _, hospital := range data.Hospitals {
		result = append(result, Availability{
			DoctorName:      strings.TrimSpace(hospital.Doctor.FirstName + " " + hospital.Doctor.LastName),
			InstitutionName: hospital.Institution.Name,
			ClinicName:      hospital.Clinic.Name,
			ExaminationName: hospital.Examination.Name,
			StartTime:       appointmentStartTime(hospital.StartTime),
		})
	}
	return result, nil
}

func (c *Client) ListClinics(ctx context.Context, criteria SearchCriteria) ([]ClinicOption, error) {
	path := fmt.Sprintf(
		"kurum/kurum/kurum-klinik/il/%d/ilce/%d/kurum/%d/aksiyon/%d/randevuTuru/-1/select-input",
		criteria.CityID, criteria.DistrictID, criteria.InstitutionID, criteria.ActionID,
	)
	var tree []selectOption
	if err := c.get(ctx, path, &tree); err != nil {
		return nil, err
	}

	clinics := make([]ClinicOption, 0, len(tree))
	seen := make(map[int64]bool)
	flattenClinicOptions(tree, &clinics, seen)
	return clinics, nil
}

type selectOption struct {
	Text     string          `json:"text"`
	Value    json.RawMessage `json:"value"`
	Children []selectOption  `json:"children"`
}

func flattenClinicOptions(options []selectOption, result *[]ClinicOption, seen map[int64]bool) {
	for _, option := range options {
		id, ok := selectOptionID(option.Value)
		name := strings.TrimSpace(option.Text)
		if ok && id > 0 && name != "" && !seen[id] {
			*result = append(*result, ClinicOption{ID: id, Name: name})
			seen[id] = true
		}
		flattenClinicOptions(option.Children, result, seen)
	}
}

func selectOptionID(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var number int64
	if json.Unmarshal(raw, &number) == nil {
		return number, true
	}
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return 0, false
	}
	if _, err := fmt.Sscan(strings.TrimSpace(text), &number); err != nil {
		return 0, false
	}
	return number, true
}

func appointmentStartTime(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var value struct {
		Time string `json:"zaman"`
	}
	if json.Unmarshal(raw, &value) == nil {
		return value.Time
	}
	return ""
}

type apiMessage struct {
	Code    string `json:"kodu"`
	Message string `json:"mesaj"`
}

type APIError struct {
	Status   int
	Messages []apiMessage
}

func (e *APIError) Error() string {
	parts := make([]string, 0, len(e.Messages))
	for _, message := range e.Messages {
		text := strings.TrimSpace(message.Message)
		if message.Code != "" && text != "" {
			parts = append(parts, message.Code+": "+text)
		} else if message.Code != "" {
			parts = append(parts, message.Code)
		} else if text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("MHRS istegi reddetti (HTTP %d)", e.Status)
	}
	return fmt.Sprintf("MHRS istegi reddetti (HTTP %d): %s", e.Status, strings.Join(parts, "; "))
}

func (e *APIError) HasCode(code string) bool {
	for _, message := range e.Messages {
		if message.Code == code {
			return true
		}
	}
	return false
}

type envelope struct {
	Success  bool            `json:"success"`
	Data     json.RawMessage `json:"data"`
	Errors   []apiMessage    `json:"errors"`
	Warnings []apiMessage    `json:"warnings"`
	Infos    []apiMessage    `json:"infos"`
}

func (c *Client) post(ctx context.Context, path string, payload, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("istek hazirlanamadi: %w", err)
	}
	return c.request(ctx, http.MethodPost, path, bytes.NewReader(body), result)
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	return c.request(ctx, http.MethodGet, path, nil, result)
}

func (c *Client) request(ctx context.Context, method, path string, body io.Reader, result any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("istek olusturulamadi: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9")
	req.Header.Set("User-Agent", "mhrs-randevu-botu/0.1 (+local interactive client)")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("MHRS istegi basarisiz: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, 1<<20)
	var env envelope
	if err := json.NewDecoder(limited).Decode(&env); err != nil {
		return fmt.Errorf("MHRS gecersiz yanit verdi (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !env.Success {
		err := apiError(resp.StatusCode, env)
		if c.token != "" && (resp.StatusCode == http.StatusUnauthorized || err.HasCode("LGN1004") || err.HasCode("LGN2001")) {
			return fmt.Errorf("%w: %v", ErrSessionExpired, err)
		}
		return err
	}
	if result != nil && len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, result); err != nil {
			return errors.New("MHRS basarili ancak beklenmeyen veri bicimi dondurdu")
		}
	}
	return nil
}

func apiError(status int, env envelope) *APIError {
	messages := append(append([]apiMessage{}, env.Errors...), env.Warnings...)
	return &APIError{Status: status, Messages: messages}
}
