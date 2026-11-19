package driver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloud-phone/internal/model"
)

func testGateway(t *testing.T, handler http.HandlerFunc) *Gateway {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewGateway(func(context.Context) (model.Settings, error) {
		return model.Settings{ProviderBaseURL: srv.URL, ProviderToken: "gateway-token"}, nil
	}, "", "")
}

func TestGatewayListDevices(t *testing.T) {
	g := testGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/phones" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer gateway-token" {
			t.Errorf("authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"phones":[
			{"external_id":"USB-1","name":"QQ-01","region":"本地","image":"Android 13","state":"online","ip":""}
		]}`))
	})
	devices, err := g.ListDevices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].ExternalID != "USB-1" || devices[0].State != DeviceOnline {
		t.Errorf("devices = %+v", devices)
	}
}

func TestGatewayListDevicesRejectsEmptyID(t *testing.T) {
	g := testGateway(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"phones":[{"name":"bad"}]}`))
	})
	_, err := g.ListDevices(context.Background())
	if !errors.Is(err, ErrProvider) {
		t.Fatalf("err = %v, want ErrProvider", err)
	}
}

func TestGatewayListDevicesUnsupported(t *testing.T) {
	g := testGateway(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, err := g.ListDevices(context.Background())
	if !errors.Is(err, ErrProvider) || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want provider 404", err)
	}
}

func TestGatewayExecuteManual(t *testing.T) {
	g := testGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/login/execute" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"manual":true,"message":"请使用 scrcpy"}`))
	})
	res, err := g.Execute(context.Background(), LoginRequest{
		Account: model.Account{ID: 1, Number: "10001"},
		Phone:   model.CloudPhone{ExternalID: "USB-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !res.Manual || res.Code != "" {
		t.Errorf("result = %+v", res)
	}
}
