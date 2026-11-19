package services

import (
	"strings"
	"testing"
)

func TestRenderEmailBodyUsesBaseURL(t *testing.T) {
	items := []emailItem{
		{Type: "application_approved", Payload: map[string]any{}},
	}
	body := renderEmailBody(items, "https://namylimsya.ru")

	if !strings.Contains(body, `href="https://namylimsya.ru"`) {
		t.Fatalf("body должен содержать baseURL, got: %s", body)
	}
	if strings.Contains(body, "localhost:3000") {
		t.Fatalf("body не должен содержать localhost:3000, got: %s", body)
	}
}

func TestRenderEmailBodyFallsBackWhenBaseURLEmpty(t *testing.T) {
	items := []emailItem{
		{Type: "application_new", Payload: map[string]any{"event_title": "Прогулка"}},
	}
	body := renderEmailBody(items, "")

	if !strings.Contains(body, `href="http://localhost:3000"`) {
		t.Fatalf("body должен содержать fallback localhost, got: %s", body)
	}
	if !strings.Contains(body, "Прогулка") {
		t.Fatalf("body должен содержать event_title, got: %s", body)
	}
}

func TestRenderEmailBodyEscapesTitle(t *testing.T) {
	// event_title — пользовательский ввод. Без escape → фишинг/XSS в письме.
	items := []emailItem{
		{Type: "application_new", Payload: map[string]any{
			"event_title": `<script>alert(1)</script>`,
		}},
	}
	body := renderEmailBody(items, "https://test.local")

	if strings.Contains(body, "<script>") {
		t.Fatalf("сырой <script> в body — XSS: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("ожидали escaped &lt;script&gt;, got: %s", body)
	}
}

func TestRenderEmailBodyEscapesTitleWithAnchor(t *testing.T) {
	// Классический фишинг-payload в названии события.
	items := []emailItem{
		{Type: "application_new", Payload: map[string]any{
			"event_title": `<a href="https://evil.example.com">Нажмите сюда</a>`,
		}},
	}
	body := renderEmailBody(items, "https://test.local")

	if strings.Contains(body, `href="https://evil.example.com"`) {
		t.Fatalf("небезопасный href в body: %s", body)
	}
	if !strings.Contains(body, "&lt;a href=") {
		t.Fatalf("ожидали escaped &lt;a href=, got: %s", body)
	}
}
