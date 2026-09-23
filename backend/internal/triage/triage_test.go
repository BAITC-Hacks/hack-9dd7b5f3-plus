package triage

import "testing"

func TestSpokenToDigits(t *testing.T) {
	cases := map[string]string{
		"телефон плюс семь семьсот семь, сто двадцать три, сорок пять, шестьдесят семь.": "+77071234567",
		"Плюс жеті, жеті жүз бір, нөл нөл нөл, нөл нөл, нөл екі.":                        "+77010000002",
		"Номер не помню, телефон плюс 7 701 000 00 07.":                                  "+77010000007",
		"8 701 555 12 34.": "+77015551234",
		"Телефон: плюс жеті, жеті жүз бір, нөл нөл нөл, нөл нөл, он.": "+77010000010",
	}
	for in, want := range cases {
		s := Analyze(in, false)
		if s.Entities["phone"] != want {
			t.Errorf("phone from %q = %q (norm %q), want %q", in, s.Entities["phone"], s.Normalized, want)
		}
	}
}

func TestEntities(t *testing.T) {
	s := Analyze("Мой 910512300456, жены 930824400789.", false)
	if s.Entities["iin"] == "" {
		t.Errorf("iin not found: %v", s.Entities)
	}
	s = Analyze("Номер 482KMA02, номер заявления CL-500311, полис SQ-OGPO-104501", false)
	if s.Entities["vehicle_plate"] != "482KMA02" || s.Entities["claim_number"] != "CL-500311" || s.Entities["policy_number"] != "SQ-OGPO-104501" {
		t.Errorf("entities: %v", s.Entities)
	}
	s = Analyze("Кеше аулада көлігімді біреу соғып кетіпті, КАСКО бар, и ещё подскажите, где у вас осмотр делают", false)
	if len(s.MultiIntentMarkers) == 0 || s.Language != "mixed" {
		t.Errorf("multi-intent/lang: %+v", s)
	}
	s = Analyze("Иә, растаймын.", true)
	if s.Confirmation != "yes" {
		t.Errorf("confirmation: %+v", s)
	}
	s = Analyze("Нет, не надо.", true)
	if s.Confirmation != "no" {
		t.Errorf("confirmation no: %+v", s)
	}
	s = Analyze("Алло, я по поводу страховки", false)
	if s.GreetingOnly {
		t.Errorf("should not be greeting only")
	}
	s = Analyze("Здравствуйте!", false)
	if !s.GreetingOnly {
		t.Errorf("should be greeting only")
	}
}
