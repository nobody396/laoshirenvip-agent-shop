package gormstore

import (
	"testing"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestExternalReferenceStoreAllocatesStableScopedIDs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&siteconnectiondomain.ExternalReference{}); err != nil {
		t.Fatal(err)
	}
	store := NewExternalReferenceStore(db)

	productID, err := store.Resolve(1, siteconnectiondomain.ExternalReferenceKindProduct, "ABC")
	if err != nil {
		t.Fatal(err)
	}
	repeatedID, err := store.Resolve(1, siteconnectiondomain.ExternalReferenceKindProduct, "ABC")
	if err != nil {
		t.Fatal(err)
	}
	skuID, err := store.Resolve(1, siteconnectiondomain.ExternalReferenceKindSKU, "ABC::250")
	if err != nil {
		t.Fatal(err)
	}
	otherConnectionID, err := store.Resolve(2, siteconnectiondomain.ExternalReferenceKindProduct, "ABC")
	if err != nil {
		t.Fatal(err)
	}

	if productID == 0 || repeatedID != productID {
		t.Fatalf("reference was not stable: first=%d repeated=%d", productID, repeatedID)
	}
	if skuID == productID || otherConnectionID == productID {
		t.Fatalf("references must be scoped by kind and connection: product=%d sku=%d other=%d", productID, skuID, otherConnectionID)
	}
	key, err := store.Lookup(1, siteconnectiondomain.ExternalReferenceKindSKU, skuID)
	if err != nil {
		t.Fatal(err)
	}
	if key != "ABC::250" {
		t.Fatalf("unexpected reverse lookup: %q", key)
	}
	missing, err := store.Lookup(2, siteconnectiondomain.ExternalReferenceKindSKU, skuID)
	if err != nil {
		t.Fatal(err)
	}
	if missing != "" {
		t.Fatalf("cross-connection lookup leaked key %q", missing)
	}
}
