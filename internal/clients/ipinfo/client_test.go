package ipinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupReturnsIPInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodGet, request.Method)
		assert.Equal(t, "application/json", request.Header.Get("Accept"))
		_, _ = writer.Write([]byte(`{"ip":"194.126.177.60","city":"Frankfurt am Main","region":"Hesse","country":"de","loc":"50.1155,8.6842","org":"AS209103 Proton AG","postal":"60306","timezone":"Europe/Berlin"}`))
	}))
	defer server.Close()

	info, err := newClient(server.Client(), server.URL).Lookup(context.Background())

	require.NoError(t, err)
	assert.Equal(t, Info{
		IP: "194.126.177.60", City: "Frankfurt am Main", Region: "Hesse", Country: "DE",
		Location: "50.1155,8.6842", Org: "AS209103 Proton AG", Postal: "60306", Timezone: "Europe/Berlin",
	}, info)
}

func TestLookupRejectsResponseWithoutCountry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"ip":"194.126.177.60"}`))
	}))
	defer server.Close()

	_, err := newClient(server.Client(), server.URL).Lookup(context.Background())

	assert.ErrorContains(t, err, "country code")
}
