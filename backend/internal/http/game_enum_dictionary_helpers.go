package http

import (
	"encoding/json"

	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/datatypes"
)

func stringSetFromJSON(value datatypes.JSON) map[string]struct{} {
	items := stringSliceFromJSON(value)
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}

func stringSliceFromJSON(value datatypes.JSON) []string {
	if len(value) == 0 {
		return nil
	}
	var items []string
	if err := json.Unmarshal(value, &items); err != nil {
		return nil
	}
	return sharedsvc.UniqueStrings(items)
}

func jsonStringArray(values []string) datatypes.JSON {
	items := sharedsvc.UniqueStrings(values)
	if len(items) == 0 {
		return nil
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return nil
	}
	return datatypes.JSON(payload)
}

const (
	enumDictionaryRechargeTypeCode = sharedsvc.EnumDictionaryRechargeTypeCode
	enumDictionaryActivityTagCode  = sharedsvc.EnumDictionaryActivityTagCode
)

func enumDictionaryPlatformConfigKey(code string) string {
	return sharedsvc.EnumDictionaryPlatformConfigKey(code)
}

func parseEnumDictionaryCodes(raw string) []string {
	return sharedsvc.ParseEnumDictionaryCodes(raw)
}

func validateEnumDictionaryValue(dictionary enumDictionaryResponse, fieldName string, value string) error {
	return sharedsvc.ValidateEnumDictionaryValue(toSharedEnumDictionary(dictionary), fieldName, value)
}

func validateEnumDictionaryValues(dictionary enumDictionaryResponse, fieldName string, values []string) error {
	return sharedsvc.ValidateEnumDictionaryValues(toSharedEnumDictionary(dictionary), fieldName, values)
}

func toHTTPEnumDictionaries(items []sharedsvc.EnumDictionaryResponse) ([]enumDictionaryResponse, error) {
	result := make([]enumDictionaryResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toHTTPEnumDictionary(item))
	}
	return result, nil
}

func toHTTPEnumDictionary(item sharedsvc.EnumDictionaryResponse) enumDictionaryResponse {
	result := enumDictionaryResponse{Code: item.Code, Key: item.Key, Strict: item.Strict}
	for _, dictionaryItem := range item.Items {
		result.Items = append(result.Items, enumDictionaryItem{
			Value:       dictionaryItem.Value,
			Label:       dictionaryItem.Label,
			LabelEn:     dictionaryItem.LabelEn,
			Description: dictionaryItem.Description,
		})
	}
	return result
}

func toSharedEnumDictionary(item enumDictionaryResponse) sharedsvc.EnumDictionaryResponse {
	result := sharedsvc.EnumDictionaryResponse{Code: item.Code, Key: item.Key, Strict: item.Strict}
	for _, dictionaryItem := range item.Items {
		result.Items = append(result.Items, sharedsvc.EnumDictionaryItem{
			Value:       dictionaryItem.Value,
			Label:       dictionaryItem.Label,
			LabelEn:     dictionaryItem.LabelEn,
			Description: dictionaryItem.Description,
		})
	}
	return result
}
