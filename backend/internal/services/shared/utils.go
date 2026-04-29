package shared

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
)

func ParseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func Round2(value float64) float64 {
	return float64(int64(value*100+0.5)) / 100
}

func NormalizeOptionalAmount(value *float64) *float64 {
	if value == nil {
		return nil
	}
	normalized := Round2(*value)
	return &normalized
}

func MustJSONBytes(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return payload
}

func DefaultCurrency(currency string) string {
	value := strings.ToUpper(strings.TrimSpace(currency))
	if value == "" {
		return "CNY"
	}
	return value
}

func DefaultRuleDepth(depth uint32) uint32 {
	if depth == 0 {
		return 1
	}
	return depth
}

func NormalizeCurrencyFilter(currency string) (string, error) {
	value := strings.TrimSpace(currency)
	if len(value) > 16 {
		return "", errors.New("invalid currency")
	}
	if value == "" {
		return "", nil
	}
	return strings.ToUpper(value), nil
}

func ValidateAgentLedgerType(ledgerType model.LedgerType) error {
	switch ledgerType {
	case model.LedgerTypeIncome, model.LedgerTypeFreeze, model.LedgerTypeUnfreeze, model.LedgerTypeReverse:
		return nil
	default:
		return errors.New("unsupported ledgerType")
	}
}

func BuildReferenceNo(prefix, seed string, index int) string {
	if index > 0 {
		return prefix + "-" + seed + "-" + strconv.Itoa(index)
	}
	return prefix + "-" + seed
}

func IsUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate")
}

func ClassifyFrozenRisk(ratio float64) string {
	switch {
	case ratio >= 0.5:
		return "high"
	case ratio >= 0.15:
		return "medium"
	default:
		return "low"
	}
}
