package handler

import (
	json "encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"k8s.io/kube-openapi/pkg/validation/spec"
)

var returnedSwagger = []byte(`{
  "swagger": "2.0",
  "info": {
   "title": "Kubernetes",
   "version": "v1.11.0"
  }}`)

func TestRegisterOpenAPIVersionedService(t *testing.T) {
	var s spec.Swagger
	err := s.UnmarshalJSON(returnedSwagger)
	if err != nil {
		t.Errorf("Unexpected error in unmarshalling SwaggerJSON: %v", err)
	}

	returnedJSON, err := json.Marshal(s)
	if err != nil {
		t.Errorf("Unexpected error in preparing returnedJSON: %v", err)
	}
	var decodedJSON map[string]interface{}
	if err := json.Unmarshal(returnedJSON, &decodedJSON); err != nil {
		t.Fatal(err)
	}
	returnedPb, err := ToProtoBinary(returnedJSON)
	if err != nil {
		t.Errorf("Unexpected error in preparing returnedPb: %v", err)
	}

	mux := http.NewServeMux()
	o, err := NewOpenAPIService(&s)
	if err != nil {
		t.Fatal(err)
	}
	if err = o.RegisterOpenAPIVersionedService("/openapi/v2", mux); err != nil {
		t.Errorf("Unexpected error in register OpenAPI versioned service: %v", err)
	}
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	tcs := []struct {
		acceptHeader              string
		respStatus                int
		responseContentTypeHeader string
		respBody                  []byte
	}{
		{"", 200, "application/json", returnedJSON},
		{"*/*", 200, "application/json", returnedJSON},
		{"application/*", 200, "application/json", returnedJSON},
		{"application/json", 200, "application/json", returnedJSON},
		{"test/test", 406, "", []byte{}},
		{"application/test", 406, "", []byte{}},
		{"application/test, */*", 200, "application/json", returnedJSON},
		{"application/test, application/json", 200, "application/json", returnedJSON},
		{"application/com.github.proto-openapi.spec.v2.v1.0+protobuf", 200, "application/com.github.proto-openapi.spec.v2.v1.0+protobuf", returnedPb},
		{"application/json, application/com.github.proto-openapi.spec.v2.v1.0+protobuf", 200, "application/json", returnedJSON},
		{"application/com.github.proto-openapi.spec.v2.v1.0+protobuf, application/json", 200, "application/com.github.proto-openapi.spec.v2.v1.0+protobuf", returnedPb},
		{"application/com.github.proto-openapi.spec.v2.v1.0+protobuf; q=0.5, application/json", 200, "application/json", returnedJSON},
		{"application/com.github.proto-openapi.spec.v2@v1.0+protobuf", 200, "application/com.github.proto-openapi.spec.v2.v1.0+protobuf", returnedPb},
		{"application/json, application/com.github.proto-openapi.spec.v2@v1.0+protobuf", 200, "application/json", returnedJSON},
		{"application/com.github.proto-openapi.spec.v2@v1.0+protobuf, application/json", 200, "application/com.github.proto-openapi.spec.v2.v1.0+protobuf", returnedPb},
		{"application/com.github.proto-openapi.spec.v2@v1.0+protobuf; q=0.5, application/json", 200, "application/json", returnedJSON},
	}

	for _, tc := range tcs {
		req, err := http.NewRequest("GET", server.URL+"/openapi/v2", nil)
		if err != nil {
			t.Errorf("Accept: %v: Unexpected error in creating new request: %v", tc.acceptHeader, err)
		}

		req.Header.Add("Accept", tc.acceptHeader)
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("Accept: %v: Unexpected error in serving HTTP request: %v", tc.acceptHeader, err)
		}

		if resp.StatusCode != tc.respStatus {
			t.Errorf("Accept: %v: Unexpected response status code, want: %v, got: %v", tc.acceptHeader, tc.respStatus, resp.StatusCode)
		}
		if tc.respStatus != 200 {
			continue
		}

		responseContentType := resp.Header.Get("Content-Type")
		if responseContentType != tc.responseContentTypeHeader {
			t.Errorf("Accept: %v: Unexpected content type in response, want: %v, got: %v", tc.acceptHeader, tc.responseContentTypeHeader, responseContentType)
		}

		_, _, err = mime.ParseMediaType(responseContentType)
		if err != nil {
			t.Errorf("Unexpected error in prarsing response content type: %v, err: %v", responseContentType, err)
		}

		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("Accept: %v: Unexpected error in reading response body: %v", tc.acceptHeader, err)
		}
		if !reflect.DeepEqual(body, tc.respBody) {
			t.Errorf("Accept: %v: Response body mismatches, \nwant: %s, \ngot:  %s", tc.acceptHeader, string(tc.respBody), string(body))
		}
	}
}

func TestToProtoBinary(t *testing.T) {
	bs, err := os.ReadFile("../../test/integration/testdata/aggregator/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ToProtoBinary(bs); err != nil {
		t.Fatal()
	}
	// TODO: add some kind of roundtrip test here
}
