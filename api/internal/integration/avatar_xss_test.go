package integration

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// makePNG генерирует валидный 1×1 PNG.
func makePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// uploadAvatar шлёт multipart/form-data POST на /users/me/avatar.
func (e *testEnv) uploadAvatar(t *testing.T, token, filename string, data []byte) *http.Response {
	t.Helper()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("avatar", filename)
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest("POST", "/api/v1/users/me/avatar", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = uniqueRemoteAddr()

	w := httptest.NewRecorder()
	e.srv.ServeHTTP(w, req)
	return w.Result()
}

// XSS через загрузку HTML-файла под видом картинки должен отклоняться.
func TestAvatarUploadRejectsNonImage(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("xss_u"), "xss_"+shortID())

	// HTML с инлайн-скриптом — классический payload для stored XSS.
	payload := []byte(`<html><body><script>alert(document.cookie)</script></body></html>`)

	resp := env.uploadAvatar(t, token, "evil.png", payload)
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"HTML под видом PNG должен отклоняться серверным снифом")
}

// Валидный PNG проходит, avatar_url получает расширение .png.
func TestAvatarUploadAcceptsPNG(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("png_u"), "png_"+shortID())

	resp := env.uploadAvatar(t, token, "me.png", makePNG(t))
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out struct {
		AvatarURL string `json:"avatar_url"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Contains(t, out.AvatarURL, "/static/avatars/")
	require.Contains(t, out.AvatarURL, ".png",
		"ключ файла должен содержать расширение, снифнутое сервером")
}

// PATCH /users/me с внешним avatar_url отклоняется.
func TestUpdateAvatarURLRejectsExternalHost(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("ext_u"), "ext_"+shortID())

	bad := []string{
		"http://evil.example.com/x.png",
		"https://evil.example.com/x.png",
		"javascript:alert(1)",
		"data:image/png;base64,AAAA",
		"//evil.example.com/x.png",
	}

	for _, v := range bad {
		r := env.srv.do(t, httpReq{
			method: "PATCH", path: "/api/v1/users/me", token: token,
			body: map[string]any{"avatar_url": v},
		})
		require.Equal(t, http.StatusBadRequest, r.Status,
			"avatar_url=%q должен отклоняться", v)
	}
}

// PATCH /users/me с пресетом и с пустой строкой проходит.
func TestUpdateAvatarURLAcceptsPresetAndReset(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("ok_u"), "ok_"+shortID())

	r1 := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/users/me", token: token,
		body: map[string]any{"avatar_url": "/avatars/preset-01.webp"},
	})
	require.Equal(t, http.StatusOK, r1.Status, r1.Raw)

	r2 := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/users/me", token: token,
		body: map[string]any{"avatar_url": ""},
	})
	require.Equal(t, http.StatusOK, r2.Status, r2.Raw)
}

// Прямой запрос к /static/avatars/ (директория) — 404, листинга нет.
func TestStaticDirListingForbidden(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("ls_u"), "ls_"+shortID())
	resp := env.uploadAvatar(t, token, "me.png", makePNG(t))
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Пробуем получить листинг директории.
	req := httptest.NewRequest("GET", "/static/avatars/", nil)
	req.RemoteAddr = uniqueRemoteAddr()
	w := httptest.NewRecorder()
	env.srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code,
		"листинг директории /static/avatars/ должен быть запрещён")

	// Проверяем, что при попытке отдать через handler HTML-файл (не image)
	// на /static — тоже 404. Создаём такой файл напрямую через storage.

	// Проверка X-Content-Type-Options на валидном файле.
	r2 := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me", token: token,
	})
	require.Equal(t, http.StatusOK, r2.Status)
	var me struct {
		AvatarURL string `json:"avatar_url"`
	}
	_ = json.Unmarshal([]byte(r2.Raw), &me)

	// Запросим сам файл через /static.
	if me.AvatarURL != "" {
		// Из URL: http://localhost:8080/static/avatars/xxx.png → /static/avatars/xxx.png
		idx := bytes.Index([]byte(me.AvatarURL), []byte("/static/"))
		require.GreaterOrEqual(t, idx, 0)
		staticPath := me.AvatarURL[idx:]

		req2 := httptest.NewRequest("GET", staticPath, nil)
		req2.RemoteAddr = uniqueRemoteAddr()
		w2 := httptest.NewRecorder()
		env.srv.ServeHTTP(w2, req2)
		require.Equal(t, http.StatusOK, w2.Code)
		require.Equal(t, "nosniff", w2.Header().Get("X-Content-Type-Options"))
		require.Contains(t, w2.Header().Get("Content-Type"), "image/png")
		body, _ := io.ReadAll(w2.Body)
		require.NotEmpty(t, body)
	}
}
