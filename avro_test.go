package avro_test

import "github.com/confluentinc/confluent-avro-go/v2"

func ConfigTeardown() {
	// Reset the caches
	avro.DefaultConfig = avro.Config{}.Freeze()
}
