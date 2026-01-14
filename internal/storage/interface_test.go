package storage

import (
	"testing"
)

func TestDatabaseFactory(t *testing.T) {
	factory := NewDatabaseFactory()

	// Test supported drivers
	drivers := factory.GetSupportedDrivers()
	expectedDrivers := []string{"sqlite", "postgres", "rqlite"}

	if len(drivers) != len(expectedDrivers) {
		t.Errorf("Expected %d drivers, got %d", len(expectedDrivers), len(drivers))
	}

	for _, expected := range expectedDrivers {
		found := false
		for _, driver := range drivers {
			if driver == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected driver %s not found", expected)
		}
	}
}

func TestDriverDetection(t *testing.T) {
	factory := NewDatabaseFactory()

	tests := []struct {
		dsn    string
		driver string
	}{
		{"http://localhost:4001", "rqlite"},
		{"https://localhost:4001", "rqlite"},
		{"postgres://user:pass@localhost/db", "postgres"},
		{"postgresql://user:pass@localhost/db", "postgres"},
		{"/path/to/file.db", "sqlite"},
		{"file.db", "sqlite"},
		{"", "sqlite"},
	}

	for _, test := range tests {
		driver := factory.DetectDriverFromDSN(test.dsn)
		if driver != test.driver {
			t.Errorf("For DSN '%s', expected driver '%s', got '%s'", test.dsn, test.driver, driver)
		}
	}
}
