package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"gorm.io/gorm"
)

const (
	EnumDictionaryRechargeTypeCode = "recharge_type"
	EnumDictionaryActivityTagCode  = "activity_tag"
)

type EnumDictionaryItem struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	LabelEn     string `json:"labelEn,omitempty"`
	Description string `json:"description,omitempty"`
}

type EnumDictionaryConfig struct {
	Strict bool                 `json:"strict"`
	Items  []EnumDictionaryItem `json:"items"`
}

type EnumDictionaryResponse struct {
	Code   string               `json:"code"`
	Key    string               `json:"key"`
	Strict bool                 `json:"strict"`
	Items  []EnumDictionaryItem `json:"items"`
}

func ParseEnumDictionaryCodes(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		code := strings.TrimSpace(part)
		if code == "" {
			continue
		}
		items = append(items, code)
	}
	if len(items) == 0 {
		for _, item := range builtInEnumDictionaries() {
			items = append(items, item.Code)
		}
	}
	return UniqueStrings(items)
}

func ListEnumDictionaries(db *gorm.DB, codes []string) ([]EnumDictionaryResponse, error) {
	items := make([]EnumDictionaryResponse, 0, len(codes))
	for _, code := range ParseEnumDictionaryCodes(strings.Join(codes, ",")) {
		dictionary, err := ResolveEnumDictionaryCached(db, code)
		if err != nil {
			return nil, err
		}
		items = append(items, dictionary)
	}
	return items, nil
}

func ResolveEnumDictionary(db *gorm.DB, code string) (EnumDictionaryResponse, error) {
	base, ok := builtInEnumDictionaryByCode(code)
	if !ok {
		return EnumDictionaryResponse{}, errors.New("unsupported enum dictionary: " + code)
	}
	configured, found, err := loadConfiguredEnumDictionary(db, code)
	if err == nil && found {
		return configured, nil
	}
	if err != nil {
		return EnumDictionaryResponse{}, err
	}
	base.Items = normalizeEnumDictionaryItems(base.Items)
	return base, nil
}

func ResolveEnumDictionaryCached(db *gorm.DB, code string) (EnumDictionaryResponse, error) {
	return resolveCachedJSON(cachedEnumDictionaryKey(code), 30*time.Second, func() (EnumDictionaryResponse, error) {
		return ResolveEnumDictionary(db, code)
	})
}

func ValidateEnumDictionaryValue(dictionary EnumDictionaryResponse, fieldName string, value string) error {
	normalized := strings.TrimSpace(value)
	if normalized == "" || !dictionary.Strict {
		return nil
	}
	for _, item := range dictionary.Items {
		if item.Value == normalized {
			return nil
		}
	}
	return fmt.Errorf("invalid %s: %s", fieldName, normalized)
}

func ValidateEnumDictionaryValues(dictionary EnumDictionaryResponse, fieldName string, values []string) error {
	if len(values) == 0 || !dictionary.Strict {
		return nil
	}
	allowed := make(map[string]struct{}, len(dictionary.Items))
	for _, item := range dictionary.Items {
		allowed[item.Value] = struct{}{}
	}
	invalid := make([]string, 0)
	for _, value := range UniqueStrings(values) {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := allowed[normalized]; !ok {
			invalid = append(invalid, normalized)
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("invalid %s: %s", fieldName, strings.Join(invalid, ", "))
	}
	return nil
}

func EnumDictionaryPlatformConfigKey(code string) string {
	return "enum.dictionary." + strings.TrimSpace(code)
}

func builtInEnumDictionaries() []EnumDictionaryResponse {
	return []EnumDictionaryResponse{
		{
			Code:   EnumDictionaryRechargeTypeCode,
			Key:    EnumDictionaryPlatformConfigKey(EnumDictionaryRechargeTypeCode),
			Strict: true,
			Items: []EnumDictionaryItem{
				{Value: "normal", Label: "常规充值", LabelEn: "Normal"},
				{Value: "first_deposit", Label: "首充", LabelEn: "First Deposit"},
				{Value: "vip", Label: "VIP充值", LabelEn: "VIP Recharge"},
			},
		},
		{
			Code:   EnumDictionaryActivityTagCode,
			Key:    EnumDictionaryPlatformConfigKey(EnumDictionaryActivityTagCode),
			Strict: true,
			Items: []EnumDictionaryItem{
				{Value: "campaign-a", Label: "活动A", LabelEn: "Campaign A"},
				{Value: "vip", Label: "VIP活动", LabelEn: "VIP Campaign"},
				{Value: "new-user", Label: "新用户活动", LabelEn: "New User Campaign"},
			},
		},
	}
}

func builtInEnumDictionaryByCode(code string) (EnumDictionaryResponse, bool) {
	for _, item := range builtInEnumDictionaries() {
		if item.Code == strings.TrimSpace(code) {
			return item, true
		}
	}
	return EnumDictionaryResponse{}, false
}

func normalizeEnumDictionaryItems(items []EnumDictionaryItem) []EnumDictionaryItem {
	normalized := make([]EnumDictionaryItem, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item.Value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, EnumDictionaryItem{
			Value:       value,
			Label:       FirstNonEmpty(strings.TrimSpace(item.Label), value),
			LabelEn:     strings.TrimSpace(item.LabelEn),
			Description: strings.TrimSpace(item.Description),
		})
	}
	return normalized
}

func loadConfiguredEnumDictionary(db *gorm.DB, code string) (EnumDictionaryResponse, bool, error) {
	var config model.PlatformConfig
	key := EnumDictionaryPlatformConfigKey(code)
	if err := db.Where("tenant_id IS NULL AND brand_id IS NULL AND key = ?", key).Order("id desc").First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return EnumDictionaryResponse{}, false, nil
		}
		return EnumDictionaryResponse{}, false, err
	}
	var payload EnumDictionaryConfig
	if err := json.Unmarshal(config.Value, &payload); err != nil {
		return EnumDictionaryResponse{}, false, err
	}
	return EnumDictionaryResponse{
		Code:   code,
		Key:    key,
		Strict: payload.Strict,
		Items:  normalizeEnumDictionaryItems(payload.Items),
	}, true, nil
}
