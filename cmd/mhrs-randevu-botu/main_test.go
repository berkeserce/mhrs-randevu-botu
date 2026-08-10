package main

import (
	"testing"
	"time"

	"github.com/berkeserce/mhrs-randevu-botu/internal/mhrs"
)

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

func TestFilterClinicsHandlesTurkishI(t *testing.T) {
	clinics := []mhrs.ClinicOption{{ID: 1, Name: "İç Hastalıkları"}, {ID: 2, Name: "Kardiyoloji"}}
	matches := filterClinics(clinics, "iç hastalıkları")
	if len(matches) != 1 || matches[0].ID != 1 {
		t.Fatalf("matches = %#v", matches)
	}
}
