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
