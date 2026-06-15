package providers

import "testing"

func TestCreate_DefaultGenericProvider(t *testing.T) {
	client, err := Create("")
	if err != nil {
		t.Fatalf("Create default provider: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil provider client")
	}
}

func TestCreate_NamedGenericProvider(t *testing.T) {
	client, err := Create("generic")
	if err != nil {
		t.Fatalf("Create generic provider: %v", err)
	}
	if _, ok := client.(*GenericClient); !ok {
		t.Fatalf("expected *GenericClient, got %T", client)
	}
}
