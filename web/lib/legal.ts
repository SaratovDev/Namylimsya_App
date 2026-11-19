// ⚠️ ЗАПОЛНИТЬ ПЕРЕД ПУБЛИКАЦИЕЙ.
// Все поля попадают в /privacy и /terms. Пустые значения → предупреждение
// в dev-консоли и на странице.
export const LEGAL_ENTITY = {
  // Полное наименование оператора (ООО "..." или ИП ФИО).
  operatorName: "ИП [ФАМИЛИЯ ИМЯ ОТЧЕСТВО]",

  // Юридический/фактический адрес.
  operatorAddress: "[ИНДЕКС, ГОРОД, УЛИЦА, ДОМ]",

  // ИНН/ОГРНИП (для полноты реквизитов).
  operatorInn: "[ИНН]",

  // Email для запросов по 152-ФЗ.
  privacyEmail: "privacy@namylimsya.ru",

  // Версия документов. Должна совпадать с legal.ConsentVersion в api.
  consentVersion: "1.0",

  // Дата редакции для отображения.
  documentsDate: "08 ноября 2026 г.",
} as const;

export function isLegalEntityConfigured(): boolean {
  return (
    !LEGAL_ENTITY.operatorName.includes("[") &&
    !LEGAL_ENTITY.operatorAddress.includes("[") &&
    !LEGAL_ENTITY.operatorInn.includes("[")
  );
}