package proxy

import "testing"

func TestParseIPInfoPayloadWrappedSuccess(t *testing.T) {
	t.Parallel()

	body := []byte(`{"success":true,"data":{"ip":"1.2.3.4","country":"X","country_code":"XC"}}`)
	data, err := parseIPInfoPayload(body)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if data["ip"] != "1.2.3.4" {
		t.Fatalf("unexpected ip: %v", data["ip"])
	}
	if data["country_code"] != "XC" {
		t.Fatalf("unexpected country_code: %v", data["country_code"])
	}
}

func TestParseIPInfoPayloadDirectObject(t *testing.T) {
	t.Parallel()

	body := []byte(`{"ip":"194.55.15.221","country":"德国","countryCode":"DE","isp":"Foo ISP"}`)
	data, err := parseIPInfoPayload(body)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if data["country_code"] != "DE" {
		t.Fatalf("expected country_code alias to be normalized, got=%v", data["country_code"])
	}
}

func TestParseIPInfoPayloadWrappedFailure(t *testing.T) {
	t.Parallel()

	body := []byte(`{"success":false,"message":"quota exceeded"}`)
	_, err := parseIPInfoPayload(body)
	if err == nil {
		t.Fatalf("expected error for failure payload")
	}
}

func TestParseIPTimezonePayloadSuccess(t *testing.T) {
	t.Parallel()

	body := []byte(`{"status":"success","countryCode":"US","timezone":"America/Los_Angeles"}`)
	timezone, countryCode, err := parseIPTimezonePayload(body)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if timezone != "America/Los_Angeles" {
		t.Fatalf("unexpected timezone: %q", timezone)
	}
	if countryCode != "US" {
		t.Fatalf("unexpected countryCode: %q", countryCode)
	}
}

func TestParseIPTimezonePayloadFailure(t *testing.T) {
	t.Parallel()

	body := []byte(`{"status":"fail","message":"invalid query"}`)
	_, _, err := parseIPTimezonePayload(body)
	if err == nil {
		t.Fatalf("expected error for failed timezone lookup payload")
	}
}

func TestParseIPTimezonePayloadMissingTimezone(t *testing.T) {
	t.Parallel()

	body := []byte(`{"status":"success","countryCode":"DE","timezone":""}`)
	_, countryCode, err := parseIPTimezonePayload(body)
	if err == nil {
		t.Fatalf("expected error when timezone is missing")
	}
	if countryCode != "DE" {
		t.Fatalf("unexpected countryCode: %q", countryCode)
	}
}
