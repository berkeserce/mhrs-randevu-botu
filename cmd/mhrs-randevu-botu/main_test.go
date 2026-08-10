package main

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/berkeserce/mhrs-randevu-botu/internal/mhrs"
)

func TestPromptInteractiveConfig(t *testing.T) {
	input := bufio.NewReader(strings.NewReader("34\n5\n2\n10\n"))
	var output bytes.Buffer
	config, err := promptInteractiveConfig(input, &output)
	if err != nil {
		t.Fatal(err)
	}
	if config.cityID != 34 || config.days != 5 || config.once || config.interval != 10*time.Minute {
		t.Fatalf("config = %#v", config)
	}
}

func TestTurkishDateAndDurationFormatting(t *testing.T) {
	value := time.Date(2026, time.August, 10, 17, 13, 59, 0, time.UTC)
	if got := formatTurkishDateTime(value, true); got != "10 Ağustos 2026 20:13:59 TSİ" {
		t.Fatalf("date = %q", got)
	}
	if got := formatDurationTurkish(5 * time.Minute); got != "5 dakika" {
		t.Fatalf("duration = %q", got)
	}
	if got := formatDurationTurkish(90 * time.Minute); got != "1 saat 30 dakika" {
		t.Fatalf("duration = %q", got)
	}
}

func TestStyledOutputCanBeDisabled(t *testing.T) {
	colorOutput = false
	if got := styled(ansiGreen, "mesaj"); got != "mesaj" {
		t.Fatalf("styled = %q", got)
	}
}

