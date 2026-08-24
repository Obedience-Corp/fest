package shared

import (
	"reflect"
	"testing"
)

func TestWorkingFestivalPickerStatuses(t *testing.T) {
	want := []string{"active", "ready", "planning", "parked"}
	if !reflect.DeepEqual(WorkingFestivalPickerStatuses, want) {
		t.Fatalf("WorkingFestivalPickerStatuses = %#v, want %#v", WorkingFestivalPickerStatuses, want)
	}
}

func TestBrowseFestivalPickerStatusesExtendsWorkingSet(t *testing.T) {
	if len(BrowseFestivalPickerStatuses) != len(WorkingFestivalPickerStatuses)+1 {
		t.Fatalf("BrowseFestivalPickerStatuses = %#v, want working set plus ritual", BrowseFestivalPickerStatuses)
	}
	for i, status := range WorkingFestivalPickerStatuses {
		if BrowseFestivalPickerStatuses[i] != status {
			t.Fatalf("BrowseFestivalPickerStatuses[%d] = %q, want %q", i, BrowseFestivalPickerStatuses[i], status)
		}
	}
	if BrowseFestivalPickerStatuses[len(WorkingFestivalPickerStatuses)] != "ritual" {
		t.Fatalf("BrowseFestivalPickerStatuses ritual suffix = %q, want ritual", BrowseFestivalPickerStatuses[len(WorkingFestivalPickerStatuses)])
	}
}
