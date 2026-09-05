package sharedstock

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ScalarString accepts the string-or-number fields returned by different
// SharedStock versions while preserving their exact decimal representation.
type ScalarString string

func (s *ScalarString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*s = ""
		return nil
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*s = ScalarString(value)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("sharedstock scalar: %w", err)
	}
	*s = ScalarString(number.String())
	return nil
}

type Connection struct {
	ShopName string       `json:"shopName"`
	Balance  ScalarString `json:"balance"`
}

type Category struct {
	ID       ScalarString `json:"id"`
	Name     string       `json:"name"`
	Children []Commodity  `json:"children"`
}

type Commodity struct {
	ID          ScalarString    `json:"id"`
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Cover       string          `json:"cover"`
	Price       ScalarString    `json:"price"`
	Stock       ScalarString    `json:"stock"`
	Config      json.RawMessage `json:"config"`
	DeliveryWay ScalarString    `json:"delivery_way"`
	Status      ScalarString    `json:"status"`
}

type Inventory struct {
	Count        ScalarString    `json:"count"`
	DeliveryWay  ScalarString    `json:"delivery_way"`
	Price        ScalarString    `json:"price"`
	UserPrice    ScalarString    `json:"user_price"`
	FactoryPrice ScalarString    `json:"factory_price"`
	Config       json.RawMessage `json:"config"`
}

type TradeRequest struct {
	SharedCode string
	Quantity   int
	RequestNo  string
	Race       string
}

type Trade struct {
	URL     string       `json:"url"`
	Amount  ScalarString `json:"amount"`
	TradeNo ScalarString `json:"tradeNo"`
	Secret  string       `json:"secret"`
}

type Order struct {
	Secret string       `json:"secret"`
	Status ScalarString `json:"status"`
}