func TestReadPromptColorsUserInputAndClinicPromptRed(t *testing.T) {
	previousColorOutput := colorOutput
	colorOutput = true
	t.Cleanup(func() { colorOutput = previousColorOutput })

	var output bytes.Buffer
	value, err := readPrompt(bufio.NewReader(strings.NewReader("cildiye\n")), &output, "Poliklinik ara: ")
	if err != nil {
		t.Fatal(err)
	}
	if value != "cildiye" {
		t.Fatalf("value = %q", value)
	}
	want := styled(ansiBoldRed, "Poliklinik ara: ") + ansiRed + ansiReset
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestPromptInteractiveConfigDefaults(t *testing.T) {
	input := bufio.NewReader(strings.NewReader("34\n\n\n\n"))
	var output bytes.Buffer
	config, err := promptInteractiveConfig(input, &output)
	if err != nil {
		t.Fatal(err)
	}
	if config.cityID != 34 || config.days != 3 || config.once || config.interval != 5*time.Minute {
		t.Fatalf("config = %#v", config)
	}
}

func TestPromptInteractiveConfigSingleCheck(t *testing.T) {
	input := bufio.NewReader(strings.NewReader("34\n7\n1\n"))
	var output bytes.Buffer
	config, err := promptInteractiveConfig(input, &output)
	if err != nil {
		t.Fatal(err)
	}
	if !config.once || config.days != 7 {
		t.Fatalf("config = %#v", config)
	}
}

func TestPromptInteractiveConfigRetriesInvalidAnswers(t *testing.T) {
	input := bufio.NewReader(strings.NewReader("0\n34\n17\n5\n3\n2\n0\n10\n"))
	var output bytes.Buffer
	config, err := promptInteractiveConfig(input, &output)
	if err != nil {
		t.Fatal(err)
	}
	if config.cityID != 34 || config.days != 5 || config.once || config.interval != 10*time.Minute {
		t.Fatalf("config = %#v", config)
	}
	if !strings.Contains(output.String(), "Lutfen") {
		t.Fatalf("retry warning missing from output: %q", output.String())
	}
}

func TestValidateSearchCriteria(t *testing.T) {
	valid := mhrs.SearchCriteria{CityID: 34, Gender: "F"}
	if err := validateSearchCriteria(valid, 5*time.Minute, false, 3); err != nil {
		t.Fatal(err)
	}
	if err := validateSearchCriteria(valid, 30*time.Second, false, 3); err == nil {
		t.Fatal("sub-minute polling should be rejected")
	}
	if err := validateSearchCriteria(valid, 0, true, 3); err != nil {
		t.Fatalf("single check should not require interval: %v", err)
	}
	if err := validateSearchCriteria(valid, 5*time.Minute, false, 16); err != nil {
		t.Fatalf("16-day window should be accepted: %v", err)
	}
	for _, days := range []int{0, 17} {
		if err := validateSearchCriteria(valid, 5*time.Minute, false, days); err == nil {
			t.Fatalf("days %d should be rejected", days)
		}
	}
}

func TestFilterAvailabilityByWindow(t *testing.T) {
	location := time.FixedZone("TRT", 3*60*60)
	now := time.Date(2026, 8, 10, 16, 0, 0, 0, location)
	deadline := now.Add(72 * time.Hour)
	items := []mhrs.Availability{
		{DoctorName: "inside", StartTime: "12.08.2026 09:20"},
		{DoctorName: "boundary", StartTime: "13.08.2026 16:00"},
		{DoctorName: "too-late", StartTime: "26.08.2026 09:20"},
		{DoctorName: "invalid", StartTime: "bilinmiyor"},
	}

	got := filterAvailabilityByWindow(items, now, deadline)
	if len(got) != 2 || got[0].DoctorName != "inside" || got[1].DoctorName != "boundary" {
		t.Fatalf("items = %#v", got)
	}

	got = filterAvailabilityByWindow(items, now, now.Add(16*24*time.Hour))
	if len(got) != 3 || got[2].DoctorName != "too-late" {
		t.Fatalf("16-day items = %#v", got)
	}
}

func TestTelegramAppointmentMessageLimitsDetails(t *testing.T) {
	items := make([]mhrs.Availability, 6)
	for index := range items {
		items[index] = mhrs.Availability{
			InstitutionName: fmt.Sprintf("Hastane %d", index+1),
			ClinicName:      "Cildiye",
			DoctorName:      "Test Hekimi",
			ExaminationName: "Test Poliklinigi",
			StartTime:       "12.08.2026 09:20",
		}
	}
	message := telegramAppointmentMessage(items, time.Date(2026, 8, 10, 17, 0, 0, 0, turkeyLocation))
	for _, expected := range []string{"MHRS RANDEVUSU BULUNDU", "Hastane 1", "5. secenek", "1 secenek daha var", "uygulama randevu almaz"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message missing %q: %s", expected, message)
		}
	}
	if strings.Contains(message, "Hastane 6") {
		t.Fatalf("message should omit sixth appointment details: %s", message)
	}
}

func TestFilterClinicsHandlesTurkishI(t *testing.T) {
	clinics := []mhrs.ClinicOption{{ID: 1, Name: "\u0130\u00e7 Hastal\u0131klar\u0131"}, {ID: 2, Name: "Kardiyoloji"}}
	matches := filterClinics(clinics, "i\u00e7 hastal\u0131klar\u0131")
	if len(matches) != 1 || matches[0].ID != 1 {
		t.Fatalf("matches = %#v", matches)
	}
}

func TestFilterInstitutionsAndDoctors(t *testing.T) {
	institutions := []mhrs.InstitutionOption{
		{ID: 1, Name: "Atatürk Devlet Hastanesi"},
		{ID: 2, Name: "Şehir Hastanesi"},
	}
	if got := filterInstitutions(institutions, "devlet"); len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("institutions = %#v", got)
	}

	doctors := []mhrs.DoctorOption{
		{ID: 10, Name: "Ayşe IŞIK"},
		{ID: 11, Name: "Mehmet Yılmaz"},
	}
	if got := filterDoctors(doctors, "ışık"); len(got) != 1 || got[0].ID != 10 {
		t.Fatalf("doctors = %#v", got)
	}
	if got := filterDoctors(doctors, "yilmaz"); len(got) != 1 || got[0].ID != 11 {
		t.Fatalf("doctors = %#v", got)
	}
}
