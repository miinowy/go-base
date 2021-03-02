package kv

import (
	"fmt"
	"testing"
)

const (
	LogInterval = 20000
	MaxSimple   = 500000
	KeyPrefix   = "testing/"
	ValuePrefix = "testing/testing/testing/testing/testing/testing/testing/testing/testing/testing/testing/testing/"
)

func TestKV(t *testing.T) {
	// Put key/value
	for i := 0; i < MaxSimple; i++ {
		if i%LogInterval == 0 {
			t.Logf("Put key/value (%d/%d)...", i, MaxSimple)
		}
		err := Put([]byte(fmt.Sprintf("%s%d", KeyPrefix, i)), []byte(fmt.Sprintf("%s%d", ValuePrefix, i)))
		if err != nil {
			t.Fatal("Put key/value got an error:", err)
		}
	}
	t.Log("Put key/value done")

	// Get key/value
	for i := 0; i < MaxSimple; i++ {
		if i%LogInterval == 0 {
			t.Logf("Get key/value (%d/%d)...", i, MaxSimple)
		}
		key := []byte(fmt.Sprintf("%s%d", KeyPrefix, i))
		value, err := Get(key)
		if err != nil {
			t.Fatal("Get key/value got an error:", err)
		}
		if value == nil {
			t.Fatal("Get key/value got nil")
		}
		if fmt.Sprintf("%s%d", ValuePrefix, i) != string(value) {
			t.Fatal("Get key/value got unexpect value")
		}
	}
	t.Log("Get key/value done")

	// Delete key/value
	for i := 0; i < MaxSimple; i++ {
		if i%LogInterval == 0 {
			t.Logf("Delete key/value (%d/%d)...", i, MaxSimple)
		}
		err := Delete([]byte(fmt.Sprintf("%s%d", KeyPrefix, i)))
		if err != nil {
			t.Fatal("Delete key/value got an error:", err)
		}
	}
	t.Log("Delete key/value done")

	// Has key/value
	for i := 0; i < MaxSimple; i++ {
		if i%LogInterval == 0 {
			t.Logf("Has key/value (%d/%d)...", i, MaxSimple)
		}
		ok, err := Has([]byte(fmt.Sprintf("%s%d", KeyPrefix, i)))
		if err != nil {
			t.Fatal("Has key/value got an error:", err)
		}
		if ok == true {
			t.Fatal("Has key/value got unexpect result")
		}
	}
	t.Log("Has key/value done")

	// Compact database
	if err := Compact(); err != nil {
		t.Fatal("Compact storage got an error:", err)
	}
}
