package avro_test

import avro "github.com/confluentinc/confluent-avro-go/v2"

func ConfigTeardown() {
	// Reset the caches
	avro.DefaultConfig = avro.Config{}.Freeze()
}
