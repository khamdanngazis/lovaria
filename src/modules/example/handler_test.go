package example

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func newTestServer() *echo.Echo {
	e := echo.New()
	Register(e.Group(""), Deps{Service: NewService(NewMemoryRepository())})
	return e
}

func request(e *echo.Echo, method, path string, form url.Values) *httptest.ResponseRecorder {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestCreateAndListNotes(t *testing.T) {
	e := newTestServer()

	rec := request(e, http.MethodPost, "/weddings/7/notes", url.Values{"body": {"<b>halo</b>"}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "&lt;b&gt;halo&lt;/b&gt;") {
		t.Errorf("output harus di-escape: %s", rec.Body)
	}

	rec = request(e, http.MethodGet, "/weddings/7/notes", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "halo") {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body)
	}

	rec = request(e, http.MethodGet, "/weddings/8/notes", nil)
	if strings.Contains(rec.Body.String(), "halo") {
		t.Errorf("catatan wedding 7 bocor ke wedding 8: %s", rec.Body)
	}
}

func TestCreateNoteValidationError(t *testing.T) {
	rec := request(newTestServer(), http.MethodPost, "/weddings/7/notes", url.Values{"body": {""}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestInvalidWeddingID(t *testing.T) {
	rec := request(newTestServer(), http.MethodGet, "/weddings/abc/notes", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}
