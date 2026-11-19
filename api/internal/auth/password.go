package auth

import (
	"golang.org/x/crypto/bcrypt"
)

// Cost по умолчанию для bcrypt. 14 — разумный прод-дефолт,
// но в тестах его можно переопределить через SetCost(4),
// иначе интеграционные тесты упираются в общий таймаут go test.
var passwordCost = 14

// SetCost устанавливает cost для bcrypt. Используется в тестах
// для ускорения (cost=4 — минимум, ~1ms на хэш).
// Значения вне [bcrypt.MinCost, bcrypt.MaxCost] игнорируются.
func SetCost(c int) {
	if c < bcrypt.MinCost || c > bcrypt.MaxCost {
		return
	}
	passwordCost = c
}

// GetCost возвращает текущий cost (для диагностики в тестах).
func GetCost() int {
	return passwordCost
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), passwordCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
