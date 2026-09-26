package catalog_test

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

type catalogProduct struct {
	ID          uint              `json:"id"`
	Title       map[string]string `json:"title"`
	Description map[string]string `json:"description"`
	Content     map[string]string `json:"content"`
	SKUs        []catalogSKU      `json:"skus"`
}

type catalogSKU struct {
	SpecValues map[string]any `json:"spec_values"`
}

var hanCharacters = regexp.MustCompile(`[\p{Han}]`)

func TestEnglishCatalogHasNoChineseFallback(t *testing.T) {
	raw, err := os.ReadFile("agent-products.json")
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}

	var products []catalogProduct
	if err := json.Unmarshal(raw, &products); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(products) == 0 {
		t.Fatal("catalog is empty")
	}

	for _, product := range products {
		for field, value := range map[string]string{
			"title":       product.Title["en-US"],
			"description": product.Description["en-US"],
			"content":     product.Content["en-US"],
		} {
			if strings.TrimSpace(value) == "" {
				t.Errorf("product %d has empty en-US %s", product.ID, field)
			}
			if hanCharacters.MatchString(value) {
				t.Errorf("product %d en-US %s still contains Chinese text", product.ID, field)
			}
		}
	}
}

func TestCodexCatalogSKUsHaveLocalizedDenominations(t *testing.T) {
	raw, err := os.ReadFile("agent-products.json")
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}

	var products []catalogProduct
	if err := json.Unmarshal(raw, &products); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}

	for _, product := range products {
		if product.ID != 19 {
			continue
		}
		if len(product.SKUs) != 3 {
			t.Fatalf("Codex product has %d SKUs, want 3", len(product.SKUs))
		}
		for index, sku := range product.SKUs {
			for _, locale := range []string{"zh-CN", "zh-TW", "en-US"} {
				value, ok := sku.SpecValues[locale].(string)
				if !ok || strings.TrimSpace(value) == "" {
					t.Errorf("Codex SKU %d has no %s denomination", index, locale)
				}
			}
		}
		return
	}

	t.Fatal("Codex credits product not found")
}

func TestNewCatalogProductsHaveCompleteLocales(t *testing.T) {
	raw, err := os.ReadFile("agent-products.json")
	if err != nil {
		t.Fatal(err)
	}
	var products []catalogProduct
	if err := json.Unmarshal(raw, &products); err != nil {
		t.Fatal(err)
	}
	missing := map[uint]bool{36: true, 37: true, 38: true}
	for _, product := range products {
		if !missing[product.ID] {
			continue
		}
		delete(missing, product.ID)
		for field, values := range map[string]map[string]string{"title": product.Title, "description": product.Description, "content": product.Content} {
			for _, locale := range []string{"zh-CN", "zh-TW", "en-US"} {
				if strings.TrimSpace(values[locale]) == "" {
					t.Errorf("product %d missing %s %s", product.ID, locale, field)
				}
				if strings.Contains(values[locale], `\n`) {
					t.Errorf("product %d escaped newline in %s %s", product.ID, locale, field)
				}
			}
			if values["en"] != values["en-US"] {
				t.Errorf("product %d has inconsistent English aliases in %s", product.ID, field)
			}
		}
	}
	if len(missing) != 0 {
		t.Fatalf("missing localized catalog products: %v", missing)
	}
}
