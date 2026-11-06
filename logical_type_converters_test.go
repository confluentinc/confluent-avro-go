package avro_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/confluentinc/confluent-avro-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logicalTypeCase describes one Avro logical type's wire encoding and its
// semantic (converted) vs. raw (unconverted) generic-decode representation.
type logicalTypeCase struct {
	name     string
	schema   string
	data     []byte
	semantic any
	raw      any
}

func logicalTypeCases() []logicalTypeCase {
	return []logicalTypeCase{
		{
			name:     "Date",
			schema:   `{"type":"int","logicalType":"date"}`,
			data:     []byte{0xAE, 0x9D, 0x02},
			semantic: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
			raw:      18263,
		},
		{
			name:     "Time-Millis",
			schema:   `{"type":"int","logicalType":"time-millis"}`,
			data:     []byte{0xAA, 0xB4, 0xDE, 0x75},
			semantic: 123456789 * time.Millisecond,
			raw:      123456789,
		},
		{
			name:     "Time-Micros",
			schema:   `{"type":"long","logicalType":"time-micros"}`,
			data:     []byte{0x86, 0xEA, 0xC8, 0xE9, 0x97, 0x07},
			semantic: 123456789123 * time.Microsecond,
			raw:      int64(123456789123),
		},
		{
			name:     "Timestamp-Millis",
			schema:   `{"type":"long","logicalType":"timestamp-millis"}`,
			data:     []byte{0x90, 0xB2, 0xAE, 0xC3, 0xEC, 0x5B},
			semantic: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
			raw:      int64(1577934245000),
		},
		{
			name:     "Timestamp-Micros",
			schema:   `{"type":"long","logicalType":"timestamp-micros"}`,
			data:     []byte{0x80, 0xCD, 0xB7, 0xA2, 0xEE, 0xC7, 0xCD, 0x05},
			semantic: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
			raw:      int64(1577934245000000),
		},
		{
			name:     "Local-Timestamp-Millis",
			schema:   `{"type":"long","logicalType":"local-timestamp-millis"}`,
			data:     []byte{0x90, 0xB2, 0xAE, 0xC3, 0xEC, 0x5B},
			semantic: time.Date(2020, 1, 2, 3, 4, 5, 0, time.Local),
			raw:      int64(1577934245000),
		},
		{
			name:     "Local-Timestamp-Micros",
			schema:   `{"type":"long","logicalType":"local-timestamp-micros"}`,
			data:     []byte{0x80, 0xCD, 0xB7, 0xA2, 0xEE, 0xC7, 0xCD, 0x05},
			semantic: time.Date(2020, 1, 2, 3, 4, 5, 0, time.Local),
			raw:      int64(1577934245000000),
		},
		{
			name:     "Decimal-Bytes",
			schema:   `{"type":"bytes","logicalType":"decimal","precision":5,"scale":2}`,
			data:     []byte{0x6, 0x00, 0x87, 0x78},
			semantic: big.NewRat(1734, 5),
			raw:      []byte{0x00, 0x87, 0x78},
		},
		{
			name:     "Decimal-Fixed",
			schema:   `{"type":"fixed","name":"decimalFixed","size":6,"logicalType":"decimal","precision":5,"scale":2}`,
			data:     []byte{0x00, 0x00, 0x00, 0x00, 0x87, 0x78},
			semantic: big.NewRat(1734, 5),
			raw:      [6]byte{0x00, 0x00, 0x00, 0x00, 0x87, 0x78},
		},
		{
			name:     "Duration",
			schema:   `{"type":"fixed","name":"duration","size":12,"logicalType":"duration"}`,
			data:     []byte{0x0c, 0x00, 0x00, 0x00, 0x22, 0x00, 0x00, 0x00, 0x52, 0xaa, 0x08, 0x00},
			semantic: avro.LogicalDuration{Months: 12, Days: 34, Milliseconds: 567890},
			raw:      [12]byte{0x0c, 0x00, 0x00, 0x00, 0x22, 0x00, 0x00, 0x00, 0x52, 0xaa, 0x08, 0x00},
		},
	}
}

// TestGenericDecode_LogicalTypes_DefaultConvertsToSemanticType is a
// regression guard: with no opt-in, generic decode must keep returning
// today's semantic Go type for every logical type in the matrix.
func TestGenericDecode_LogicalTypes_DefaultConvertsToSemanticType(t *testing.T) {
	for _, tc := range logicalTypeCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			schema := avro.MustParse(tc.schema)

			var got any
			err := avro.Unmarshal(schema, tc.data, &got)

			require.NoError(t, err)
			assert.Equal(t, tc.semantic, got)
			assert.IsType(t, tc.semantic, got)
		})
	}
}

// TestGenericDecode_LogicalTypes_RawModeReturnsUnderlyingPrimitive exercises
// the new opt-in: requesting raw output must hand back the underlying Avro
// primitive instead of the semantic Go type.
func TestGenericDecode_LogicalTypes_RawModeReturnsUnderlyingPrimitive(t *testing.T) {
	cfg := avro.Config{DisableLogicalTypeConversion: true}.Freeze()

	for _, tc := range logicalTypeCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			schema := avro.MustParse(tc.schema)

			var got any
			err := cfg.Unmarshal(schema, tc.data, &got)

			require.NoError(t, err)
			assert.Equal(t, tc.raw, got)
			assert.IsType(t, tc.raw, got)
		})
	}
}

