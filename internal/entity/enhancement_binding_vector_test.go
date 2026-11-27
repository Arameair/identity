package entity

import "testing"

func TestEnhancementV2BindingVector(t *testing.T) {
	scope := EnhancementScope{SparkID:"spark-1", RequestID:"request-1", PayloadSHA256:"b4474426b5fa431ece71e66cef9b71ba75303a516648b4b03573984e164400da", Recipient:"fixture://fast", PolicyVersion:"v1", FenceEpoch:1}
	if got := enhancementBinding(EnhanceFastAIWorker, "entity-1", scope); got != "0fc10e69cdc6ed0c48d85d84d5919748a143d13410c1cc5c4d5df14a29750e45" {
		t.Fatalf("binding vector: %s", got)
	}
}
