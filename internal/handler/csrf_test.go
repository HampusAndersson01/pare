// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func csrfCookieFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == csrfCookieName {
			return c
		}
	}
	t.Fatal("no CSRF cookie issued")
	return nil
}

func TestCSRFProtect(t *testing.T) {
	s := &Server{}
	var served bool
	h := s.csrfProtect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
		if csrfToken(r) == "" {
			t.Error("token missing from context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	// GET issues a token cookie and reaches the handler.
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if getRec.Code != http.StatusOK || !served {
		t.Fatalf("GET blocked: code=%d served=%v", getRec.Code, served)
	}
	cookie := csrfCookieFrom(t, getRec)
	if cookie.SameSite != http.SameSiteStrictMode || !cookie.HttpOnly {
		t.Fatalf("weak CSRF cookie: sameSite=%v httpOnly=%v", cookie.SameSite, cookie.HttpOnly)
	}

	newPost := func(token string, withCookie bool) *http.Request {
		body := url.Values{}
		if token != "" {
			body.Set("csrf", token)
		}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if withCookie {
			req.AddCookie(cookie)
		}
		return req
	}

	// POST with matching cookie + field passes.
	served = false
	okRec := httptest.NewRecorder()
	h.ServeHTTP(okRec, newPost(cookie.Value, true))
	if okRec.Code != http.StatusOK || !served {
		t.Fatalf("valid POST rejected: code=%d", okRec.Code)
	}

	// POST with cookie but no field is forbidden.
	served = false
	noField := httptest.NewRecorder()
	h.ServeHTTP(noField, newPost("", true))
	if noField.Code != http.StatusForbidden || served {
		t.Fatalf("missing-token POST not blocked: code=%d served=%v", noField.Code, served)
	}

	// POST with a forged field but no matching cookie (the cross-site case) is
	// forbidden: the attacker cannot know the per-browser token.
	served = false
	forged := httptest.NewRecorder()
	h.ServeHTTP(forged, newPost("attacker-guess", false))
	if forged.Code != http.StatusForbidden || served {
		t.Fatalf("forged POST not blocked: code=%d served=%v", forged.Code, served)
	}

	// POST with cookie but wrong field value is forbidden.
	served = false
	mismatch := httptest.NewRecorder()
	h.ServeHTTP(mismatch, newPost("wrong", true))
	if mismatch.Code != http.StatusForbidden || served {
		t.Fatalf("mismatched-token POST not blocked: code=%d served=%v", mismatch.Code, served)
	}
}

func TestCSRFProtectMultipartUpload(t *testing.T) {
	s := &Server{}
	var served bool
	h := s.csrfProtect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
		if _, _, err := r.FormFile("file"); err != nil {
			t.Errorf("uploaded file unavailable after CSRF validation: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/kvitton", nil))
	cookie := csrfCookieFrom(t, getRec)

	newMultipartPost := func(token string) *http.Request {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		if token != "" {
			if err := mw.WriteField("csrf", token); err != nil {
				t.Fatal(err)
			}
		}
		fw, err := mw.CreateFormFile("file", "receipt.pdf")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte("%PDF-test")); err != nil {
			t.Fatal(err)
		}
		if err := mw.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/kvitton", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.AddCookie(cookie)
		return req
	}

	served = false
	okRec := httptest.NewRecorder()
	h.ServeHTTP(okRec, newMultipartPost(cookie.Value))
	if okRec.Code != http.StatusOK || !served {
		t.Fatalf("valid multipart upload rejected: code=%d served=%v", okRec.Code, served)
	}

	served = false
	missingRec := httptest.NewRecorder()
	h.ServeHTTP(missingRec, newMultipartPost(""))
	if missingRec.Code != http.StatusForbidden || served {
		t.Fatalf("multipart upload without CSRF token not blocked: code=%d served=%v", missingRec.Code, served)
	}

	served = false
	wrongRec := httptest.NewRecorder()
	h.ServeHTTP(wrongRec, newMultipartPost("wrong"))
	if wrongRec.Code != http.StatusForbidden || served {
		t.Fatalf("multipart upload with wrong CSRF token not blocked: code=%d served=%v", wrongRec.Code, served)
	}
}