// TestGenericDecode_NullableLogicalType_Union protects the existing
// union-recursion behavior (NOTES.md §3.5) in both conversion modes: a
// logical type wrapped in a nullable union must still resolve correctly.
func TestGenericDecode_NullableLogicalType_Union(t *testing.T) {
	schema := avro.MustParse(`["null", {"type":"long","logicalType":"timestamp-millis"}]`)
	// union index 1 (non-null branch), then the long timestamp-millis value.
	data := []byte{0x02, 0x90, 0xB2, 0xAE, 0xC3, 0xEC, 0x5B}

	t.Run("default converts", func(t *testing.T) {
		var got any
		err := avro.Unmarshal(schema, data, &got)

		require.NoError(t, err)
		// "long.timestamp-millis" is a pre-registered resolver name, so
		// avro.Unmarshal resolves it directly to its semantic type rather
		// than falling back to the generic map[string]any wrapping that
		// Reader.ReadNext uses for unresolved union branches.
		want := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
		assert.Equal(t, want, got)
	})

	t.Run("raw mode returns underlying primitive", func(t *testing.T) {
		cfg := avro.Config{DisableLogicalTypeConversion: true}.Freeze()

		var got any
		err := cfg.Unmarshal(schema, data, &got)

		require.NoError(t, err)
		want := map[string]any{"long.timestamp-millis": int64(1577934245000)}
		assert.Equal(t, want, got)
	})
}

// TestGenericDecode_UnionResolutionError_RawModeLogicalType protects against
// a decoderOfResolvedUnion regression: raw mode's intentional skip of a
// logical-type union branch (to avoid resolving it to its semantic type) was
// previously indistinguishable, at decode time, from a genuine
// failed-to-resolve branch. Combining DisableLogicalTypeConversion with
// UnionResolutionError must not misreport the raw-mode skip as "unknown
// union type" -- it should fall back to the generic decode, same as when
// UnionResolutionError is unset.
func TestGenericDecode_UnionResolutionError_RawModeLogicalType(t *testing.T) {
	schema := avro.MustParse(`["null", {"type":"long","logicalType":"timestamp-millis"}]`)
	// union index 1 (non-null branch), then the long timestamp-millis value.
	data := []byte{0x02, 0x90, 0xB2, 0xAE, 0xC3, 0xEC, 0x5B}

	cfg := avro.Config{DisableLogicalTypeConversion: true, UnionResolutionError: true}.Freeze()

	var got any
	err := cfg.Unmarshal(schema, data, &got)

	require.NoError(t, err)
	want := map[string]any{"long.timestamp-millis": int64(1577934245000)}
	assert.Equal(t, want, got)
}

// TestRoundTrip_RawLogicalTypeWriteback proves the write side needs no
// changes: a raw primitive decoded in raw mode must re-encode and
// round-trip back to the original semantic value when read again with the
// default (converting) config.
func TestRoundTrip_RawLogicalTypeWriteback(t *testing.T) {
	for _, tc := range logicalTypeCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			schema := avro.MustParse(tc.schema)
			rawCfg := avro.Config{DisableLogicalTypeConversion: true}.Freeze()

			// Original encode, using the default (converting) semantics.
			original, err := avro.Marshal(schema, tc.semantic)
			require.NoError(t, err)

			// Decode generically in raw mode.
			var raw any
			err = rawCfg.Unmarshal(schema, original, &raw)
			require.NoError(t, err)
			assert.Equal(t, tc.raw, raw)

			// Re-encode the raw value with the default writer.
			reEncoded, err := avro.Marshal(schema, raw)
			require.NoError(t, err)

			// Decode again with the default (converting) reader.
			var final any
			err = avro.Unmarshal(schema, reEncoded, &final)
			require.NoError(t, err)

			assert.Equal(t, tc.semantic, final)
		})
	}
}

// logicalTypeStruct mirrors specific (struct-target) decode/encode, which
// must remain unaffected by the new option in either setting -- it was
// never part of the bug (NOTES.md §3, IMPLEMENTATION-PLAN.md §4).
type logicalTypeStruct struct {
	Timestamp time.Time     `avro:"timestamp"`
	Elapsed   time.Duration `avro:"elapsed"`
	Amount    *big.Rat      `avro:"amount"`
}

func logicalTypeStructSchema() string {
	return `{
		"type": "record",
		"name": "logicalTypeStruct",
		"fields": [
			{"name": "timestamp", "type": {"type": "long", "logicalType": "timestamp-millis"}},
			{"name": "elapsed", "type": {"type": "long", "logicalType": "time-micros"}},
			{"name": "amount", "type": {"type": "bytes", "logicalType": "decimal", "precision": 5, "scale": 2}}
		]
	}`
}

// TestSpecificDecode_UnaffectedByLogicalTypeConversionSetting asserts that
// decoding into a concrete Go struct produces identical results regardless
// of DisableLogicalTypeConversion -- the new option only changes the
// *generic* decode path.
func TestSpecificDecode_UnaffectedByLogicalTypeConversionSetting(t *testing.T) {
	schema := avro.MustParse(logicalTypeStructSchema())
	want := logicalTypeStruct{
		Timestamp: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
		Elapsed:   123456789123 * time.Microsecond,
		Amount:    big.NewRat(1734, 5),
	}

	data, err := avro.Marshal(schema, want)
	require.NoError(t, err)

	t.Run("default config", func(t *testing.T) {
		var got logicalTypeStruct
		err := avro.Unmarshal(schema, data, &got)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("raw mode config", func(t *testing.T) {
		cfg := avro.Config{DisableLogicalTypeConversion: true}.Freeze()

		var got logicalTypeStruct
		err := cfg.Unmarshal(schema, data, &got)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}
