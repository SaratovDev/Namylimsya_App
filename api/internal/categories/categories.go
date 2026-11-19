package categories

import "strings"

// Slugs — канонический список. Порядок = порядок в UI.
var Slugs = []string{
	"прогулки",
	"спорт",
	"напитки/еда",
	"животные",
	"культура",
	"вечеринки",
	"путешествия",
	"обучение",
	"волонтерство",
	"романтика/свидания",
	"настолки/игры",
	"другое",
}

var set = func() map[string]struct{} {
	m := make(map[string]struct{}, len(Slugs))
	for _, s := range Slugs {
		m[s] = struct{}{}
	}
	return m
}()

// Normalize приводит вход к каноническому slug'у.
// Возвращает ("", false), если категория не из списка.
func Normalize(raw string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := set[s]; !ok {
		return "", false
	}
	return s, true
}

// IsValid — проверка без нормализации.
func IsValid(slug string) bool {
	_, ok := set[slug]
	return ok
}
